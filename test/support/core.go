package support

import (
	"github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Node(t Test, name string) func(g gomega.Gomega) *corev1.Node {
	return func(g gomega.Gomega) *corev1.Node {
		node, err := t.Client().Core().CoreV1().Nodes().Get(t.Ctx(), name, metav1.GetOptions{})
		g.Expect(err).NotTo(gomega.HaveOccurred())
		return node
	}
}
