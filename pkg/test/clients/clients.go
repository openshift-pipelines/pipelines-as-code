package clients

import (
	"context"
	"testing"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	fakepacclientset "github.com/openshift-pipelines/pipelines-as-code/pkg/generated/clientset/versioned/fake"
	informersv1alpha1 "github.com/openshift-pipelines/pipelines-as-code/pkg/generated/informers/externalversions/pipelinesascode/v1alpha1"
	fakepacclient "github.com/openshift-pipelines/pipelines-as-code/pkg/generated/injection/client/fake"
	fakerepositoryinformers "github.com/openshift-pipelines/pipelines-as-code/pkg/generated/injection/informers/pipelinesascode/v1alpha1/repository/fake"
	v1alpha12 "github.com/openshift-pipelines/pipelines-as-code/pkg/generated/listers/pipelinesascode/v1alpha1"
	pipelinev1beta1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	fakepipelineclientset "github.com/tektoncd/pipeline/pkg/client/clientset/versioned/fake"
	"github.com/tektoncd/pipeline/pkg/client/informers/externalversions/pipeline/v1beta1"
	fakepipelineclient "github.com/tektoncd/pipeline/pkg/client/injection/client/fake"
	fakepipelineruninformer "github.com/tektoncd/pipeline/pkg/client/injection/informers/pipeline/v1beta1/pipelinerun/fake"
	v1beta12 "github.com/tektoncd/pipeline/pkg/client/listers/pipeline/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	fakekubeclientset "k8s.io/client-go/kubernetes/fake"
	fakekubeclient "knative.dev/pkg/client/injection/kube/client/fake"
)

type Clients struct {
	Pipeline         *fakepipelineclientset.Clientset
	PipelineAsCode   *fakepacclientset.Clientset
	Kube             *fakekubeclientset.Clientset
	PipelineLister   v1beta12.PipelineRunLister
	RepositoryLister v1alpha12.RepositoryLister
}

// Informers holds references to informers which are useful for reconciler tests.
type Informers struct {
	PipelineRun v1beta1.PipelineRunInformer
	Repository  informersv1alpha1.RepositoryInformer
}

type Data struct {
	PipelineRuns []*pipelinev1beta1.PipelineRun
	Repositories []*v1alpha1.Repository
	Namespaces   []*corev1.Namespace
	Secret       []*corev1.Secret
	Events       []*corev1.Event
}

// SeedTestData returns Clients and Informers populated with the
// given Data.
func SeedTestData(t *testing.T, ctx context.Context, d Data) (Clients, Informers) { //nolint: golint, revive
	c := Clients{
		PipelineAsCode: fakepacclient.Get(ctx),
		Kube:           fakekubeclient.Get(ctx),
		Pipeline:       fakepipelineclient.Get(ctx),
	}
	i := Informers{
		Repository:  fakerepositoryinformers.Get(ctx),
		PipelineRun: fakepipelineruninformer.Get(ctx),
	}
	c.PipelineLister = i.PipelineRun.Lister()
	c.RepositoryLister = i.Repository.Lister()

	for _, pr := range d.PipelineRuns {
		if err := i.PipelineRun.Informer().GetIndexer().Add(pr); err != nil {
			t.Fatal(err)
		}
		if _, err := c.Pipeline.TektonV1beta1().PipelineRuns(pr.Namespace).Create(ctx, pr, metav1.CreateOptions{}); err != nil {
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

	for _, n := range d.Namespaces {
		if _, err := c.Kube.CoreV1().Namespaces().Create(ctx, n, metav1.CreateOptions{}); err != nil {
			t.Fatal(err)
		}
	}

	for _, n := range d.Secret {
		if _, err := c.Kube.CoreV1().Secrets(n.Namespace).Create(ctx, n, metav1.CreateOptions{}); err != nil {
			t.Fatal(err)
		}
	}

	for _, n := range d.Events {
		if _, err := c.Kube.CoreV1().Events(n.Namespace).Create(ctx, n, metav1.CreateOptions{}); err != nil {
			t.Fatal(err)
		}
	}

	c.PipelineAsCode.ClearActions()
	c.Pipeline.ClearActions()
	c.Kube.ClearActions()
	return c, i
}
