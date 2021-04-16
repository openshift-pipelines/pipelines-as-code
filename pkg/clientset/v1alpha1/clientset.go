package v1alpha1

import (
	v1alpha1 "github.com/openshift-pipelines/pipelines-as-code/pkg/api/types/v1alpha1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
)

type RepositoryV1Alpha1Interface interface {
	Repositories(namespace string) RepositoryInterface
}

type RepositoryV1Alpha1Client struct {
	restClient rest.Interface
}

func setConfigDefaults(config *rest.Config) error {
	gv := v1alpha1.SchemeGroupVersion
	config.GroupVersion = &gv
	config.APIPath = "/apis"
	config.NegotiatedSerializer = scheme.Codecs.WithoutConversion()

	if config.UserAgent == "" {
		config.UserAgent = rest.DefaultKubernetesUserAgent()
	}

	return nil
}

// NewForConfig creates a new TektonV1beta1Client for the given config.
func NewForConfig(c *rest.Config) (*RepositoryV1Alpha1Client, error) {
	v1alpha1.AddToScheme(scheme.Scheme)

	config := *c
	if err := setConfigDefaults(&config); err != nil {
		return nil, err
	}
	client, err := rest.RESTClientFor(&config)
	if err != nil {
		return nil, err
	}
	return &RepositoryV1Alpha1Client{client}, nil
}

func (c *RepositoryV1Alpha1Client) Repositories(namespace string) RepositoryInterface {
	return &repositoryClient{
		client: c.restClient,
		ns:     namespace,
	}
}
