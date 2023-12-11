package webhook

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli/prompt"
	ghtesthelper "github.com/openshift-pipelines/pipelines-as-code/pkg/test/github"
	"gotest.tools/v3/assert"
	rtesting "knative.dev/pkg/reconciler/testing"
)

func TestAskGHWebhookConfig(t *testing.T) {
	//nolint
	io, _, _, _ := cli.IOTest()
	tests := []struct {
		name                string
		wantErrStr          string
		askStubs            func(*prompt.AskStubber)
		repoURL             string
		controllerURL       string
		personalaccesstoken string
	}{
		{
			name: "invalid repo format",
			askStubs: func(as *prompt.AskStubber) {
				as.StubOne("invalid-repo")
			},
			wantErrStr: "invalid repo url at least a organization/project and a repo needs to be specified: invalid-repo",
		},
		{
			name: "ask all details no defaults",
			askStubs: func(as *prompt.AskStubber) {
				as.StubOne("https://github.com/pac/test")
				as.StubOne("https://controller.url")
				as.StubOne("webhook-secret")
				as.StubOne("token")
			},
			wantErrStr: "",
		},
		{
			name: "with defaults",
			askStubs: func(as *prompt.AskStubber) {
				as.StubOne(true)
				as.StubOne("webhook-secret")
				as.StubOne("token")
			},
			repoURL:       "https://github.com/pac/demo",
			controllerURL: "https://test",
			wantErrStr:    "",
		},
		{
			name: "with defaults and a slash",
			askStubs: func(as *prompt.AskStubber) {
				as.StubOne(true)
				as.StubOne("webhook-secret")
				as.StubOne("token")
			},
			repoURL:       "https://github.com/pac/demo/",
			controllerURL: "https://test",
			wantErrStr:    "",
		},
		{
			name: "with personalaccesstoken",
			askStubs: func(as *prompt.AskStubber) {
				as.StubOne(true)
				as.StubOne("webhook-secret")
				as.StubOne("token")
			},
			repoURL:             "https://github.com/pac/demo/",
			controllerURL:       "https://test",
			personalaccesstoken: "Yzg5NzhlYmNkNTQwNzYzN2E2ZGExYzhkMTc4NjU0MjY3ZmQ2NmMeZg==",
			wantErrStr:          "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			as, teardown := prompt.InitAskStubber()
			defer teardown()
			if tt.askStubs != nil {
				tt.askStubs(as)
			}
			gh := gitHubConfig{IOStream: io}
			err := gh.askGHWebhookConfig(tt.repoURL, tt.controllerURL, "", tt.personalaccesstoken)
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
	//nolint
	io, _, _, _ := cli.IOTest()

	// webhook created for repo pac/valid
	mux.HandleFunc("/repos/pac/valid/hooks", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})

	// webhook failed for repo pac/invalid
	mux.HandleFunc("/repos/pac/invalid/hooks", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
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
			gh := gitHubConfig{
				IOStream:  io,
				Client:    fakeclient,
				repoOwner: tt.repoOwner,
				repoName:  tt.repoName,
			}
			err := gh.create(ctx)
			if !tt.wantErr {
				assert.NilError(t, err)
			}
		})
	}
}
