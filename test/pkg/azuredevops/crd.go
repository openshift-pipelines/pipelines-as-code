package azuredevops

import (
	"context"
	"os"
	"testing"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	azprovider "github.com/openshift-pipelines/pipelines-as-code/pkg/provider/azuredevops"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/options"
	pacrepo "github.com/openshift-pipelines/pipelines-as-code/test/pkg/repository"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/secret"
	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func CreateCRD(ctx context.Context, t *testing.T, azProvider azprovider.Provider, run *params.Run, opts options.E2E, targetNS string) {

	adoToken := os.Getenv("TEST_AZURE_DEVOPS_TOKEN")
	adoRepo := os.Getenv("TEST_AZURE_DEVOPS_REPO")

	err := pacrepo.CreateNS(ctx, targetNS, run)
	assert.NilError(t, err)

	err = secret.Create(ctx, run, map[string]string{"token": adoToken}, targetNS, "webhook-token")
	assert.NilError(t, err)

	repository := &v1alpha1.Repository{
		ObjectMeta: metav1.ObjectMeta{
			Name: targetNS,
		},
		Spec: v1alpha1.RepositorySpec{
			URL: adoRepo,
		},
	}

	repository.Spec.GitProvider = &v1alpha1.GitProvider{
		//URL:    adoRepo,
		Secret: &v1alpha1.Secret{Name: "webhook-token", Key: "token"},
	}
	err = pacrepo.CreateRepo(ctx, targetNS, run, repository)
	assert.NilError(t, err)

}
