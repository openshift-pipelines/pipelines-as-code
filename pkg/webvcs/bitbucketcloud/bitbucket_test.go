package bitbucketcloud

import (
	"strings"
	"testing"

	"github.com/ktrysmt/go-bitbucket"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/webvcs"
	bbcloudtest "github.com/openshift-pipelines/pipelines-as-code/pkg/webvcs/bitbucketcloud/test"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/webvcs/bitbucketcloud/types"
	"gotest.tools/v3/assert"
	rtesting "knative.dev/pkg/reconciler/testing"
)

func TestGetConfig(t *testing.T) {
	v := &VCS{}
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
			testDirPath:     "../../pipelineascode/testdata/pull_request/",
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
			v := &VCS{Client: bbclient}
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
	ctx, _ := rtesting.SetupFakeContext(t)
	v := VCS{}
	token := "HELLOTOKEN"
	v.SetClient(ctx, &info.PacOpts{VCSToken: token})
	assert.Equal(t, token, *v.Token)
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
			v := &VCS{Client: bbclient}
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
		status                webvcs.StatusOpts
		expectedDescSubstr    string
		expectedCommentSubstr string
	}{
		{
			name: "skipped",
			status: webvcs.StatusOpts{
				Conclusion: "skipped",
			},
			expectedDescSubstr: "Skipping",
		},
		{
			name: "neutral",
			status: webvcs.StatusOpts{
				Conclusion: "neutral",
			},
			expectedDescSubstr: "stopped",
		},
		{
			name: "completed with comment",
			status: webvcs.StatusOpts{
				Conclusion: "success",
				Status:     "completed",
				Text:       "Happy as a bunny",
			},
			expectedDescSubstr:    "validated",
			expectedCommentSubstr: "Happy as a bunny",
		},
		{
			name: "failed",
			status: webvcs.StatusOpts{
				Conclusion: "failure",
			},
			expectedDescSubstr: "Failed",
		},
		{
			name: "details url",
			status: webvcs.StatusOpts{
				Conclusion: "failure",
				DetailsURL: "http://fail.com",
			},
			expectedDescSubstr: "Failed",
		},
		{
			name: "pending",
			status: webvcs.StatusOpts{
				Conclusion: "pending",
			},
			expectedDescSubstr: "started",
		},
		{
			name: "success",
			status: webvcs.StatusOpts{
				Conclusion: "success",
			},
			expectedDescSubstr: "validated",
		},
		{
			name: "completed",
			status: webvcs.StatusOpts{
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
			v := &VCS{Client: bbclient}
			event := bbcloudtest.MakeEvent(nil)
			event.EventType = "pull_request"

			bbcloudtest.MuxCreateCommitstatus(t, mux, event, tt.expectedDescSubstr, tt.status)
			bbcloudtest.MuxCreateComment(t, mux, event, tt.expectedCommentSubstr)

			err := v.CreateStatus(ctx, event, &info.PacOpts{
				VCSToken:        "token",
				ApplicationName: "HELLO APP",
			}, tt.status)
			assert.NilError(t, err)
		})
	}
}
