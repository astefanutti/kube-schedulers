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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kueuev1beta1 "sigs.k8s.io/kueue/apis/kueue/v1beta1"
	kueuev1beta1ac "sigs.k8s.io/kueue/client-go/applyconfiguration/kueue/v1beta1"
)

func TestKueue(t *testing.T) {
	test := With(t)

	test.T().Logf("Configuring nodes")

	applyNodeConfiguration(test, sampleNodeConfiguration())

	allocatableResources := corev1.ResourceList{}

	for i := 0; i < NodesCount; i++ {
		node := applyNodeConfiguration(test, workerNodeConfiguration(fmt.Sprintf("kwok-node-%03d", i)))
		for k, v := range node.Status.Allocatable {
			quantity := allocatableResources[k]
			quantity.Add(v)
			allocatableResources[k] = quantity
		}
	}

	test.T().Logf("Configuring priority classes")

	applyPriorityClassConfiguration(test, highPriorityClassConfiguration())

	test.T().Logf("Configuring Kueue")

	flavorAC := kueuev1beta1ac.ResourceFlavor("kwok").
		WithSpec(kueuev1beta1ac.ResourceFlavorSpec().
			WithNodeLabels(map[string]string{"type": "kwok"}).
			WithTolerations(corev1.Toleration{
				Key:      kwokNode,
				Operator: corev1.TolerationOpEqual,
				Value:    string(FakeNode),
				Effect:   corev1.TaintEffectNoSchedule,
			}))

	flavor, err := test.Client().Kueue().KueueV1beta1().ResourceFlavors().Apply(test.Ctx(), flavorAC, ApplyOptions)
	test.Expect(err).NotTo(HaveOccurred())

	clusterQueueAC := kueuev1beta1ac.ClusterQueue("queue").
		WithSpec(kueuev1beta1ac.ClusterQueueSpec().
			WithNamespaceSelector(metav1.LabelSelector{}).
			WithResourceGroups(kueuev1beta1ac.ResourceGroup().
				WithCoveredResources(corev1.ResourceCPU, corev1.ResourceMemory).
				WithFlavors(kueuev1beta1ac.FlavorQuotas().
					WithName(kueuev1beta1.ResourceFlavorReference(flavor.Name)).
					WithResources(
						kueuev1beta1ac.ResourceQuota().
							WithName(corev1.ResourceCPU).
							WithNominalQuota(allocatableResources[corev1.ResourceCPU]),
						kueuev1beta1ac.ResourceQuota().
							WithName(corev1.ResourceMemory).
							WithNominalQuota(allocatableResources[corev1.ResourceMemory]),
					))))

	clusterQueue, err := test.Client().Kueue().KueueV1beta1().ClusterQueues().Apply(test.Ctx(), clusterQueueAC, ApplyOptions)
	test.Expect(err).NotTo(HaveOccurred())

	ns := test.NewTestNamespace()

	test.T().Logf("Created test namespace %s", ns.Namespace)

	localQueueAC := kueuev1beta1ac.LocalQueue("queue", ns.Name).
		WithSpec(kueuev1beta1ac.LocalQueueSpec().
			WithClusterQueue(kueuev1beta1.ClusterQueueReference(clusterQueue.Name)))

	localQueue, err := test.Client().Kueue().KueueV1beta1().LocalQueues(ns.Name).Apply(test.Ctx(), localQueueAC, ApplyOptions)
	test.Expect(err).NotTo(HaveOccurred())

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
				job := testJob(ns.Name, fmt.Sprintf("job-%03d", j))
				job.Labels = map[string]string{
					"kueue.x-k8s.io/queue-name": localQueue.Name,
				}
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
