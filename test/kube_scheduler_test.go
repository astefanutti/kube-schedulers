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
)

func TestKubeScheduler(t *testing.T) {
	test := With(t)

	test.T().Logf("Configuring nodes")

	applyNodeConfiguration(test, sampleNodeConfiguration())

	for i := 0; i < NodesCount; i++ {
		applyNodeConfiguration(test, workerNodeConfiguration(fmt.Sprintf("kwok-node-%03d", i)))
	}

	ns := test.NewTestNamespace()

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

				_, err = test.Client().Core().BatchV1().Jobs(ns.Name).Create(ctx, job, metav1.CreateOptions{})
				if err != nil {
					return err
				}

				if j%10 == 0 {
					_, err = test.Client().Core().BatchV1().Jobs(ns.Name).
						Create(ctx, SampleJob(ns.Name, fmt.Sprintf("%s%03d", SampleJobPrefix, j/10)), metav1.CreateOptions{})
					if err != nil {
						return err
					}
				}
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
