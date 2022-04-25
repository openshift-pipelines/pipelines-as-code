package bitbucketcloud

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/ktrysmt/go-bitbucket"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider"
	bbcloudtest "github.com/openshift-pipelines/pipelines-as-code/pkg/provider/bitbucketcloud/test"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider/bitbucketcloud/types"
	"go.uber.org/zap"
	zapobserver "go.uber.org/zap/zaptest/observer"
	"gotest.tools/v3/assert"
	rtesting "knative.dev/pkg/reconciler/testing"
)

func TestGetConfig(t *testing.T) {
	v := &Provider{}
	config := v.GetConfig()
	assert.Equal(t, config.APIURL, bitbucket.DEFAULT_BITBUCKET_API_BASE_URL)
}

func TestGetTektonDir(t *testing.T) {
	tests := []struct {
		name            string
		event           *info.Event
		path            string
		testDirPath     string
		contentContains string
		wantErr         bool
		removeSuffix    bool
	}{
		{
			name:            "Get Tekton Directory",
			event:           bbcloudtest.MakeEvent(nil),
			path:            ".tekton",
			testDirPath:     "../../pipelineascode/testdata/pull_request/.tekton",
			contentContains: "kind: PipelineRun",
		},
		{
			name:            "No yaml files in there",
			event:           bbcloudtest.MakeEvent(nil),
			path:            ".tekton",
			testDirPath:     "./",
			contentContains: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, _ := rtesting.SetupFakeContext(t)
			bbclient, mux, tearDown := bbcloudtest.SetupBBCloudClient(t)
			defer tearDown()
			v := &Provider{Client: bbclient}
			bbcloudtest.MuxDirContent(t, mux, tt.event, tt.testDirPath, tt.path)
			content, err := v.GetTektonDir(ctx, tt.event, tt.path)
			if tt.wantErr {
				assert.Assert(t, err != nil, "GetTektonDir() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.contentContains == "" {
				assert.Equal(t, content, "")
				return
			}
			assert.Assert(t, strings.Contains(content, tt.contentContains), "content %s doesn't have %s", content, tt.contentContains)
		})
	}
}

func TestSetClient(t *testing.T) {
	tests := []struct {
		name          string
		wantErrSubstr string
		event         *info.Event
		token         string
	}{
		{
			name: "set token",
			event: &info.Event{
				Provider: &info.Provider{
					Token: "token",
					User:  "user",
				},
			},
		},
		{
			name: "no user",
			event: &info.Event{
				Provider: &info.Provider{
					Token: "token",
					User:  "",
				},
			},
			wantErrSubstr: "no git_provider.user",
		},
		{
			name: "no token",
			event: &info.Event{
				Provider: &info.Provider{
					Token: "",
					User:  "user",
				},
			},
			wantErrSubstr: "no git_provider.secret",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, _ := rtesting.SetupFakeContext(t)
			v := Provider{}
			err := v.SetClient(ctx, tt.event)
			if tt.wantErrSubstr != "" {
				assert.ErrorContains(t, err, tt.wantErrSubstr)
				return
			}
			assert.Equal(t, tt.event.Provider.Token, *v.Token)
			assert.Equal(t, tt.event.Provider.User, *v.Username)
		})
	}
}

func TestGetCommitInfo(t *testing.T) {
	tests := []struct {
		name       string
		event      *info.Event
		wantErr    bool
		commitinfo types.Commit
		repoinfo   *bitbucket.Repository
	}{
		{
			name:  "Get commit info",
			event: bbcloudtest.MakeEvent(nil),
			commitinfo: types.Commit{
				Hash: "convertedcommit",
				Links: types.Links{
					HTML: types.HTMLLink{
						HRef: "https://everywhereigo",
					},
				},
				Message: "Das Commit",
				Author:  types.Author{},
			},
			repoinfo: &bitbucket.Repository{
				Mainbranch: bitbucket.RepositoryBranch{Name: "branshe"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, _ := rtesting.SetupFakeContext(t)
			bbclient, mux, tearDown := bbcloudtest.SetupBBCloudClient(t)
			defer tearDown()
			v := &Provider{Client: bbclient}
			bbcloudtest.MuxCommits(t, mux, tt.event, []types.Commit{
				tt.commitinfo,
			})
			bbcloudtest.MuxRepoInfo(t, mux, tt.event, tt.repoinfo)

			if err := v.GetCommitInfo(ctx, tt.event); (err != nil) != tt.wantErr {
				t.Errorf("GetCommitInfo() error = %v, wantErr %v", err, tt.wantErr)
			}

			assert.Equal(t, tt.repoinfo.Mainbranch.Name, tt.event.DefaultBranch)
			assert.Equal(t, tt.commitinfo.Links.HTML.HRef, tt.event.SHAURL)
			assert.Equal(t, tt.commitinfo.Hash, tt.event.SHA)
			assert.Equal(t, tt.commitinfo.Message, tt.event.SHATitle)
		})
	}
}

func TestCreateStatus(t *testing.T) {
	tests := []struct {
		name                  string
		wantErr               bool
		status                provider.StatusOpts
		expectedDescSubstr    string
		expectedCommentSubstr string
	}{
		{
			name: "skipped",
			status: provider.StatusOpts{
				Conclusion: "skipped",
			},
			expectedDescSubstr: "Skipping",
		},
		{
			name: "neutral",
			status: provider.StatusOpts{
				Conclusion: "neutral",
			},
			expectedDescSubstr: "stopped",
		},
		{
			name: "completed with comment",
			status: provider.StatusOpts{
				Conclusion: "success",
				Status:     "completed",
				Text:       "Happy as a bunny",
			},
			expectedDescSubstr:    "validated",
			expectedCommentSubstr: "Happy as a bunny",
		},
		{
			name: "failed",
			status: provider.StatusOpts{
				Conclusion: "failure",
			},
			expectedDescSubstr: "Failed",
		},
		{
			name: "details url",
			status: provider.StatusOpts{
				Conclusion: "failure",
				DetailsURL: "http://fail.com",
			},
			expectedDescSubstr: "Failed",
		},
		{
			name: "pending",
			status: provider.StatusOpts{
				Conclusion: "pending",
			},
			expectedDescSubstr: "started",
		},
		{
			name: "success",
			status: provider.StatusOpts{
				Conclusion: "success",
			},
			expectedDescSubstr: "validated",
		},
		{
			name: "completed",
			status: provider.StatusOpts{
				Conclusion: "completed",
			},
			expectedDescSubstr: "Completed",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, _ := rtesting.SetupFakeContext(t)
			bbclient, mux, tearDown := bbcloudtest.SetupBBCloudClient(t)
			defer tearDown()
			v := &Provider{Client: bbclient}
			event := bbcloudtest.MakeEvent(nil)
			event.EventType = "pull_request"
			event.Provider.Token = "token"

			bbcloudtest.MuxCreateCommitstatus(t, mux, event, tt.expectedDescSubstr, tt.status)
			bbcloudtest.MuxCreateComment(t, mux, event, tt.expectedCommentSubstr)

			err := v.CreateStatus(ctx, event, &info.PacOpts{
				ApplicationName: "HELLO APP",
			}, tt.status)
			assert.NilError(t, err)
		})
	}
}

func TestProvider_Detect(t *testing.T) {
	tests := []struct {
		name          string
		wantErrString string
		isBC          bool
		processReq    bool
		event         interface{}
		eventType     string
		wantReason    string
	}{
		{
			name:       "not a bitbucket cloud Event",
			eventType:  "",
			isBC:       false,
			processReq: false,
		},
		{
			name:       "invalid bitbucket cloud Event",
			eventType:  "validator",
			isBC:       false,
			processReq: false,
		},
		{
			event: types.PushRequestEvent{
				Push: types.Push{
					Changes: []types.Change{
						{
							New: types.ChangeType{Name: "new"},
							Old: types.ChangeType{Name: "old"},
						},
					},
				},
			},
			eventType:  "repo:push",
			isBC:       true,
			processReq: true,
			name:       "push event",
		},
		{
			name:       "pull_request event",
			event:      types.PullRequestEvent{},
			eventType:  "pullrequest:created",
			isBC:       true,
			processReq: true,
		},
		{
			name:       "updated pull_request event",
			event:      types.PullRequestEvent{},
			eventType:  "pullrequest:updated",
			isBC:       true,
			processReq: true,
		},
		{
			name: "retest comment",
			event: types.PullRequestEvent{
				Comment: types.Comment{
					Content: types.Content{
						Raw: "/retest",
					},
				},
			},
			eventType:  "pullrequest:comment_created",
			isBC:       true,
			processReq: true,
		},
		{
			name: "random comment",
			event: types.PullRequestEvent{
				Comment: types.Comment{
					Content: types.Content{
						Raw: "abc",
					},
				},
			},
			eventType:  "pullrequest:comment_created",
			isBC:       true,
			processReq: false,
		},
		{
			name: "ok-to-test comment",
			event: types.PullRequestEvent{
				Comment: types.Comment{
					Content: types.Content{
						Raw: "/ok-to-test",
					},
				},
			},
			eventType:  "pullrequest:comment_created",
			isBC:       true,
			processReq: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bprovider := Provider{}
			logger := getLogger()

			jeez, err := json.Marshal(tt.event)
			if err != nil {
				assert.NilError(t, err)
			}

			header := &http.Header{}
			header.Set("X-Event-Key", tt.eventType)

			isBS, processReq, _, reason, err := bprovider.Detect(header, string(jeez), logger)
			if tt.wantErrString != "" {
				assert.ErrorContains(t, err, tt.wantErrString)
				return
			}
			if tt.wantReason != "" {
				assert.Assert(t, strings.Contains(reason, tt.wantReason))
				return
			}
			assert.NilError(t, err)
			assert.Equal(t, tt.isBC, isBS)
			assert.Equal(t, tt.processReq, processReq)
		})
	}
}

func getLogger() *zap.SugaredLogger {
	observer, _ := zapobserver.New(zap.InfoLevel)
	logger := zap.New(observer).Sugar()
	return logger
}
