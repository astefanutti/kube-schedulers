package test

import (
	"context"
	"fmt"
	"time"

	. "github.com/astefanutti/kube-schedulers/pkg"
	. "github.com/astefanutti/kube-schedulers/test/support"
	"github.com/go-logr/logr/testr"
	. "github.com/onsi/gomega"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	schedulingv1 "k8s.io/api/scheduling/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/wait"
	corev1ac "k8s.io/client-go/applyconfigurations/core/v1"
	schedulingv1ac "k8s.io/client-go/applyconfigurations/scheduling/v1"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	NodesCount               = 100
	JobsCount                = 1000
	PodsByJobCount           = 10
	JobsCreationRoutines     = 5
	JobActiveDeadlineSeconds = 15 * 60
	JobsCompletionTimeout    = 60 * time.Minute
)

const (
	kwokNode              = "kwok.x-k8s.io/node"
	highPriorityClassName = "high-priority"
	sampleJobsLabel       = "sample-jobs"
)

type NodeType string

var (
	FakeNode   NodeType = "fake"
	SampleNode NodeType = "sample"
)

var (
	PodResourceCPU    = resource.MustParse("2")
	PodResourceMemory = resource.MustParse("2Gi")
)

var LogOptions = testr.Options{
	LogTimestamp: false,
	Verbosity:    1,
}

func testJob(namespace, name string) *batchv1.Job {
	return &batchv1.Job{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		Spec: batchv1.JobSpec{
			Parallelism:           ptr.To(int32(PodsByJobCount)),
			Completions:           ptr.To(int32(PodsByJobCount)),
			ActiveDeadlineSeconds: ptr.To(int64(JobActiveDeadlineSeconds)),
			BackoffLimit:          ptr.To(int32(0)),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"duration": wait.Jitter(3*time.Minute, 0.5).String(),
					},
				},
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Affinity: &corev1.Affinity{
						NodeAffinity: &corev1.NodeAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
								NodeSelectorTerms: []corev1.NodeSelectorTerm{
									{
										MatchExpressions: []corev1.NodeSelectorRequirement{
											{
												Key:      "type",
												Operator: corev1.NodeSelectorOpIn,
												Values:   []string{"kwok"},
											},
										},
									},
								},
							},
						},
					},
					Tolerations: []corev1.Toleration{
						{
							Key:      kwokNode,
							Operator: corev1.TolerationOpEqual,
							Value:    string(FakeNode),
							Effect:   corev1.TaintEffectNoSchedule,
						},
					},
					Containers: []corev1.Container{
						{
							Name:  "fake",
							Image: "fake",
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    PodResourceCPU,
									corev1.ResourceMemory: PodResourceMemory,
								},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    PodResourceCPU,
									corev1.ResourceMemory: PodResourceMemory,
								},
							},
						},
					},
				},
			},
		},
	}
}

func sampleJob(namespace, name string, duration time.Duration) *batchv1.Job {
	return &batchv1.Job{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Namespace:    namespace,
			Name:         name + "-" + rand.String(5),
			GenerateName: name,
			Labels: map[string]string{
				"app.kubernetes.io/part-of": sampleJobsLabel,
			},
		},
		Spec: batchv1.JobSpec{
			Parallelism:  ptr.To(int32(1)),
			Completions:  ptr.To(int32(1)),
			BackoffLimit: ptr.To(int32(0)),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"duration": duration.String(),
					},
				},
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Affinity: &corev1.Affinity{
						NodeAffinity: &corev1.NodeAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
								NodeSelectorTerms: []corev1.NodeSelectorTerm{
									{
										MatchExpressions: []corev1.NodeSelectorRequirement{
											{
												Key:      "type",
												Operator: corev1.NodeSelectorOpIn,
												Values:   []string{"kwok"},
											},
										},
									},
								},
							},
						},
					},
					Tolerations: []corev1.Toleration{
						{
							Key:      kwokNode,
							Operator: corev1.TolerationOpEqual,
							Value:    string(SampleNode),
							Effect:   corev1.TaintEffectNoSchedule,
						},
					},
					Containers: []corev1.Container{
						{
							Name:  "fake",
							Image: "fake",
						},
					},
				},
			},
		},
	}
}

func excludeSampleJobs(test Test) labels.Selector {
	selector, err := labels.Parse("app.kubernetes.io/part-of!=" + sampleJobsLabel)
	test.Expect(err).NotTo(HaveOccurred())
	return selector
}

func createJob(test Test, job *batchv1.Job) *batchv1.Job {
	test.T().Helper()

	job, err := test.Client().Core().BatchV1().Jobs(job.Namespace).Create(test.Ctx(), job, metav1.CreateOptions{})
	test.Expect(err).NotTo(HaveOccurred())

	return job
}

func maybeCreateSampleJob(test Test, ns *corev1.Namespace, index int32) {
	test.T().Helper()

	if index%10 == 0 {
		var sample *batchv1.Job
		if (index/10)%2 == 0 {
			sample = sampleJob(ns.Name, fmt.Sprintf("%sdefault-%03d", SampleJobPrefix, index/10), 10*time.Second)
		} else {
			sample = sampleJob(ns.Name, fmt.Sprintf("%shigh-%03d", SampleJobPrefix, index/10), 10*time.Second)
			sample.Spec.Template.Spec.PriorityClassName = highPriorityClassName
		}
		createJob(test, sample)
	}
}

func jobs(ctx context.Context, mgr ctrl.Manager, ns *corev1.Namespace, selector labels.Selector) func(g Gomega) []batchv1.Job {
	return func(g Gomega) []batchv1.Job {
		list := batchv1.JobList{}
		err := mgr.GetClient().List(ctx, &list, client.InNamespace(ns.Name),
			client.MatchingLabelsSelector{Selector: selector})
		g.Expect(err).NotTo(HaveOccurred())
		return list.Items
	}
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
				WithKey(kwokNode).
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
				WithValue(string(FakeNode)))).
		WithStatus(corev1ac.NodeStatus().
			WithAllocatable(corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("100"),
				corev1.ResourceMemory: resource.MustParse("100Gi"),
				corev1.ResourcePods:   resource.MustParse("1000"),
			}).
			WithCapacity(corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("100"),
				corev1.ResourceMemory: resource.MustParse("100Gi"),
				corev1.ResourcePods:   resource.MustParse("1000"),
			}).
			WithNodeInfo(corev1ac.NodeSystemInfo().
				WithKubeProxyVersion("fake").
				WithKubeletVersion("fake")))
}

func applyPriorityClassConfiguration(test Test, priorityClassAC *schedulingv1ac.PriorityClassApplyConfiguration) *schedulingv1.PriorityClass {
	test.T().Helper()

	priorityClass, err := test.Client().Core().SchedulingV1().PriorityClasses().Apply(test.Ctx(), priorityClassAC, ApplyOptions)
	test.Expect(err).NotTo(HaveOccurred())

	return priorityClass
}

func highPriorityClassConfiguration() *schedulingv1ac.PriorityClassApplyConfiguration {
	return schedulingv1ac.PriorityClass(highPriorityClassName).
		WithValue(1000).
		WithGlobalDefault(false)
}
