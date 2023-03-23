package gitea

import (
	"context"

	"code.gitea.io/sdk/gitea"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	pacrepo "github.com/openshift-pipelines/pipelines-as-code/test/pkg/repository"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/secret"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func createToken(topts *TestOpts) (string, error) {
	token, _, err := topts.GiteaCNX.Client.CreateAccessToken(gitea.CreateAccessTokenOption{
		Name:   topts.TargetNS,
		Scopes: []string{"repo", "admin:org", "admin:public_key", "admin:repo_hook", "admin:org_hook", "notification", "user", "delete_repo", "package", "admin:gpg_key", "admin:application", "sudo"},
	})
	if err != nil {
		return "", err
	}
	return token.Token, nil
}

func CreateCRD(ctx context.Context, topts *TestOpts) error {
	token, err := createToken(topts)
	if err != nil {
		return err
	}

	if err := secret.Create(ctx, topts.Params, map[string]string{"token": token}, topts.TargetNS, "gitea-secret"); err != nil {
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
		},
	}

	return pacrepo.CreateRepo(ctx, topts.TargetNS, topts.Params, repository)
}
