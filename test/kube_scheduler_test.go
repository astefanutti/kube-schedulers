package test

import (
	"fmt"
	"testing"
	"time"

	. "github.com/astefanutti/kube-schedulers/test/support"
	. "github.com/onsi/gomega"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/wait"
	batchv1ac "k8s.io/client-go/applyconfigurations/batch/v1"
	corev1ac "k8s.io/client-go/applyconfigurations/core/v1"
)

func TestKubeScheduler(t *testing.T) {
	test := With(t)

	sample := corev1ac.Node("sample").
		WithAnnotations(map[string]string{
			"node.alpha.kubernetes.io/ttl": "0",
			kwokNode:                       "fake",
		}).
		WithLabels(map[string]string{
			"type":                           "kwok",
			"kubernetes.io/arch":             "amd64",
			"kubernetes.io/hostname":         "sample",
			"kubernetes.io/os":               "linux",
			"kubernetes.io/role":             "sample",
			"node-role.kubernetes.io/sample": "",
		}).
		WithSpec(corev1ac.NodeSpec().
			WithTaints(corev1ac.Taint().
				WithKey("kwok.x-k8s.io/node").
				WithEffect(corev1.TaintEffectNoSchedule).
				WithValue(string(sample)))).
		WithStatus(corev1ac.NodeStatus().
			WithAllocatable(corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("1000"),
				corev1.ResourceMemory: resource.MustParse("1000Gi"),
				corev1.ResourcePods:   resource.MustParse("1000"),
			}).
			WithCapacity(corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("1000"),
				corev1.ResourceMemory: resource.MustParse("1000Gi"),
				corev1.ResourcePods:   resource.MustParse("1000"),
			}).
			WithNodeInfo(corev1ac.NodeSystemInfo().
				WithKubeProxyVersion("fake").
				WithKubeletVersion("fake")))

	_, err := test.Client().Core().CoreV1().Nodes().Apply(test.Ctx(), sample, ApplyOptions)
	test.Expect(err).NotTo(HaveOccurred())

	_, err = test.Client().Core().CoreV1().Nodes().ApplyStatus(test.Ctx(), sample, ApplyOptions)
	test.Expect(err).NotTo(HaveOccurred())

	test.Eventually(Node(test, "sample")).
		Should(WithTransform(ConditionStatus(corev1.NodeReady), Equal(corev1.ConditionTrue)))

	for i := 0; i < NodesCount; i++ {
		name := fmt.Sprintf("kwok-node-%03d", i)
		node := corev1ac.Node(name).
			WithAnnotations(map[string]string{
				"node.alpha.kubernetes.io/ttl": "0",
				kwokNode:                       "fake",
			}).
			WithLabels(map[string]string{
				"type":                          "kwok",
				"kubernetes.io/arch":            "amd64",
				"kubernetes.io/hostname":        name,
				"kubernetes.io/os":              "linux",
				"kubernetes.io/role":            "agent",
				"node-role.kubernetes.io/agent": "",
			}).
			WithSpec(corev1ac.NodeSpec().
				WithTaints(corev1ac.Taint().
					WithKey(kwokNode).
					WithEffect(corev1.TaintEffectNoSchedule).
					WithValue(string(fake)))).
			WithStatus(corev1ac.NodeStatus().
				WithAllocatable(corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("10"),
					corev1.ResourceMemory: resource.MustParse("10Gi"),
					corev1.ResourcePods:   resource.MustParse("100"),
				}).
				WithCapacity(corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("10"),
					corev1.ResourceMemory: resource.MustParse("10Gi"),
					corev1.ResourcePods:   resource.MustParse("100"),
				}).
				WithNodeInfo(corev1ac.NodeSystemInfo().
					WithKubeProxyVersion("fake").
					WithKubeletVersion("fake")))

		_, err := test.Client().Core().CoreV1().Nodes().Apply(test.Ctx(), node, ApplyOptions)
		test.Expect(err).NotTo(HaveOccurred())

		_, err = test.Client().Core().CoreV1().Nodes().ApplyStatus(test.Ctx(), node, ApplyOptions)
		test.Expect(err).NotTo(HaveOccurred())

		test.Eventually(Node(test, name)).
			Should(WithTransform(ConditionStatus(corev1.NodeReady), Equal(corev1.ConditionTrue)))
	}

	ns := test.NewTestNamespace()

	for j := 0; j < JobsCount; j++ {
		name := fmt.Sprintf("job-%03d", j)

		batchAC := batchv1ac.Job(name, ns.Name).
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
									corev1.ResourceCPU:    resource.MustParse("1"),
									corev1.ResourceMemory: resource.MustParse("1Gi"),
								}).
								WithLimits(corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("1"),
									corev1.ResourceMemory: resource.MustParse("1Gi"),
								}))))))

		_, err := test.Client().Core().BatchV1().Jobs(ns.Name).Apply(test.Ctx(), batchAC, ApplyOptions)
		test.Expect(err).NotTo(HaveOccurred())
	}

	test.T().Logf("Waiting for jobs to complete")

	test.Eventually(Jobs(test, ns)).WithPolling(15 * time.Second).WithTimeout(15 * time.Minute).
		Should(And(
			HaveLen(JobsCount),
			HaveEach(Or(
				WithTransform(ConditionStatus(batchv1.JobComplete), Equal(corev1.ConditionTrue)),
				WithTransform(ConditionStatus(batchv1.JobFailed), Equal(corev1.ConditionTrue)),
			))))

	test.T().Logf("Cleaning namespace %s up", ns.Name)
}
