package test

import (
	"time"

	. "github.com/astefanutti/kube-schedulers/pkg"
	. "github.com/astefanutti/kube-schedulers/test/support"
	"github.com/go-logr/logr/testr"
	. "github.com/onsi/gomega"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	corev1ac "k8s.io/client-go/applyconfigurations/core/v1"
	"k8s.io/utils/ptr"
)

const (
	NodesCount               = 100
	JobsCount                = 1000
	PodsByJobCount           = 10
	JobsCreationRoutines     = 5
	JobActiveDeadlineSeconds = 15 * 60
	JobsCompletionTimeout    = 60 * time.Minute
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
							Key:      KwokNode,
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
