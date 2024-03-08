package test

import (
	"fmt"
	"testing"
	"time"

	. "github.com/astefanutti/kube-schedulers/test/support"
	. "github.com/onsi/gomega"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/wait"
	batchv1ac "k8s.io/client-go/applyconfigurations/batch/v1"
	corev1ac "k8s.io/client-go/applyconfigurations/core/v1"

	schedulingv1alpha1 "sigs.k8s.io/scheduler-plugins/apis/scheduling/v1alpha1"
)

const schedulerName = "scheduler-plugins-scheduler"

func TestCoscheduling(t *testing.T) {
	test := With(t)

	applyNodeConfiguration(test, sampleNodeConfiguration())

	for i := 0; i < NodesCount; i++ {
		applyNodeConfiguration(test, workerNodeConfiguration(fmt.Sprintf("kwok-node-%03d", i)))
	}

	ns := test.NewTestNamespace()

	for j := 0; j < JobsCount; j++ {
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
		test.Expect(err).NotTo(HaveOccurred())

		batchAC := batchv1ac.Job(name, ns.Name).
			WithSpec(batchv1ac.JobSpec().
				WithCompletions(PodsByJobCount).
				WithParallelism(PodsByJobCount).
				// WithActiveDeadlineSeconds(JobActiveDeadlineSeconds).
				WithBackoffLimit(0).
				WithTemplate(corev1ac.PodTemplateSpec().
					WithAnnotations(map[string]string{
						"duration": wait.Jitter(2*time.Minute, 0.5).String(),
					}).
					WithLabels(map[string]string{
						schedulingv1alpha1.PodGroupLabel: name,
					}).
					WithSpec(corev1ac.PodSpec().
						WithSchedulerName(schedulerName).
						WithRestartPolicy(corev1.RestartPolicyNever).
						WithAffinity(corev1ac.Affinity().
							WithNodeAffinity(corev1ac.NodeAffinity().
								WithRequiredDuringSchedulingIgnoredDuringExecution(corev1ac.NodeSelector().
									WithNodeSelectorTerms(corev1ac.NodeSelectorTerm().
										WithMatchExpressions(corev1ac.NodeSelectorRequirement().
											WithKey("type").
											WithOperator(corev1.NodeSelectorOpIn).
											WithValues("kwok")))))).
						WithTolerations(corev1ac.Toleration().
							WithKey(kwokNode).
							WithEffect(corev1.TaintEffectNoSchedule).
							WithOperator(corev1.TolerationOpEqual).
							WithValue(string(fake))).
						WithContainers(corev1ac.Container().
							WithName("fake").
							WithImage("fake").
							WithResources(corev1ac.ResourceRequirements().
								WithRequests(corev1.ResourceList{
									corev1.ResourceCPU:    PodResourceCPU,
									corev1.ResourceMemory: PodResourceMemory,
								}).
								WithLimits(corev1.ResourceList{
									corev1.ResourceCPU:    PodResourceCPU,
									corev1.ResourceMemory: PodResourceMemory,
								}))))))

		_, err = test.Client().Core().BatchV1().Jobs(ns.Name).Apply(test.Ctx(), batchAC, ApplyOptions)
		test.Expect(err).NotTo(HaveOccurred())
	}

	annotatePodsWithJobReadiness(test, ns)

	test.T().Logf("Waiting for jobs to complete")

	test.Eventually(Jobs(test, ns)).WithPolling(15 * time.Second).WithTimeout(JobsCompletionTimeout).
		Should(And(
			HaveLen(JobsCount),
			HaveEach(Or(
				WithTransform(ConditionStatus(batchv1.JobComplete), Equal(corev1.ConditionTrue)),
				WithTransform(ConditionStatus(batchv1.JobFailed), Equal(corev1.ConditionTrue)),
			))))

	test.T().Logf("Cleaning namespace %s up", ns.Name)
}
