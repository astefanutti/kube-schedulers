package test

import (
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/sync/errgroup"

	. "github.com/astefanutti/kube-schedulers/pkg"
	. "github.com/astefanutti/kube-schedulers/test/support"
	"github.com/go-logr/logr/testr"
	. "github.com/onsi/gomega"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	schedulingv1alpha1 "sigs.k8s.io/scheduler-plugins/apis/scheduling/v1alpha1"
)

const schedulerName = "scheduler-plugins-scheduler"

func TestCoscheduling(t *testing.T) {
	test := With(t)

	test.T().Logf("Configuring nodes")

	applyNodeConfiguration(test, sampleNodeConfiguration())

	for i := 0; i < NodesCount; i++ {
		applyNodeConfiguration(test, workerNodeConfiguration(fmt.Sprintf("kwok-node-%03d", i)))
	}

	test.T().Logf("Configuring priority classes")

	applyPriorityClassConfiguration(test, highPriorityClassConfiguration())

	ns := test.NewTestNamespace()

	test.T().Logf("Created test namespace %s", ns.Namespace)

	test.T().Logf("Starting manager")

	mgr, err := NewManager(test.Client().GetConfig(), testr.NewWithOptions(test.T(), LogOptions), ns.Name)
	test.Expect(err).NotTo(HaveOccurred())

	go func() {
		test.Expect(mgr.Start(test.Ctx())).To(Succeed())
	}()

	test.T().Logf("Creating jobs")

	group, ctx := errgroup.WithContext(test.Ctx())
	var count atomic.Int32
	for i := 0; i < JobsCreationRoutines; i++ {
		group.Go(func() error {
			for j := count.Add(1); j <= JobsCount && ctx.Err() == nil; j = count.Add(1) {
				name := fmt.Sprintf("job-%03d", j)

				groupResourceCPU := PodResourceCPU.DeepCopy()
				groupResourceCPU.Mul(JobsCount)

				groupResourceMemory := PodResourceMemory.DeepCopy()
				groupResourceMemory.Mul(JobsCount)

				podGroup := &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": schedulingv1alpha1.SchemeGroupVersion.String(),
						"kind":       "PodGroup",
						"metadata": map[string]interface{}{
							"name": name,
						},
						"spec": map[string]interface{}{
							"minMember": PodsByJobCount,
							"minResources": map[string]interface{}{
								"cpu":    groupResourceCPU.String(),
								"memory": groupResourceMemory.String(),
							},
							"scheduleTimeoutSeconds": int64(JobsCompletionTimeout.Seconds()),
						},
					},
				}

				_, err := test.Client().Dynamic().
					Resource(schedulingv1alpha1.SchemeGroupVersion.WithResource("podgroups")).
					Namespace(ns.Name).
					Apply(test.Ctx(), name, podGroup, ApplyOptions)
				if err != nil {
					return err
				}

				job := testJob(ns.Name, name)
				job.Spec.Template.Labels = map[string]string{
					schedulingv1alpha1.PodGroupLabel: name,
				}
				job.Spec.Template.Spec.SchedulerName = schedulerName
				job.Spec.ActiveDeadlineSeconds = nil
				createJob(test, job)

				maybeCreateSampleJob(test, ns, j)
			}
			return nil
		})
	}
	test.Expect(group.Wait()).To(Succeed())

	test.T().Logf("Waiting for jobs to complete")

	test.Eventually(Jobs(test, ns, LabelSelector("app.kubernetes.io/part-of!=sample-jobs"))).
		WithPolling(15 * time.Second).
		WithTimeout(JobsCompletionTimeout).
		Should(And(
			HaveLen(JobsCount),
			HaveEach(Or(
				WithTransform(ConditionStatus(batchv1.JobComplete), Equal(corev1.ConditionTrue)),
				WithTransform(ConditionStatus(batchv1.JobFailed), Equal(corev1.ConditionTrue)),
			))))

	test.T().Logf("Cleaning namespace %s up", ns.Name)
}
