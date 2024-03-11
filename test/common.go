package test

import (
	"time"

	. "github.com/astefanutti/kube-schedulers/pkg"
	. "github.com/astefanutti/kube-schedulers/test/support"
	"github.com/go-logr/logr/testr"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	corev1ac "k8s.io/client-go/applyconfigurations/core/v1"
)

const (
	NodesCount               = 100
	JobsCount                = 500
	PodsByJobCount           = 10
	JobsCreationRoutines     = 10
	JobActiveDeadlineSeconds = 600
	JobsCompletionTimeout    = 30 * time.Minute
)

var (
	PodResourceCPU    = resource.MustParse("1")
	PodResourceMemory = resource.MustParse("1Gi")
)

var LogOptions = testr.Options{
	LogTimestamp: false,
	Verbosity:    1,
}

func applyNodeConfiguration(test Test, nodeAC *corev1ac.NodeApplyConfiguration) *corev1.Node {
	test.T().Helper()

	node, err := test.Client().Core().CoreV1().Nodes().Apply(test.Ctx(), nodeAC, ApplyOptions)
	test.Expect(err).NotTo(HaveOccurred())

	node, err = test.Client().Core().CoreV1().Nodes().ApplyStatus(test.Ctx(), nodeAC, ApplyOptions)
	test.Expect(err).NotTo(HaveOccurred())

	test.Eventually(Node(test, "sample")).
		Should(WithTransform(ConditionStatus(corev1.NodeReady), Equal(corev1.ConditionTrue)))

	return node
}

func sampleNodeConfiguration() *corev1ac.NodeApplyConfiguration {
	return corev1ac.Node("sample").
		WithAnnotations(map[string]string{
			"node.alpha.kubernetes.io/ttl": "0",
			KwokNode:                       "fake",
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
				WithKey(KwokNode).
				WithEffect(corev1.TaintEffectNoSchedule).
				WithValue(string(SampleNode)))).
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
}

func workerNodeConfiguration(name string) *corev1ac.NodeApplyConfiguration {
	return corev1ac.Node(name).
		WithAnnotations(map[string]string{
			"node.alpha.kubernetes.io/ttl": "0",
			KwokNode:                       "fake",
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
				WithKey(KwokNode).
				WithEffect(corev1.TaintEffectNoSchedule).
				WithValue(string(FakeNode)))).
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
}
