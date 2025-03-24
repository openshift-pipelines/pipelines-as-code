package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/google/go-github/v70/github"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/clients"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/settings"
	testclient "github.com/openshift-pipelines/pipelines-as-code/pkg/test/clients"
	"go.uber.org/zap"
	zapobserver "go.uber.org/zap/zaptest/observer"
	"gotest.tools/v3/assert"
	v12 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	rtesting "knative.dev/pkg/reconciler/testing"
)

func TestConfigureRepository(t *testing.T) {
	observer, _ := zapobserver.New(zap.InfoLevel)
	logger := zap.New(observer).Sugar()

	testEvent := github.RepositoryEvent{Action: github.Ptr("updated")}
	repoUpdatedEvent, err := json.Marshal(testEvent)
	assert.NilError(t, err)

	testRepoName := "test-repo"
	testRepoOwner := "pac"
	testURL := fmt.Sprintf("https://github.com/%v/%v", testRepoOwner, testRepoName)

	testCreateEvent := github.RepositoryEvent{Action: github.Ptr("created"), Repo: &github.Repository{HTMLURL: github.Ptr(testURL)}}
	repoCreateEvent, err := json.Marshal(testCreateEvent)
	assert.NilError(t, err)

	tests := []struct {
		name        string
		request     *http.Request
		eventType   string
		event       []byte
		detected    bool
		configuring bool
		wantErr     string
		expectedNs  string
		nsTemplate  string
		testData    testclient.Data
	}{
		{
			name:        "non supported event",
			event:       []byte{},
			eventType:   "push",
			detected:    false,
			configuring: false,
			wantErr:     "",
			testData:    testclient.Data{},
		},
		{
			name:        "repo updated event",
			event:       repoUpdatedEvent,
			eventType:   "repository",
			detected:    true,
			configuring: false,
			wantErr:     "",
			expectedNs:  "",
			testData:    testclient.Data{},
		},
		{
			name:        "repo create event with no ns template",
			event:       repoCreateEvent,
			eventType:   "repository",
			detected:    true,
			configuring: true,
			wantErr:     "",
			expectedNs:  "test-repo-pipelines",
			testData:    testclient.Data{},
		},
		{
			name:        "repo create event with ns template",
			event:       repoCreateEvent,
			eventType:   "repository",
			detected:    true,
			configuring: true,
			wantErr:     "",
			expectedNs:  "pac-test-repo-ci",
			nsTemplate:  "{{repo_owner}}-{{repo_name}}-ci",
			testData:    testclient.Data{},
		},
		{
			name:        "repo create event with ns already exist",
			event:       repoCreateEvent,
			eventType:   "repository",
			detected:    true,
			configuring: true,
			wantErr:     "",
			expectedNs:  "test-repo-pipelines",
			testData: testclient.Data{
				Namespaces: []*v12.Namespace{
					{
						ObjectMeta: v1.ObjectMeta{
							Name: "test-repo-pipelines",
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, _ := rtesting.SetupFakeContext(t)
			cs, _ := testclient.SeedTestData(t, ctx, tt.testData)
			run := &params.Run{
				Clients: clients.Clients{
					PipelineAsCode: cs.PipelineAsCode,
					Kube:           cs.Kube,
				},
			}
			req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, "URL", bytes.NewReader(tt.event))
			if err != nil {
				t.Fatalf("error creating request: %s", err)
			}
			req.Header.Set("X-Github-Event", tt.eventType)

			infoPac := &info.PacOpts{
				Settings: settings.Settings{
					AutoConfigureNewGitHubRepo:         true,
					AutoConfigureRepoNamespaceTemplate: tt.nsTemplate,
				},
			}
			detected, configuring, err := ConfigureRepository(ctx, run, req, string(tt.event), infoPac, logger)
			assert.Equal(t, detected, tt.detected)
			assert.Equal(t, configuring, tt.configuring)

			if tt.wantErr != "" {
				assert.Equal(t, tt.wantErr, err.Error())
			} else {
				assert.NilError(t, err)
			}

			if tt.configuring {
				ns, err := run.Clients.Kube.CoreV1().Namespaces().Get(ctx, tt.expectedNs, v1.GetOptions{})
				assert.NilError(t, err)
				assert.Equal(t, ns.Name, tt.expectedNs)

				repo, err := run.Clients.PipelineAsCode.PipelinesascodeV1alpha1().Repositories(tt.expectedNs).Get(ctx, tt.expectedNs, v1.GetOptions{})
				assert.NilError(t, err)
				assert.Equal(t, repo.Name, tt.expectedNs)
			}
		})
	}
}

func TestGetNamespace(t *testing.T) {
	tests := []struct {
		name       string
		nsTemplate string
		gitEvent   *github.RepositoryEvent
		want       string
	}{
		{
			name:       "no template",
			nsTemplate: "",
			gitEvent: &github.RepositoryEvent{
				Repo: &github.Repository{
					HTMLURL: github.Ptr("https://github.com/user/pac"),
				},
			},
			want: "pac-pipelines",
		},
		{
			name:       "template",
			nsTemplate: "{{repo_owner}}-{{repo_name}}-ci",
			gitEvent: &github.RepositoryEvent{
				Repo: &github.Repository{
					HTMLURL: github.Ptr("https://github.com/user/pac"),
				},
			},
			want: "user-pac-ci",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := generateNamespaceName(tt.nsTemplate, tt.gitEvent)
			assert.NilError(t, err)
			assert.Equal(t, got, tt.want)
		})
	}
}
