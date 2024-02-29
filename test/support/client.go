package support

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	kueue "sigs.k8s.io/kueue/client-go/clientset/versioned"
)

var ApplyOptions = metav1.ApplyOptions{FieldManager: "test", Force: true}

type Client interface {
	Core() kubernetes.Interface
	Dynamic() dynamic.Interface
	Kueue() kueue.Interface
}

type testClient struct {
	core    kubernetes.Interface
	dynamic dynamic.Interface
	kueue   kueue.Interface
}

var _ Client = (*testClient)(nil)

func (t *testClient) Core() kubernetes.Interface {
	return t.core
}

func (t *testClient) Dynamic() dynamic.Interface {
	return t.dynamic
}

func (t *testClient) Kueue() kueue.Interface {
	return t.kueue
}

func newTestClient(cfg *rest.Config) (Client, error) {
	var err error
	if cfg == nil {
		cfg, err = clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
			clientcmd.NewDefaultClientConfigLoadingRules(),
			&clientcmd.ConfigOverrides{},
		).ClientConfig()
		if err != nil {
			return nil, err
		}
	}

	kubeClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}

	dynamicClient, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}

	kueueClient, err := kueue.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}

	return &testClient{
		core:    kubeClient,
		dynamic: dynamicClient,
		kueue:   kueueClient,
	}, nil
}
