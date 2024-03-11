package test

import (
	"fmt"
	"testing"
	"time"

	. "github.com/astefanutti/kube-schedulers/test/support"
	. "github.com/onsi/gomega"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	batchv1ac "k8s.io/client-go/applyconfigurations/batch/v1"
	corev1ac "k8s.io/client-go/applyconfigurations/core/v1"

	kueuev1beta1 "sigs.k8s.io/kueue/apis/kueue/v1beta1"
	kueuev1beta1ac "sigs.k8s.io/kueue/client-go/applyconfiguration/kueue/v1beta1"
)

func TestKueue(t *testing.T) {
	test := With(t)

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

	flavorAC := kueuev1beta1ac.ResourceFlavor("kwok").
		WithSpec(kueuev1beta1ac.ResourceFlavorSpec().
			WithNodeLabels(map[string]string{"type": "kwok"}).
			WithTolerations(corev1.Toleration{
				Key:      kwokNode,
				Operator: corev1.TolerationOpEqual,
				Value:    string(fake),
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

	localQueueAC := kueuev1beta1ac.LocalQueue("queue", ns.Name).
		WithSpec(kueuev1beta1ac.LocalQueueSpec().
			WithClusterQueue(kueuev1beta1.ClusterQueueReference(clusterQueue.Name)))

	localQueue, err := test.Client().Kueue().KueueV1beta1().LocalQueues(ns.Name).Apply(test.Ctx(), localQueueAC, ApplyOptions)
	test.Expect(err).NotTo(HaveOccurred())

	watchJobs(test, ns, annotatePodsWithJobReadiness, injectJobSamples)

	applyJobConfiguration(test, sampleJobConfiguration(fmt.Sprintf("%s%03d", sampleJobPrefix, 0)).WithNamespace(ns.Name))

	test.T().Logf("Creating jobs")

	for j := 0; j < JobsCount; j++ {
		name := fmt.Sprintf("job-%03d", j)

		batchAC := batchv1ac.Job(name, ns.Name).
			WithLabels(map[string]string{
				"kueue.x-k8s.io/queue-name": localQueue.Name,
			}).
			WithSpec(batchv1ac.JobSpec().
				WithCompletions(PodsByJobCount).
				WithParallelism(PodsByJobCount).
				WithActiveDeadlineSeconds(JobActiveDeadlineSeconds).
				WithBackoffLimit(0).
				WithTemplate(corev1ac.PodTemplateSpec().
					WithAnnotations(map[string]string{
						"duration": wait.Jitter(2*time.Minute, 0.5).String(),
					}).
					WithSpec(corev1ac.PodSpec().
						WithRestartPolicy(corev1.RestartPolicyNever).
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

		_, err := test.Client().Core().BatchV1().Jobs(ns.Name).Apply(test.Ctx(), batchAC, ApplyOptions)
		test.Expect(err).NotTo(HaveOccurred())
	}

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
