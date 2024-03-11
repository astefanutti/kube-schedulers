package pkg

import (
	"github.com/go-logr/logr"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/rest"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/config"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

var (
	scheme = runtime.NewScheme()
)

func init() {
	utilruntime.Must(corev1.AddToScheme(scheme))
	utilruntime.Must(batchv1.AddToScheme(scheme))
}

func NewManager(cfg *rest.Config, logger logr.Logger, namespace string) (ctrl.Manager, error) {
	ctrl.SetLogger(logger)

	options := ctrl.Options{
		Scheme:         scheme,
		LeaderElection: false,
		Metrics: metricsserver.Options{
			BindAddress: "0",
		},
		HealthProbeBindAddress: "0",
		Controller: config.Controller{
			MaxConcurrentReconciles: 10,
		},
		Cache: cache.Options{
			DefaultNamespaces: map[string]cache.Config{
				namespace: {},
			},
		},
	}

	mgr, err := ctrl.NewManager(cfg, options)
	if err != nil {
		return nil, err
	}

	err = newJobReconciler(mgr.GetClient()).setupWithManager(mgr)
	if err != nil {
		return nil, err
	}

	return mgr, nil
}
