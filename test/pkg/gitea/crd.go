package gitea

import (
	"context"

	"code.gitea.io/sdk/gitea"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	pacrepo "github.com/openshift-pipelines/pipelines-as-code/test/pkg/repository"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/secret"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CreateToken creates gitea token with all scopes.
func CreateToken(topts *TestOpts) (string, error) {
	token, _, err := topts.GiteaCNX.Client.CreateAccessToken(gitea.CreateAccessTokenOption{
		Name:   topts.TargetNS,
		Scopes: []gitea.AccessTokenScope{gitea.AccessTokenScopeAll},
	})
	if err != nil {
		return "", err
	}
	return token.Token, nil
}

func CreateCRD(ctx context.Context, topts *TestOpts) error {
	if err := secret.Create(ctx, topts.ParamsRun, map[string]string{"token": topts.Token}, topts.TargetNS, "gitea-secret"); err != nil {
		return err
	}
	repository := &v1alpha1.Repository{
		ObjectMeta: metav1.ObjectMeta{
			Name: topts.TargetNS,
		},
		Spec: v1alpha1.RepositorySpec{
			URL: topts.GitHTMLURL,
			GitProvider: &v1alpha1.GitProvider{
				Type: "gitea",
				// caveat this assume gitea running on the same cluster, which
				// we do and need for e2e tests but that may be changed somehow
				URL:    topts.InternalGiteaURL,
				Secret: &v1alpha1.Secret{Name: "gitea-secret", Key: "token"},
			},
			ConcurrencyLimit: topts.ConcurrencyLimit,
			Params:           topts.RepoCRParams,
			Settings:         topts.Settings,
		},
	}

	return pacrepo.CreateRepo(ctx, topts.TargetNS, topts.ParamsRun, repository)
}
