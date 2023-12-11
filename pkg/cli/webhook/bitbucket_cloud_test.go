package webhook

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli/prompt"
	bbcloudtest "github.com/openshift-pipelines/pipelines-as-code/pkg/provider/bitbucketcloud/test"
	"gotest.tools/v3/assert"
)

func TestAskBBWebhookConfig(t *testing.T) {
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
				as.StubOne("https://bitbucket.org/pac/test")
				as.StubOne("https://controller.url")
				as.StubOne("user")
				as.StubOne("token")
			},
			wantErrStr: "",
		},
		{
			name: "with defaults",
			askStubs: func(as *prompt.AskStubber) {
				as.StubOne(true)
				as.StubOne("webhook-secret")
				as.StubOne("user")
				as.StubOne("token")
				as.StubOne("")
			},
			repoURL:       "https://bitbucket.org/pac/demo",
			controllerURL: "https://test",
			wantErrStr:    "",
		},
		{
			name: "with personalaccesstoken",
			askStubs: func(as *prompt.AskStubber) {
				as.StubOne(true)
				as.StubOne("webhook-secret")
				as.StubOne("user")
				as.StubOne("token")
				as.StubOne("")
			},
			repoURL:             "https://bitbucket.org/pac/demo",
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
			bb := bitbucketCloudConfig{IOStream: io}
			err := bb.askBBWebhookConfig(tt.repoURL, tt.controllerURL, "", tt.personalaccesstoken)
			if tt.wantErrStr != "" {
				assert.Equal(t, err.Error(), tt.wantErrStr)
				return
			}
			assert.NilError(t, err)
		})
	}
}

func TestBBCreate(t *testing.T) {
	bbclient, mux, tearDown := bbcloudtest.SetupBBCloudClient(t)
	defer tearDown()
	//nolint
	io, _, _, _ := cli.IOTest()

	// webhook created for repo pac/repo
	mux.HandleFunc("/repositories/pac/repo/hooks", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = fmt.Fprint(w, `{"type": "ok"}`)
	})

	// webhook failed for repo pac/invalid
	mux.HandleFunc("/repositories/pac/invalid/hooks", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = fmt.Fprint(w, `{"type": "error"}`)
	})

	tests := []struct {
		name      string
		wantErr   bool
		repoName  string
		repoOwner string
		apiURL    string
	}{
		{
			name:      "webhook created",
			repoOwner: "pac",
			repoName:  "repo",
		},
		{
			name:      "webhook failed",
			repoOwner: "pac",
			repoName:  "invalid",
			wantErr:   true,
			apiURL:    "https://api.bitbucket.org/2.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bb := bitbucketCloudConfig{
				IOStream:      io,
				Client:        bbclient,
				repoOwner:     tt.repoOwner,
				repoName:      tt.repoName,
				APIURL:        tt.apiURL,
				controllerURL: "https://bb.pac.test",
			}
			err := bb.create()
			if !tt.wantErr {
				assert.NilError(t, err)
			}
		})
	}
}
