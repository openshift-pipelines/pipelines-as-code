package webhook

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli/prompt"
	thelp "github.com/openshift-pipelines/pipelines-as-code/pkg/provider/gitlab/test"
	"gotest.tools/v3/assert"
	rtesting "knative.dev/pkg/reconciler/testing"
)

func TestAskGLWebhookConfig(t *testing.T) {
	tests := []struct {
		name          string
		wantErrStr    string
		askStubs      func(*prompt.AskStubber)
		controllerURL string
	}{
		{
			name: "ask all details no defaults",
			askStubs: func(as *prompt.AskStubber) {
				as.StubOne("id")
				as.StubOne("https://test")
				as.StubOne("webhook-secret")
				as.StubOne("token")
			},
			wantErrStr: "",
		},
		{
			name: "with defaults",
			askStubs: func(as *prompt.AskStubber) {
				as.StubOne("id")
				as.StubOne("webhook-secret")
				as.StubOne("token")
			},
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
			gl := gitLabConfig{}
			err := gl.askGLWebhookConfig(tt.controllerURL)
			if tt.wantErrStr != "" {
				assert.Equal(t, err.Error(), tt.wantErrStr)
				return
			}
			assert.NilError(t, err)
		})
	}
}

func TestGLCreate(t *testing.T) {
	ctx, _ := rtesting.SetupFakeContext(t)
	fakeclient, mux, teardown := thelp.Setup(ctx, t)
	defer teardown()

	// webhook created
	mux.HandleFunc("/projects/11/hooks", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(201)
	})

	// webhook failed
	mux.HandleFunc("/projects/13/hooks", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(403)
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
			ctx, _ := rtesting.SetupFakeContext(t)
			gl := gitLabConfig{
				Client:    fakeclient,
				projectID: tt.projectID,
			}
			err := gl.create(ctx)
			if !tt.wantErr {
				assert.NilError(t, err)
			}
		})
	}
}
