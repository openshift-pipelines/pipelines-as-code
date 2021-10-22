package bitbucketserver

import (
	"path/filepath"
	"strings"
	"testing"

	bbv1 "github.com/gfleury/go-bitbucket-v1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/webvcs"
	bbtest "github.com/openshift-pipelines/pipelines-as-code/pkg/webvcs/bitbucketserver/test"
	"gotest.tools/v3/assert"
	rtesting "knative.dev/pkg/reconciler/testing"
)

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
			event:           bbtest.MakeEvent(nil),
			path:            ".tekton",
			testDirPath:     "../../pipelineascode/testdata/pull_request/",
			contentContains: "kind: PipelineRun",
		},
		{
			name:            "No yaml files in there",
			event:           bbtest.MakeEvent(nil),
			path:            ".tekton",
			testDirPath:     "./",
			contentContains: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, _ := rtesting.SetupFakeContext(t)
			client, mux, tearDown := bbtest.SetupBBServerClient(ctx, t)
			defer tearDown()
			v := &VCS{Client: client}
			bbtest.MuxDirContent(t, mux, tt.event, tt.testDirPath, tt.path)
			content, err := v.GetTektonDir(ctx, tt.event, tt.path)
			if tt.wantErr {
				assert.Assert(t, err != nil,
					"GetTektonDir() error = %v, wantErr %v", err, tt.wantErr)
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

func TestCreateStatus(t *testing.T) {
	pacopts := info.PacOpts{
		VCSToken:        "token",
		ApplicationName: "HELLO APP",
	}
	pullRequestNumber := 10

	tests := []struct {
		name                  string
		status                webvcs.StatusOpts
		expectedDescSubstr    string
		expectedCommentSubstr string
		pacOpts               info.PacOpts
		nilClient             bool
		wantErrSubstr         string
	}{
		{
			name:          "bad/null client",
			nilClient:     true,
			wantErrSubstr: "no token has been set",
		},
		{
			name: "good/skipped",
			status: webvcs.StatusOpts{
				Conclusion: "skipped",
			},
			expectedDescSubstr: "Skipping",
			pacOpts:            pacopts,
		},
		{
			name: "good/neutral",
			status: webvcs.StatusOpts{
				Conclusion: "neutral",
			},
			expectedDescSubstr: "stopped",
			pacOpts:            pacopts,
		},
		{
			name: "good/completed with comment",
			status: webvcs.StatusOpts{
				Conclusion: "success",
				Status:     "completed",
				Text:       "Happy as a bunny",
			},
			expectedDescSubstr:    "validated",
			expectedCommentSubstr: "Happy as a bunny",
			pacOpts:               pacopts,
		},
		{
			name: "good/failed",
			status: webvcs.StatusOpts{
				Conclusion: "failure",
			},
			expectedDescSubstr: "Failed",
			pacOpts:            pacopts,
		},
		{
			name: "good/details url",
			status: webvcs.StatusOpts{
				Conclusion: "failure",
				DetailsURL: "http://fail.com",
			},
			expectedDescSubstr: "Failed",
			pacOpts:            pacopts,
		},
		{
			name: "good/pending",
			status: webvcs.StatusOpts{
				Conclusion: "pending",
			},
			expectedDescSubstr: "started",
			pacOpts:            pacopts,
		},
		{
			name: "good/success",
			status: webvcs.StatusOpts{
				Conclusion: "success",
			},
			expectedDescSubstr: "validated",
			pacOpts:            pacopts,
		},
		{
			name: "good/completed",
			status: webvcs.StatusOpts{
				Conclusion: "completed",
			},
			expectedDescSubstr: "Completed",
			pacOpts:            pacopts,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, _ := rtesting.SetupFakeContext(t)
			client, mux, tearDown := bbtest.SetupBBServerClient(ctx, t)
			defer tearDown()
			if tt.nilClient {
				client = nil
			}
			v := VCS{Client: client, pullRequestNumber: pullRequestNumber}
			event := bbtest.MakeEvent(nil)
			event.EventType = "pull_request"
			bbtest.MuxCreateAndTestCommitStatus(t, mux, event, tt.expectedDescSubstr, tt.status)
			bbtest.MuxCreateComment(t, mux, event, tt.expectedCommentSubstr, pullRequestNumber)
			err := v.CreateStatus(ctx, event, &tt.pacOpts, tt.status)
			if tt.wantErrSubstr != "" {
				assert.ErrorContains(t, err, tt.wantErrSubstr)
				return
			}
			assert.NilError(t, err)
		})
	}
}

func TestGetFileInsideRepo(t *testing.T) {
	tests := []struct {
		name          string
		wantErr       bool
		event         *info.Event
		path          string
		targetbranch  string
		filescontents map[string]string
		assertOutput  string
	}{
		{
			name:         "get file inside repo",
			event:        bbtest.MakeEvent(nil),
			path:         "foo/file.txt",
			assertOutput: "hello moto",
			filescontents: map[string]string{
				"foo/file.txt": "hello moto",
			},
			targetbranch: "main",
		},
		{
			name:         "get file inside default branch",
			event:        bbtest.MakeEvent(&info.Event{DefaultBranch: "yolo"}),
			path:         "foo/file.txt",
			assertOutput: "hello moto",
			filescontents: map[string]string{
				"foo/file.txt": "hello moto",
			},
			targetbranch: "yolo",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, _ := rtesting.SetupFakeContext(t)
			client, mux, tearDown := bbtest.SetupBBServerClient(ctx, t)
			defer tearDown()
			v := &VCS{Client: client, defaultBranchLatestCommit: "1234"}
			bbtest.MuxFiles(t, mux, tt.event, tt.targetbranch, filepath.Dir(tt.path), tt.filescontents)
			fc, err := v.GetFileInsideRepo(ctx, tt.event, tt.path, tt.targetbranch)
			assert.NilError(t, err)
			assert.Equal(t, tt.assertOutput, fc)
		})
	}
}

func TestSetClient(t *testing.T) {
	tests := []struct {
		name          string
		apiURL        string
		opts          *info.PacOpts
		wantErrSubstr string
	}{
		{
			name:          "bad/no username",
			opts:          &info.PacOpts{},
			wantErrSubstr: "no webvcs_api_user",
		},
		{
			name:          "bad/no secret",
			opts:          &info.PacOpts{VCSUser: "foo"},
			wantErrSubstr: "no webvcs_api_secret",
		},
		{
			name:          "bad/no url",
			opts:          &info.PacOpts{VCSUser: "foo", VCSToken: "bar"},
			wantErrSubstr: "no webvcs_api_url",
		},
		{
			name:   "good/url append /rest",
			opts:   &info.PacOpts{VCSUser: "foo", VCSToken: "bar", VCSAPIURL: "https://foo.bar"},
			apiURL: "https://foo.bar/rest",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, _ := rtesting.SetupFakeContext(t)
			v := &VCS{}
			err := v.SetClient(ctx, tt.opts)
			if tt.wantErrSubstr != "" {
				assert.ErrorContains(t, err, tt.wantErrSubstr)
				return
			}
			assert.NilError(t, err)
			assert.Equal(t, tt.apiURL, v.apiURL)
		})
	}
}

func TestGetCommitInfo(t *testing.T) {
	defaultBaseURL := "https://base"
	tests := []struct {
		name          string
		event         *info.Event
		commit        bbv1.Commit
		defaultBranch string
		latestCommit  string
	}{
		{
			name: "Test valid Commit",
			event: &info.Event{
				Owner:      "owner",
				Repository: "repo",
				SHA:        "sha",
			},
			defaultBranch: "branchmain",
			commit: bbv1.Commit{
				Message: "hello moto",
			},
			latestCommit: "latestcommit",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, _ := rtesting.SetupFakeContext(t)
			bbclient, mux, tearDown := bbtest.SetupBBServerClient(ctx, t)
			bbtest.MuxCommitInfo(t, mux, tt.event, tt.commit)
			bbtest.MuxDefaultBranch(t, mux, tt.event, tt.defaultBranch, tt.latestCommit)
			defer tearDown()
			v := &VCS{Client: bbclient, baseURL: defaultBaseURL}
			err := v.GetCommitInfo(ctx, tt.event)
			assert.NilError(t, err)
			assert.Equal(t, tt.defaultBranch, tt.event.DefaultBranch)
			assert.Equal(t, tt.latestCommit, v.defaultBranchLatestCommit)
			assert.Equal(t, tt.commit.Message, tt.event.SHATitle)
		})
	}
}

func TestGetConfig(t *testing.T) {
	v := &VCS{}
	config := v.GetConfig()
	assert.Equal(t, config.TaskStatusTMPL, taskStatusTemplate)
}
