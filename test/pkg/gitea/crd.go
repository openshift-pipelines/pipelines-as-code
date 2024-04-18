package gitea

import (
	"context"

	"code.gitea.io/sdk/gitea"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
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

func CreateCRD(ctx context.Context, topts *TestOpts, spec v1alpha1.RepositorySpec, isGlobal bool) error {
	var ns string
	if isGlobal {
		ns = info.GetNS(ctx)
	} else {
		ns = topts.TargetNS
	}

	if spec.GitProvider != nil && spec.GitProvider.Secret != nil {
		secretName := spec.GitProvider.Secret.Name
		if err := topts.ParamsRun.Clients.Kube.CoreV1().Secrets(ns).Delete(ctx, secretName, metav1.DeleteOptions{}); err == nil {
			if isGlobal {
				topts.ParamsRun.Clients.Log.Infof("Secret global %s has been deleted in %s", secretName, ns)
			} else {
				topts.ParamsRun.Clients.Log.Infof("Secret %s has been deleted in %s", secretName, ns)
			}
		}
		if isGlobal {
			topts.ParamsRun.Clients.Log.Infof("Creating global secret %s for global repository in %s", secretName, ns)
		}
		if err := secret.Create(ctx, topts.ParamsRun, map[string]string{"token": topts.Token}, ns, secretName); err != nil {
			return err
		}
	}

	repository := &v1alpha1.Repository{
		ObjectMeta: metav1.ObjectMeta{
			Name: func() string {
				if isGlobal {
					return info.DefaultGlobalRepoName
				}
				return topts.TargetNS
			}(),
		},
		Spec: spec,
	}

	if isGlobal {
		_ = topts.ParamsRun.Clients.PipelineAsCode.PipelinesascodeV1alpha1().Repositories(ns).Delete(ctx, repository.GetName(), metav1.DeleteOptions{})
	}

	return pacrepo.CreateRepo(ctx, ns, topts.ParamsRun, repository)
}
