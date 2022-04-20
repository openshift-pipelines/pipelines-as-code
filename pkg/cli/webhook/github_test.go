package webhook

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli/prompt"
	ghtesthelper "github.com/openshift-pipelines/pipelines-as-code/pkg/test/github"
	"gotest.tools/v3/assert"
	rtesting "knative.dev/pkg/reconciler/testing"
)

func TestGithubWebhook(t *testing.T) {
	tests := []struct {
		name       string
		wantErrStr string
		askStubs   func(*prompt.AskStubber)
		response   response
	}{
		{
			name: "declined by user",
			askStubs: func(as *prompt.AskStubber) {
				as.StubOne("n")
			},
			response: response{UserDeclined: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, _ := rtesting.SetupFakeContext(t)
			w := Webhook{}
			as, teardown := prompt.InitAskStubber()
			defer teardown()
			if tt.askStubs != nil {
				tt.askStubs(as)
			}
			res, err := w.githubWebhook(ctx)
			if tt.wantErrStr != "" {
				assert.Equal(t, err.Error(), tt.wantErrStr)
				return
			}
			assert.NilError(t, err)
			assert.Equal(t, res.UserDeclined, tt.response.UserDeclined)
		})
	}
}

func TestAskGHWebhookConfig(t *testing.T) {
	tests := []struct {
		name          string
		wantErrStr    string
		askStubs      func(*prompt.AskStubber)
		repoURL       string
		controllerURL string
	}{
		{
			name: "invalid repo format",
			askStubs: func(as *prompt.AskStubber) {
				as.StubOne("invalid-repo")
			},
			wantErrStr: "invalid repository, needs to be of format 'org-name/repo-name'",
		},
		{
			name: "ask all details no defaults",
			askStubs: func(as *prompt.AskStubber) {
				as.StubOne("pac/demo")
				as.StubOne("https://test")
				as.StubOne("webhook-secret")
				as.StubOne("token")
			},
			wantErrStr: "",
		},
		{
			name: "with defaults",
			askStubs: func(as *prompt.AskStubber) {
				as.StubOne("") // use default
				as.StubOne("webhook-secret")
				as.StubOne("token")
			},
			repoURL:       "https:/github.com/pac/demo",
			controllerURL: "https://test",
			wantErrStr:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			as, teardown := prompt.InitAskStubber()
			defer teardown()
			if tt.askStubs != nil {
				tt.askStubs(as)
			}
			_, err := askGHWebhookConfig(tt.repoURL, tt.controllerURL)
			if tt.wantErrStr != "" {
				assert.Equal(t, err.Error(), tt.wantErrStr)
				return
			}
			assert.NilError(t, err)
		})
	}
}

func TestCreate(t *testing.T) {
	fakeclient, mux, _, teardown := ghtesthelper.SetupGH()
	defer teardown()

	// webhook created for repo pac/valid
	mux.HandleFunc("/repos/pac/valid/hooks", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(201)
	})

	// webhook failed for repo pac/invalid
	mux.HandleFunc("/repos/pac/invalid/hooks", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(403)
		_, _ = fmt.Fprint(w, `{"status": "forbidden"}`)
	})

	tests := []struct {
		name      string
		wantErr   bool
		repoName  string
		repoOwner string
	}{
		{
			name:      "webhook created",
			repoOwner: "pac",
			repoName:  "valid",
		},
		{
			name:      "webhook failed",
			repoOwner: "pac",
			repoName:  "invalid",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, _ := rtesting.SetupFakeContext(t)
			gh := gitHubWebhookConfig{
				Client:    fakeclient,
				RepoOwner: tt.repoOwner,
				RepoName:  tt.repoName,
			}
			err := gh.create(ctx)
			if !tt.wantErr {
				assert.NilError(t, err)
			}
		})
	}
}
