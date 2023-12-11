package webhook

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli/prompt"
	thelp "github.com/openshift-pipelines/pipelines-as-code/pkg/provider/gitlab/test"
	"gotest.tools/v3/assert"
	rtesting "knative.dev/pkg/reconciler/testing"
)

func TestAskGLWebhookConfig(t *testing.T) {
	//nolint
	io, _, _, _ := cli.IOTest()
	tests := []struct {
		name                string
		wantErrStr          string
		askStubs            func(*prompt.AskStubber)
		providerURL         string
		controllerURL       string
		repoURL             string
		personalaccesstoken string
	}{
		{
			name: "ask all details no defaults",
			askStubs: func(as *prompt.AskStubber) {
				as.StubOne("https://gitlab.com/pac/test")
				as.StubOne("id")
				as.StubOne("https://test")
				as.StubOne("webhook-secret")
				as.StubOne("token")
				as.StubOne("https://gl.pac.test")
			},
			wantErrStr: "",
		},
		{
			name: "with defaults",
			askStubs: func(as *prompt.AskStubber) {
				as.StubOne("id")
				as.StubOne(true)
				as.StubOne("webhook-secret")
				as.StubOne("token")
			},
			repoURL:       "https://gitlab.com/pac/demo",
			controllerURL: "https://test",
			providerURL:   "https://gl.pac.test",
			wantErrStr:    "",
		},
		{
			name: "with defaults and given personalaccesstoken",
			askStubs: func(as *prompt.AskStubber) {
				as.StubOne("id")
				as.StubOne(true)
				as.StubOne("webhook-secret")
				as.StubOne("token")
			},
			repoURL:             "https://gitlab.com/pac/demo",
			controllerURL:       "https://test",
			providerURL:         "https://gl.pac.test",
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
			gl := gitLabConfig{IOStream: io}
			err := gl.askGLWebhookConfig(tt.repoURL, tt.controllerURL, tt.providerURL, tt.personalaccesstoken)
			if tt.wantErrStr != "" {
				assert.Equal(t, err.Error(), tt.wantErrStr)
				return
			}
			assert.NilError(t, err)
		})
	}
}

func TestGLCreate(t *testing.T) {
	_, _ = rtesting.SetupFakeContext(t)
	fakeclient, mux, teardown := thelp.Setup(t)
	defer teardown()
	//nolint
	io, _, _, _ := cli.IOTest()

	// webhook created
	mux.HandleFunc("/projects/11/hooks", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = fmt.Fprint(w, `{"status": "ok"}`)
	})

	// webhook failed
	mux.HandleFunc("/projects/13/hooks", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = fmt.Fprint(w, `{"status": "forbidden"}`)
	})

	tests := []struct {
		name      string
		projectID string
		wantErr   bool
	}{
		{
			name:      "webhook created",
			projectID: "11",
			wantErr:   false,
		},
		{
			name:      "webhook failed",
			projectID: "13",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gl := gitLabConfig{
				IOStream:  io,
				Client:    fakeclient,
				projectID: tt.projectID,
			}
			err := gl.create()
			if !tt.wantErr {
				assert.NilError(t, err)
			}
		})
	}
}
