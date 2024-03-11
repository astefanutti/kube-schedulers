package support

import (
	"github.com/onsi/gomega"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Jobs(t Test, ns *corev1.Namespace, options ...Option[*metav1.ListOptions]) func(g gomega.Gomega) []batchv1.Job {
	return func(g gomega.Gomega) []batchv1.Job {
		listOptions := metav1.ListOptions{}
		err := applyOptions(&listOptions, options...)
		g.Expect(err).NotTo(gomega.HaveOccurred())

		list, err := t.Client().Core().BatchV1().Jobs(ns.Name).List(t.Ctx(), listOptions)
		g.Expect(err).NotTo(gomega.HaveOccurred())
		return list.Items
	}
}
