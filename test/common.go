package test

import (
	"context"
	"strings"
	"time"

	"github.com/astefanutti/kube-schedulers/test/support"
	. "github.com/astefanutti/kube-schedulers/test/support"
	. "github.com/onsi/gomega"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	batchv1ac "k8s.io/client-go/applyconfigurations/batch/v1"
	corev1ac "k8s.io/client-go/applyconfigurations/core/v1"
	retrywatch "k8s.io/client-go/tools/watch"
)

const kwokNode = "kwok.x-k8s.io/node"

const (
	NodesCount               = 100
	JobsCount                = 500
	PodsByJobCount           = 10
	JobActiveDeadlineSeconds = 600
	JobsCompletionTimeout    = 30 * time.Minute

	sampleJobPrefix = "sample-"
)

var (
	PodResourceCPU    = resource.MustParse("1")
	PodResourceMemory = resource.MustParse("1Gi")
)

type NodeType string

var (
	fake   NodeType = "fake"
	sample NodeType = "sample"
)

type watchInterface interface {
	Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error)
}

type watcher struct {
	ctx    context.Context
	client watchInterface
}

func newWatcher(ctx context.Context, client watchInterface) *watcher {
	return &watcher{ctx: ctx, client: client}
}

func (w *watcher) Watch(options metav1.ListOptions) (watch.Interface, error) {
	return w.client.Watch(w.ctx, options)
}

func watchJobs(test Test, ns *corev1.Namespace, handlers ...func(Test, *batchv1.Job)) {
	test.T().Helper()

	jobs, err := test.Client().Core().BatchV1().Jobs(ns.Name).List(test.Ctx(), metav1.ListOptions{})
	test.Expect(err).NotTo(HaveOccurred())

	jobsWatcher, err := retrywatch.NewRetryWatcher(jobs.ResourceVersion, newWatcher(test.Ctx(),
		test.Client().Core().BatchV1().Jobs(ns.Name)))
	test.Expect(err).NotTo(HaveOccurred())

	go func() {
		ctx, cancel := context.WithCancel(test.Ctx())
		test.T().Cleanup(cancel)
		defer jobsWatcher.Stop()
		for {
			select {
			case <-ctx.Done():
				break
			case e := <-jobsWatcher.ResultChan():
				switch e.Type {
				case watch.Error:
					test.T().Errorf("error watching for Jobs: %v", apierrors.FromObject(e.Object))
				case watch.Added, watch.Modified:
					job, ok := e.Object.(*batchv1.Job)
					if !ok {
						test.T().Errorf("unexpected event object: %v", e.Object)
					}
					for _, handler := range handlers {
						handler(test, job)
					}
				}
			}
		}
	}()
}

func annotatePodsWithJobReadiness(test Test, job *batchv1.Job) {
	if job.Status.CompletionTime != nil || !job.Status.CompletionTime.IsZero() {
		return
	}
	if ready := job.Status.Ready; ready != nil && *ready == *job.Spec.Parallelism {
		pods, err := test.Client().Core().CoreV1().Pods(job.Namespace).List(test.Ctx(), metav1.ListOptions{
			LabelSelector: batchv1.JobNameLabel + "=" + job.Name,
		})
		test.Expect(err).NotTo(HaveOccurred())

		for _, pod := range pods.Items {
			podAC := corev1ac.Pod(pod.Name, pod.Namespace).
				WithAnnotations(map[string]string{
					"job-ready": "true",
				})
			_, err := test.Client().Core().CoreV1().Pods(pod.Namespace).ApplyStatus(test.Ctx(), podAC, ApplyOptions)
			test.Expect(err).NotTo(HaveOccurred())
		}
	}
}

func injectJobSamples(test Test, job *batchv1.Job) {
	if !strings.HasPrefix(job.Name, sampleJobPrefix) {
		return
	}
	if job.Status.CompletionTime == nil || job.Status.CompletionTime.IsZero() {
		return
	}
	applyJobConfiguration(test, sampleJobConfiguration(job.GenerateName).WithNamespace(job.Namespace))
}

func applyJobConfiguration(test support.Test, jobAC *batchv1ac.JobApplyConfiguration) *batchv1.Job {
	test.T().Helper()

	test.Expect(jobAC.Namespace).NotTo(BeNil())

	job, err := test.Client().Core().BatchV1().Jobs(*jobAC.Namespace).Apply(test.Ctx(), jobAC, ApplyOptions)
	test.Expect(err).NotTo(HaveOccurred())

	return job
}

func applyNodeConfiguration(test support.Test, nodeAC *corev1ac.NodeApplyConfiguration) *corev1.Node {
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
}

func workerNodeConfiguration(name string) *corev1ac.NodeApplyConfiguration {
	return corev1ac.Node(name).
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
}

func sampleJobConfiguration(name string) *batchv1ac.JobApplyConfiguration {
	return batchv1ac.Job(name+"-"+rand.String(5), "").
		WithGenerateName(name).
		WithLabels(map[string]string{
			"app.kubernetes.io/part-of": "sample-jobs",
		}).
		WithSpec(batchv1ac.JobSpec().
			WithCompletions(1).
			WithParallelism(1).
			WithActiveDeadlineSeconds(JobActiveDeadlineSeconds).
			WithBackoffLimit(0).
			WithTemplate(corev1ac.PodTemplateSpec().
				WithAnnotations(map[string]string{
					"duration": wait.Jitter(10*time.Second, 0.5).String(),
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
						WithValue(string(sample))).
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
}
