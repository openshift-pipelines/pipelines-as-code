package gitlab

import (
	"context"
	"os"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/options"
	pacrepo "github.com/openshift-pipelines/pipelines-as-code/test/pkg/repository"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/secret"
	gitlab "gitlab.com/gitlab-org/api/client-go"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func CreateCRD(ctx context.Context, projectinfo *gitlab.Project, run *params.Run, opts options.E2E, targetNS string, incomings *[]v1alpha1.Incoming) error {
	if err := pacrepo.CreateNS(ctx, targetNS, run); err != nil {
		return err
	}

	token, _ := os.LookupEnv("TEST_GITLAB_TOKEN")
	webhookSecret, _ := os.LookupEnv("TEST_EL_WEBHOOK_SECRET")
	apiURL, _ := os.LookupEnv("TEST_GITLAB_API_URL")
	if err := secret.Create(ctx, run, map[string]string{"token": token}, targetNS, "webhook-token"); err != nil {
		return err
	}
	if err := secret.Create(ctx, run, map[string]string{"secret": webhookSecret}, targetNS, "webhook-secret"); err != nil {
		return err
	}
	repository := &v1alpha1.Repository{
		ObjectMeta: metav1.ObjectMeta{
			Name: targetNS,
		},
		Spec: v1alpha1.RepositorySpec{
			Settings: &opts.Settings,
			URL:      projectinfo.WebURL,
			GitProvider: &v1alpha1.GitProvider{
				Type:          "gitlab",
				URL:           apiURL,
				Secret:        &v1alpha1.Secret{Name: "webhook-token", Key: "token"},
				WebhookSecret: &v1alpha1.Secret{Name: "webhook-secret", Key: "secret"},
			},
			Incomings: incomings,
		},
	}

	return pacrepo.CreateRepo(ctx, targetNS, run, repository)
}
