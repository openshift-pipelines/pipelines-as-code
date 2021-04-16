package test

import (
	"context"
	"testing"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	fakepacclientset "github.com/openshift-pipelines/pipelines-as-code/pkg/generated/clientset/versioned/fake"
	informersv1alpha1 "github.com/openshift-pipelines/pipelines-as-code/pkg/generated/informers/externalversions/pipelinesascode/v1alpha1"
	fakepacclient "github.com/openshift-pipelines/pipelines-as-code/pkg/generated/injection/client/fake"
	fakerepositoryinformers "github.com/openshift-pipelines/pipelines-as-code/pkg/generated/injection/informers/pipelinesascode/v1alpha1/repository/fake"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Clients struct {
	PipelineAsCode *fakepacclientset.Clientset
}

// Informers holds references to informers which are useful for reconciler tests.
type Informers struct {
	Repository informersv1alpha1.RepositoryInformer
}

type Data struct {
	Repositories []*v1alpha1.Repository
}

// SeedTestData returns Clients and Informers populated with the
// given Data.
// nolint: golint
func SeedTestData(t *testing.T, ctx context.Context, d Data) (Clients, Informers) {
	c := Clients{
		PipelineAsCode: fakepacclient.Get(ctx),
	}
	i := Informers{
		Repository: fakerepositoryinformers.Get(ctx),
	}

	for _, repo := range d.Repositories {
		if err := i.Repository.Informer().GetIndexer().Add(repo); err != nil {
			t.Fatal(err)
		}
		if _, err := c.PipelineAsCode.PipelinesascodeV1alpha1().Repositories(repo.Namespace).Create(ctx, repo, metav1.CreateOptions{}); err != nil {
			t.Fatal(err)
		}

	}

	c.PipelineAsCode.ClearActions()
	return c, i
}
