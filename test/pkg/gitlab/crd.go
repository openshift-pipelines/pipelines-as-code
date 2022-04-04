package gitlab

import (
	"context"
	"os"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	pacrepo "github.com/openshift-pipelines/pipelines-as-code/test/pkg/repository"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/secret"
	"github.com/xanzy/go-gitlab"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func CreateCRD(ctx context.Context, projectinfo *gitlab.Project, run *params.Run, targetNS string) error {
	err := pacrepo.CreateNS(ctx, targetNS, run)
	if err != nil {
		return err
	}

	token, _ := os.LookupEnv("TEST_GITLAB_TOKEN")
	webhookSecret, _ := os.LookupEnv("TEST_EL_WEBHOOK_SECRET")
	apiURL, _ := os.LookupEnv("TEST_GITLAB_API_URL")
	err = secret.Create(ctx, run, map[string]string{"token": token}, targetNS, "webhook-token")
	if err != nil {
		return err
	}
	err = secret.Create(ctx, run, map[string]string{"secret": webhookSecret}, targetNS, "webhook-secret")
	if err != nil {
		return err
	}
	repository := &v1alpha1.Repository{
		ObjectMeta: metav1.ObjectMeta{
			Name: targetNS,
		},
		Spec: v1alpha1.RepositorySpec{
			URL: projectinfo.WebURL,
			GitProvider: &v1alpha1.GitProvider{
				URL:           apiURL,
				Secret:        &v1alpha1.GitProviderSecret{Name: "webhook-token", Key: "token"},
				WebhookSecret: &v1alpha1.GitProviderSecret{Name: "webhook-secret", Key: "secret"},
			},
		},
	}

	return pacrepo.CreateRepo(ctx, targetNS, run, repository)
}
