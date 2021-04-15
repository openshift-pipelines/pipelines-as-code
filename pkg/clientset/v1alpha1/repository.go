package v1alpha1

import (
	"context"

	v1alpha1 "github.com/openshift-pipelines/pipelines-as-code/pkg/api/types/v1apha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
)

type RepositoryInterface interface {
	List(ctx context.Context, opts v1.ListOptions) (*v1alpha1.RepositoryList, error)
	Get(ctx context.Context, name string, options v1.GetOptions) (*v1alpha1.Repository, error)
}

type repositoryClient struct {
	client rest.Interface
	ns     string
}

func (c *repositoryClient) List(ctx context.Context, opts v1.ListOptions) (*v1alpha1.RepositoryList, error) {
	result := v1alpha1.RepositoryList{}
	err := c.client.
		Get().
		Namespace(c.ns).
		Resource("repositories").
		VersionedParams(&opts, scheme.ParameterCodec).
		Do(ctx).
		Into(&result)

	return &result, err
}

func (c *repositoryClient) Get(ctx context.Context, name string, opts v1.GetOptions) (*v1alpha1.Repository, error) {
	result := v1alpha1.Repository{}
	err := c.client.
		Get().
		Namespace(c.ns).
		Resource("repositories").
		Name(name).
		VersionedParams(&opts, scheme.ParameterCodec).
		Do(ctx).
		Into(&result)

	return &result, err
}
