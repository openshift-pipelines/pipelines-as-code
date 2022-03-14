package github

import (
	"context"
	"os"
	"testing"

	ghlib "github.com/google/go-github/v42/github"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/options"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/repository"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/secret"
	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func CreateCRD(ctx context.Context, t *testing.T, repoinfo *ghlib.Repository, run *params.Run, opts options.E2E, targetNS string) error {
	repo := &v1alpha1.Repository{
		ObjectMeta: metav1.ObjectMeta{
			Name: targetNS,
		},
		Spec: v1alpha1.RepositorySpec{
			URL: repoinfo.GetHTMLURL(),
		},
	}

	err := repository.CreateNS(ctx, targetNS, run)
	assert.NilError(t, err)

	if opts.DirectWebhook {
		token, _ := os.LookupEnv("TEST_GITHUB_TOKEN")
		apiURL, _ := os.LookupEnv("TEST_GITHUB_API_URL")
		err := secret.Create(ctx, run, map[string]string{"token": token}, targetNS, "webhook-token")
		assert.NilError(t, err)
		repo.Spec.GitProvider = &v1alpha1.GitProvider{
			URL:    apiURL,
			Secret: &v1alpha1.GitProviderSecret{Name: "webhook-token", Key: "token"},
		}
	}

	err = repository.CreateRepo(ctx, targetNS, run, repo)
	assert.NilError(t, err)
	return err
}
