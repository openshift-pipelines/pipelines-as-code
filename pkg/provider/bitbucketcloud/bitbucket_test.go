package bitbucketcloud

import (
	"strings"
	"testing"

	"github.com/ktrysmt/go-bitbucket"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/settings"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/triggertype"
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
		name                 string
		event                *info.Event
		testDirPath          string
		contentContains      string
		wantErr              string
		removeSuffix         bool
		provenance           string
		filterMessageSnippet string
	}{
		{
			name:                 "Get Tekton Directory on pull request",
			event:                bbcloudtest.MakeEvent(&info.Event{TriggerTarget: triggertype.PullRequest}),
			testDirPath:          "../../pipelineascode/testdata/pull_request/.tekton",
			contentContains:      "kind: PipelineRun",
			filterMessageSnippet: "Using PipelineRun definition from source pull_request commit SHA",
		},
		{
			name:                 "Get Tekton Directory on push",
			event:                bbcloudtest.MakeEvent(&info.Event{TriggerTarget: triggertype.Push}),
			testDirPath:          "../../pipelineascode/testdata/pull_request/.tekton",
			contentContains:      "kind: PipelineRun",
			filterMessageSnippet: "Using PipelineRun definition from source push commit SHA",
		},
		{
			name:                 "Get Tekton Directory Mainbranch",
			event:                bbcloudtest.MakeEvent(&info.Event{DefaultBranch: "main"}),
			testDirPath:          "../../pipelineascode/testdata/pull_request/.tekton",
			contentContains:      "kind: PipelineRun",
			provenance:           "default_branch",
			filterMessageSnippet: "Using PipelineRun definition from default_branch: main",
		},
		{
			name:            "Get Tekton Directory and subdirectory",
			event:           bbcloudtest.MakeEvent(nil),
			testDirPath:     "../../pipelineascode/testdata/subdir/.tekton",
			contentContains: "kind: PipelineRun",
		},
		{
			name:        "Bad yaml files in there",
			event:       bbcloudtest.MakeEvent(nil),
			testDirPath: "../../pipelineascode/testdata/bad_yaml/.tekton",
			wantErr:     "error unmarshalling yaml file .tekton/badyaml.yaml: yaml: line 2: did not find expected key",
		},
		{
			name:            "No yaml files in there",
			event:           bbcloudtest.MakeEvent(nil),
			testDirPath:     "../../pipelineascode/testdata/no_yaml/.tekton",
			contentContains: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			observer, exporter := zapobserver.New(zap.InfoLevel)
			fakelogger := zap.New(observer).Sugar()
			ctx, _ := rtesting.SetupFakeContext(t)
			bbclient, mux, tearDown := bbcloudtest.SetupBBCloudClient(t)
			defer tearDown()
			v := &Provider{Logger: fakelogger, bbClient: bbclient}
			bbcloudtest.MuxDirContent(t, mux, tt.event, tt.testDirPath, tt.provenance)
			content, err := v.GetTektonDir(ctx, tt.event, ".tekton", tt.provenance)
			if tt.wantErr != "" {
				assert.Assert(t, err != nil, "expected error %s, got %v", tt.wantErr, err)
				assert.ErrorContains(t, err, tt.wantErr)
				return
			}
			if tt.contentContains == "" {
				assert.Equal(t, content, "")
				return
			}
			assert.Assert(t, strings.Contains(content, tt.contentContains), "content %s doesn't have %s", content, tt.contentContains)

			if tt.filterMessageSnippet != "" {
				gotcha := exporter.FilterMessageSnippet(tt.filterMessageSnippet)
				assert.Assert(t, gotcha.Len() > 0, "expected to find %s in logs, found %v", tt.filterMessageSnippet, exporter.All())
			}
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
			err := v.SetClient(ctx, nil, tt.event, nil, nil)
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
		{
			name: "Get commit info no SHA",
			event: bbcloudtest.MakeEvent(&info.Event{
				Organization: "org",
				Repository:   "repo",
				HeadBranch:   "hello",
				SHA:          "none",
			}),
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
			v := &Provider{bbClient: bbclient}
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
			v := &Provider{
				bbClient: bbclient,
				run:      params.New(),
				pacInfo: &info.PacOpts{
					Settings: settings.Settings{
						ApplicationName: settings.PACApplicationNameDefaultValue,
					},
				},
			}
			event := bbcloudtest.MakeEvent(nil)
			event.EventType = "pull_request"
			event.Provider.Token = "token"

			bbcloudtest.MuxCreateCommitstatus(t, mux, event, tt.expectedDescSubstr, tt.status)
			bbcloudtest.MuxCreateComment(t, mux, event, tt.expectedCommentSubstr)

			err := v.CreateStatus(ctx, event, tt.status)
			assert.NilError(t, err)
		})
	}
}
