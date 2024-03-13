package pkg

import (
	"context"
	"strings"

	"github.com/go-logr/logr"

	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/klog/v2"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type jobReconciler struct {
	client client.Client
}

func newJobReconciler(client client.Client) *jobReconciler {
	return &jobReconciler{
		client: client,
	}
}

func (r *jobReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var job batchv1.Job

	if err := r.client.Get(ctx, req.NamespacedName, &job); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	log := ctrl.LoggerFrom(ctx)
	log.V(2).Info("Reconciling Job")

	if strings.HasPrefix(job.Name, SampleJobPrefix) {
		if job.Status.CompletionTime != nil && !job.Status.CompletionTime.IsZero() {
			log.V(1).Info("Sample job completed")

			// Re-inject the sample job probe
			sample := job.DeepCopy()
			sample.Name = job.GenerateName + "-" + rand.String(5)
			sample.ResourceVersion = ""
			sample.Spec.Selector = nil
			sample.Spec.Template.Labels = nil
			return ctrl.Result{}, r.client.Create(ctx, sample)
		}
	}

	if job.Status.CompletionTime != nil || !job.Status.CompletionTime.IsZero() {
		return ctrl.Result{}, nil
	}

	if ready := job.Status.Ready; ready != nil && *ready == *job.Spec.Parallelism {
		log.V(1).Info("Job is ready", "pods", *job.Status.Ready)

		pods := &metav1.PartialObjectMetadataList{
			TypeMeta: metav1.TypeMeta{
				Kind:       "PodList",
				APIVersion: "v1",
			},
		}
		err := r.client.List(ctx, pods, client.InNamespace(job.Namespace),
			client.MatchingLabels{batchv1.JobNameLabel: job.Name})
		if err != nil {
			return ctrl.Result{}, err
		}

		for _, pod := range pods.Items {
			patch := &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Pod",
					"metadata": map[string]interface{}{
						"namespace": pod.Namespace,
						"name":      pod.Name,
						"annotations": map[string]string{
							"job-ready": "true",
						},
					},
				},
			}
			err = r.client.Patch(ctx, patch, client.Apply, client.FieldOwner("job-controller"))
			if err != nil {
				return ctrl.Result{}, err
			}
		}
	}

	return ctrl.Result{}, nil
}

func (r *jobReconciler) setupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&batchv1.Job{}).
		WithLogConstructor(func(request *reconcile.Request) logr.Logger {
			logger := ctrl.Log.WithName("job-reconciler")
			if request != nil {
				return logger.WithValues("job", klog.KRef(request.Namespace, request.Name))
			}
			return logger
		}).
		Complete(r)
}
