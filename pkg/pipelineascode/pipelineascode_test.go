package pipelineascode

import (
	"testing"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/test"
	"gotest.tools/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	rtesting "knative.dev/pkg/reconciler/testing"
)

func newRepo(name, url, branch, eventType, namespace string) *v1alpha1.Repository {
	return &v1alpha1.Repository{
		ObjectMeta: v1.ObjectMeta{Name: name},
		Spec: v1alpha1.RepositorySpec{
			Namespace: namespace,
			URL:       url,
			Branch:    branch,
			EventType: eventType,
		},
	}
}

func TestFilterBy(t *testing.T) {
	ctx, _ := rtesting.SetupFakeContext(t)
	testParams := []struct {
		name, namespace, url, branch, eventType string
		nomatch                                 bool
		repositories                            []*v1alpha1.Repository
	}{
		{
			name:         "test-good",
			repositories: []*v1alpha1.Repository{newRepo("test-good", "https://foo/bar", "lovedone", "pull_request", "namespace")},
			url:          "https://foo/bar",
			eventType:    "pull_request",
			branch:       "lovedone",
			namespace:    "namespace",
		},
		{
			name:         "test-notmatch",
			repositories: []*v1alpha1.Repository{newRepo("test-notmatch", "https://foo/bar", "lovedone", "pull_request", "namespace")},
			url:          "https://xyz/vlad",
			eventType:    "pull_request",
			branch:       "lovedone",
			namespace:    "namespace",
			nomatch:      true,
		},
	}

	for _, tp := range testParams {
		t.Run(tp.name, func(t *testing.T) {
			d := test.Data{
				Repositories: tp.repositories,
			}
			cs, _ := test.SeedTestData(t, ctx, d)
			pac := PipelineAsCode{Client: cs.PipelineAsCode.PipelinesascodeV1alpha1()}
			repo, err := pac.FilterBy(tp.url, tp.branch, tp.eventType)
			if err != nil {
				t.Fatal(err)
			}

			if tp.nomatch {
				assert.Equal(t, repo.Spec.Namespace, "")
			} else {
				assert.Equal(t, repo.Spec.Namespace, tp.namespace)
			}
		})

	}

}
