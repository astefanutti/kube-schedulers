package test

import (
	"fmt"
	"testing"

	. "github.com/astefanutti/kube-schedulers/test/support"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	corev1ac "k8s.io/client-go/applyconfigurations/core/v1"
)

func TestKubeScheduler(t *testing.T) {
	test := With(t)

	for i := 0; i < 10; i++ {
		name := fmt.Sprintf("kwok-node-%03d", i)
		node := corev1ac.Node(name).
			WithAnnotations(map[string]string{
				"node.alpha.kubernetes.io/ttl": "0",
				"kwok.x-k8s.io/node":           "fake",
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
					WithKey("kwok.x-k8s.io/node").
					WithEffect(corev1.TaintEffectNoSchedule).
					WithValue("fake"))).
			WithStatus(corev1ac.NodeStatus().
				WithAllocatable(corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("16"),
					corev1.ResourceMemory: resource.MustParse("32Gi"),
					corev1.ResourcePods:   resource.MustParse("100"),
				}).
				WithCapacity(corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("16"),
					corev1.ResourceMemory: resource.MustParse("32Gi"),
					corev1.ResourcePods:   resource.MustParse("100"),
				}).
				WithNodeInfo(corev1ac.NodeSystemInfo().
					WithKubeProxyVersion("fake").
					WithKubeletVersion("fake")))

		_, err := test.Client().Core().CoreV1().Nodes().Apply(test.Ctx(), node, ApplyOptions)
		test.Expect(err).NotTo(HaveOccurred())

		_, err = test.Client().Core().CoreV1().Nodes().ApplyStatus(test.Ctx(), node, ApplyOptions)
		test.Expect(err).NotTo(HaveOccurred())
	}
}
