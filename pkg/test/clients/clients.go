package clients

import (
	"context"
	"testing"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	fakepacclientset "github.com/openshift-pipelines/pipelines-as-code/pkg/generated/clientset/versioned/fake"
	informersv1alpha1 "github.com/openshift-pipelines/pipelines-as-code/pkg/generated/informers/externalversions/pipelinesascode/v1alpha1"
	fakepacclient "github.com/openshift-pipelines/pipelines-as-code/pkg/generated/injection/client/fake"
	fakerepositoryinformers "github.com/openshift-pipelines/pipelines-as-code/pkg/generated/injection/informers/pipelinesascode/v1alpha1/repository/fake"
	pipelinev1alpha1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	fakepipelineclientset "github.com/tektoncd/pipeline/pkg/client/clientset/versioned/fake"
	pipelineinformersv1alpha1 "github.com/tektoncd/pipeline/pkg/client/informers/externalversions/pipeline/v1alpha1"
	fakepipelineclient "github.com/tektoncd/pipeline/pkg/client/injection/client/fake"
	fakepipelineruninformer "github.com/tektoncd/pipeline/pkg/client/injection/informers/pipeline/v1alpha1/pipelinerun/fake"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	fakekubeclientset "k8s.io/client-go/kubernetes/fake"
	fakekubeclient "knative.dev/pkg/client/injection/kube/client/fake"
)

type Clients struct {
	Pipeline       *fakepipelineclientset.Clientset
	PipelineAsCode *fakepacclientset.Clientset
	Kube           *fakekubeclientset.Clientset
}

// Informers holds references to informers which are useful for reconciler tests.
type Informers struct {
	PipelineRun pipelineinformersv1alpha1.PipelineRunInformer
	Repository  informersv1alpha1.RepositoryInformer
}

type Data struct {
	PipelineRuns []*pipelinev1alpha1.PipelineRun
	Repositories []*v1alpha1.Repository
	Namespaces   []*corev1.Namespace
}

// SeedTestData returns Clients and Informers populated with the
// given Data.
// nolint: golint, revive
func SeedTestData(t *testing.T, ctx context.Context, d Data) (Clients, Informers) {
	c := Clients{
		PipelineAsCode: fakepacclient.Get(ctx),
		Kube:           fakekubeclient.Get(ctx),
		Pipeline:       fakepipelineclient.Get(ctx),
	}
	i := Informers{
		Repository:  fakerepositoryinformers.Get(ctx),
		PipelineRun: fakepipelineruninformer.Get(ctx),
	}

	for _, pr := range d.PipelineRuns {
		if err := i.PipelineRun.Informer().GetIndexer().Add(pr); err != nil {
			t.Fatal(err)
		}
		if _, err := c.Pipeline.TektonV1alpha1().PipelineRuns(pr.Namespace).Create(ctx, pr, metav1.CreateOptions{}); err != nil {
			t.Fatal(err)
		}
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
