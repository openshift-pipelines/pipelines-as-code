package webhook

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli/prompt"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/clients"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	testclient "github.com/openshift-pipelines/pipelines-as-code/pkg/test/clients"
	corev1 "k8s.io/api/core/v1"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	rtesting "knative.dev/pkg/reconciler/testing"
)

func TestWebhookUpdateToken(t *testing.T) {
	namespace1 := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "namespace1",
		},
	}
	repo1 := &v1alpha1.Repository{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "repo1",
			Namespace: namespace1.GetName(),
		},
		Spec: v1alpha1.RepositorySpec{
			GitProvider: nil,
		},
	}

	namespace2 := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "namespace2",
		},
	}
	secret2 := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "secret2",
			Namespace: namespace2.GetName(),
		},
		Data: map[string][]byte{
			"hub.token":      []byte(`Yzg5NzhlYmNkNTQwNzYzN2E2ZGExYzhkMTc4NjU0MjY3ZmQ2NmMeZg==`),
			"webhook.secret": []byte(`NTA3MDc=`),
		},
	}
	repo2 := &v1alpha1.Repository{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "repo2",
			Namespace: namespace2.GetName(),
		},
		Spec: v1alpha1.RepositorySpec{
			GitProvider: &v1alpha1.GitProvider{
				Secret: &v1alpha1.Secret{
					Name: "secret2",
					Key:  "hub.token",
				},
				WebhookSecret: &v1alpha1.Secret{
					Name: "repo2",
					Key:  "webhook.secret",
				},
			},
			URL: "https://anurl.com/owner/repo",
		},
	}

	repo3 := &v1alpha1.Repository{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "repo3",
			Namespace: namespace2.GetName(),
		},
		Spec: v1alpha1.RepositorySpec{
			GitProvider: &v1alpha1.GitProvider{},
			URL:         "https://anurl.com/owner/repo1",
		},
	}

	tests := []struct {
		name         string
		askStubs     func(*prompt.AskStubber)
		namespaces   []*corev1.Namespace
		repositories []*v1alpha1.Repository
		secrets      []*corev1.Secret
		repoName     string
		secretName   string // if both secretName and repoName are different
		opts         *cli.PacCliOpts
		wantErr      bool
		wantMsg      string
	}{{
		name:         "Don't use webhook update-token command when GithubApp is configured",
		namespaces:   []*corev1.Namespace{namespace1},
		repositories: []*v1alpha1.Repository{repo1},
		repoName:     "repo1",
		opts: &cli.PacCliOpts{
			Namespace: namespace1.GetName(),
		},
		wantErr: false,
		wantMsg: "ℹ Webhook is not configured for the repository repo1 ",
	}, {
		name:         "Don't use webhook update-token command when GithubApp is configured",
		namespaces:   []*corev1.Namespace{namespace1},
		repositories: []*v1alpha1.Repository{repo3},
		repoName:     "repo3",
		opts: &cli.PacCliOpts{
			Namespace: namespace2.GetName(),
		},
		wantErr: false,
		wantMsg: "! Can not update provider token when git_provider secret is empty",
	}, {
		name:         "list all repositories when GithubApp is configured",
		namespaces:   []*corev1.Namespace{namespace1},
		repositories: []*v1alpha1.Repository{repo1},
		repoName:     "",
		opts: &cli.PacCliOpts{
			Namespace: namespace1.GetName(),
		},
		wantErr: false,
		wantMsg: "ℹ Webhook is not configured for the repository  ",
	}, {
		name:         "list all repository in a namespace where none repo exist",
		namespaces:   []*corev1.Namespace{namespace1},
		repositories: []*v1alpha1.Repository{repo1},
		repoName:     "",
		opts: &cli.PacCliOpts{
			Namespace: "default",
		},
		wantErr: true, // error out here saying no repo found because no repo created in default ns
	}, {
		name:         "invalid repository",
		namespaces:   []*corev1.Namespace{namespace1},
		repositories: []*v1alpha1.Repository{repo1},
		repoName:     "invalidRepo",
		opts: &cli.PacCliOpts{
			Namespace: namespace1.GetName(),
		},
		wantErr: true, // error out with invalidRepo not found because it was not created
	}, {
		name: "Update provider token for existing github webhook",
		askStubs: func(as *prompt.AskStubber) {
			as.StubOne("Yzg5NzhlYmNkNTQwNzYzN2E2ZGExYzhkMTc4NjU0MjY3ZmQ2NmNeZg==")
		},
		namespaces:   []*corev1.Namespace{namespace2},
		repositories: []*v1alpha1.Repository{repo2},
		secrets:      []*corev1.Secret{secret2},
		repoName:     "repo2",
		secretName:   "secret2",
		opts: &cli.PacCliOpts{
			Namespace: namespace2.GetName(),
		},
		wantMsg: "🔑 Secret secret2 has been updated with new personal access token in the namespace2 namespace.\n",
		wantErr: false,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			as, teardown := prompt.InitAskStubber()
			defer teardown()
			if tt.askStubs != nil {
				tt.askStubs(as)
			}
			tdata := testclient.Data{
				Namespaces:   tt.namespaces,
				Repositories: tt.repositories,
				Secret:       tt.secrets,
			}
			ctx, _ := rtesting.SetupFakeContext(t)
			stdata, _ := testclient.SeedTestData(t, ctx, tdata)
			cs := &params.Run{
				Clients: clients.Clients{
					PipelineAsCode: stdata.PipelineAsCode,
					Kube:           stdata.Kube,
				},
				Info: info.Info{Kube: &info.KubeOpts{Namespace: tt.opts.Namespace}},
			}
			io, out := newIOStream()
			if err := update(ctx, tt.opts, cs, io,
				tt.repoName); (err != nil) != tt.wantErr {
				t.Errorf("update() error = %v, wantErr %v", err, tt.wantErr)
			} else {
				if res := cmp.Diff(out.String(), tt.wantMsg); res != "" {
					t.Errorf("Diff %s:", res)
				}
			}
			secretData, err := cs.Clients.Kube.CoreV1().Secrets(tt.opts.Namespace).Get(ctx, tt.secretName, metav1.GetOptions{})
			if err != nil {
				if !apiErrors.IsNotFound(err) {
					t.Error(err)
				}
			} else {
				tokenData, ok := secretData.Data[tt.repositories[0].Spec.GitProvider.Secret.Key]
				if !ok {
					t.Errorf("Failed to update token")
				}
				if string(tokenData) != "Yzg5NzhlYmNkNTQwNzYzN2E2ZGExYzhkMTc4NjU0MjY3ZmQ2NmNeZg==" {
					t.Errorf("provider token has not been updated")
				}
			}
		})
	}
}
