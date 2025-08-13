package bitbucketdatacenter

import (
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/settings"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/triggertype"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider"
	bbtest "github.com/openshift-pipelines/pipelines-as-code/pkg/provider/bitbucketdatacenter/test"

	"github.com/jenkins-x/go-scm/scm"
	"go.uber.org/zap"
	zapobserver "go.uber.org/zap/zaptest/observer"
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
		wantDirAPIErr   bool
		wantFilesAPIErr bool
		wantErr         string
		removeSuffix    bool
	}{
		{
			name:            "good/get tekton directory",
			event:           bbtest.MakeEvent(nil),
			path:            ".tekton",
			testDirPath:     "../../pipelineascode/testdata/pull_request/.tekton",
			contentContains: "kind: PipelineRun",
		},
		{
			name:            "bad/no yaml files in there",
			event:           bbtest.MakeEvent(nil),
			path:            ".tekton",
			testDirPath:     "./",
			contentContains: "",
		},
		{
			name:        "bad/badly formatted yaml",
			event:       bbtest.MakeEvent(nil),
			path:        ".tekton",
			testDirPath: "../../pipelineascode/testdata/bad_yaml/.tekton",
			wantErr:     "error unmarshalling yaml file .tekton/badyaml.yaml: yaml: line 2: did not find expected key",
		},
		{
			name:          "bad/get dir api error",
			event:         bbtest.MakeEvent(nil),
			path:          ".tekton",
			testDirPath:   "../../pipelineascode/testdata/pull_request/.tekton",
			wantDirAPIErr: true,
			wantErr:       "cannot list content of .tekton directory: No message available",
		},
		{
			name:            "bad/get files api error",
			event:           bbtest.MakeEvent(nil),
			path:            ".tekton",
			testDirPath:     "../../pipelineascode/testdata/pull_request/.tekton",
			wantFilesAPIErr: true,
			wantErr:         "cannot find .tekton/pipeline.yaml inside the repo repository: No message available",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			observer, _ := zapobserver.New(zap.InfoLevel)
			logger := zap.New(observer).Sugar()
			ctx, _ := rtesting.SetupFakeContext(t)
			client, mux, tearDown, tURL := bbtest.SetupBBDataCenterClient()
			defer tearDown()
			v := &Provider{Logger: logger, baseURL: tURL, scmClient: client, projectKey: tt.event.Organization}
			bbtest.MuxDirContent(t, mux, tt.event, tt.testDirPath, tt.path, tt.wantDirAPIErr, tt.wantFilesAPIErr)
			content, err := v.GetTektonDir(ctx, tt.event, tt.path, "")
			if tt.wantErr != "" {
				assert.Assert(t, err != nil, "we should have get an error here")
				assert.ErrorContains(t, err, tt.wantErr)
				return
			}
			assert.NilError(t, err)
			assert.Assert(t, strings.Contains(content, tt.contentContains), "content %s doesn't have %s", content, tt.contentContains)
		})
	}
}

func TestCreateStatus(t *testing.T) {
	pacopts := info.PacOpts{
		Settings: settings.Settings{
			ApplicationName: "HELLO APP",
		},
	}
	pullRequestNumber := 10

	tests := []struct {
		name                  string
		status                provider.StatusOpts
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
			status: provider.StatusOpts{
				Conclusion: "skipped",
				Text:       "Skipping",
			},

			pacOpts: pacopts,
		},
		{
			name: "good/neutral",
			status: provider.StatusOpts{
				Conclusion: "neutral",
				Text:       "stopped",
			},

			pacOpts: pacopts,
		},
		{
			name: "good/completed with comment",
			status: provider.StatusOpts{
				Conclusion: "success",
				Status:     "completed",
				Text:       "validated",
			},

			expectedCommentSubstr: "Happy as a bunny",
			pacOpts:               pacopts,
		},
		{
			name: "good/failed",
			status: provider.StatusOpts{
				Conclusion: "failure",
				Text:       "Failed",
			},

			pacOpts: pacopts,
		},
		{
			name: "good/details url",
			status: provider.StatusOpts{
				Conclusion: "failure",
				DetailsURL: "http://fail.com",
				Text:       "Failed",
			},

			pacOpts: pacopts,
		},
		{
			name: "good/pending",
			status: provider.StatusOpts{
				Conclusion: "pending",
				Text:       "started",
			},

			pacOpts: pacopts,
		},
		{
			name: "good/success",
			status: provider.StatusOpts{
				Conclusion: "success",
				Text:       "validated",
			},

			pacOpts: pacopts,
		},
		{
			name: "good/completed",
			status: provider.StatusOpts{
				Conclusion: "completed",
				Text:       "Completed",
			},

			pacOpts: pacopts,
		},
		{
			name: "good/pending",
			status: provider.StatusOpts{
				Conclusion: "pending",
				Status:     "queued",
				Text:       "Pending approval, waiting for an /ok-to-test",
			},

			pacOpts: pacopts,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, _ := rtesting.SetupFakeContext(t)
			client, mux, tearDown, tURL := bbtest.SetupBBDataCenterClient()
			defer tearDown()
			if tt.nilClient {
				client = nil
			}
			event := bbtest.MakeEvent(nil)
			event.EventType = "pull_request"
			event.Provider.Token = "token"
			v := &Provider{
				baseURL:           tURL,
				scmClient:         client,
				pullRequestNumber: pullRequestNumber,
				projectKey:        event.Organization,
				run:               &params.Run{},
				pacInfo:           &tt.pacOpts,
			}
			bbtest.MuxCreateAndTestCommitStatus(t, mux, event, tt.status.Text, tt.status)
			bbtest.MuxCreateComment(t, mux, event, tt.expectedCommentSubstr, pullRequestNumber)
			err := v.CreateStatus(ctx, event, tt.status)
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
		wantErr       string
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
		{
			name:         "bad/get files api error",
			event:        bbtest.MakeEvent(&info.Event{DefaultBranch: "yolo"}),
			path:         "foo/file.txt",
			assertOutput: "hello moto",
			filescontents: map[string]string{
				"foo/file.txt": "hello moto",
			},
			targetbranch: "yolo",
			wantErr:      "cannot find foo/file.txt inside the repo repository: No message available",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, _ := rtesting.SetupFakeContext(t)
			client, mux, tearDown, tURL := bbtest.SetupBBDataCenterClient()
			defer tearDown()
			v := &Provider{scmClient: client, baseURL: tURL, defaultBranchLatestCommit: "1234", projectKey: tt.event.Organization}
			bbtest.MuxFiles(t, mux, tt.event, tt.targetbranch, filepath.Dir(tt.path), tt.filescontents, tt.wantErr != "")
			fc, err := v.GetFileInsideRepo(ctx, tt.event, tt.path, tt.targetbranch)
			if tt.wantErr != "" {
				assert.Equal(t, err.Error(), tt.wantErr)
				return
			}
			assert.NilError(t, err)
			assert.Equal(t, tt.assertOutput, fc)
		})
	}
}

func TestSetClient(t *testing.T) {
	tests := []struct {
		name          string
		apiURL        string
		opts          *info.Event
		wantErrSubstr string
		muxUser       func(w http.ResponseWriter, r *http.Request)
	}{
		{
			name:          "bad/no username",
			opts:          info.NewEvent(),
			wantErrSubstr: "no spec.git_provider.user",
		},
		{
			name: "bad/no secret",
			opts: &info.Event{
				Provider: &info.Provider{
					User: "foo",
				},
			},
			wantErrSubstr: "no spec.git_provider.secret",
		},
		{
			name: "bad/no url",
			opts: &info.Event{
				Provider: &info.Provider{
					User:  "foo",
					Token: "bar",
				},
			},
			wantErrSubstr: "no spec.git_provider.url",
		},
		{
			name: "bad/invalid user",
			opts: &info.Event{
				Provider: &info.Provider{
					User:  "foo",
					Token: "bar",
					URL:   "https://foo.bar",
				},
			},
			muxUser: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"errors": [{"message": "Unauthorized"}]}`))
			},
			apiURL:        "https://foo.bar/rest",
			wantErrSubstr: "cannot get user foo with token",
		},
		{
			name: "bad/unknown error",
			opts: &info.Event{
				Provider: &info.Provider{
					User:  "foo",
					Token: "bar",
					URL:   "https://foo.bar",
				},
			},
			muxUser: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"errors": [{"message": "Internal Server Error"}]}`))
			},
			apiURL:        "https://foo.bar/rest",
			wantErrSubstr: "cannot get user foo: Internal Server Error",
		},
		{
			name: "good/url append /rest",
			opts: &info.Event{
				Provider: &info.Provider{
					User:  "foo",
					Token: "bar",
					URL:   "https://foo.bar",
				},
			},
			muxUser: func(w http.ResponseWriter, _ *http.Request) {
				fmt.Fprint(w, `{"name": "foo"}`)
			},
			apiURL: "https://foo.bar/rest",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, _ := rtesting.SetupFakeContext(t)
			client, mux, tearDown, tURL := bbtest.SetupBBDataCenterClient()
			defer tearDown()
			if tt.muxUser != nil {
				mux.HandleFunc("/users/foo", tt.muxUser)
			}
			v := &Provider{scmClient: client, baseURL: tURL}
			err := v.SetClient(ctx, nil, tt.opts, nil, nil)
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
	tests := []struct {
		name          string
		event         *info.Event
		commit        scm.Commit
		defaultBranch string
		latestCommit  string
	}{
		{
			name: "Test valid Commit",
			event: &info.Event{
				Organization: "owner",
				Repository:   "repo",
				SHA:          "sha",
			},
			defaultBranch: "branchmain",
			commit: scm.Commit{
				Message: "hello moto",
			},
			latestCommit: "latestcommit",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, _ := rtesting.SetupFakeContext(t)
			scmClient, mux, tearDown, tURL := bbtest.SetupBBDataCenterClient()
			bbtest.MuxCommitInfo(t, mux, tt.event, tt.commit)
			bbtest.MuxDefaultBranch(t, mux, tt.event, tt.defaultBranch, tt.latestCommit)
			defer tearDown()
			v := &Provider{scmClient: scmClient, baseURL: tURL, projectKey: tt.event.Organization}
			err := v.GetCommitInfo(ctx, tt.event)
			assert.NilError(t, err)
			assert.Equal(t, tt.defaultBranch, tt.event.DefaultBranch)
			assert.Equal(t, tt.latestCommit, v.defaultBranchLatestCommit)
			assert.Equal(t, tt.commit.Message, tt.event.SHATitle)
		})
	}
}

func TestGetConfig(t *testing.T) {
	v := &Provider{}
	config := v.GetConfig()
	assert.Equal(t, config.TaskStatusTMPL, taskStatusTemplate)
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name         string
		wantErr      bool
		secret       string
		payload      string
		hashFunc     func() hash.Hash
		prefixheader string
	}{
		{
			name:         "secret missing",
			secret:       "",
			payload:      `{"hello": "moto"}`,
			hashFunc:     sha256.New,
			prefixheader: "sha256",
			wantErr:      true,
		},
		{
			name:         "good/SHA256Signature",
			secret:       "secrete",
			payload:      `{"hello": "moto"}`,
			hashFunc:     sha256.New,
			prefixheader: "sha256",
		},
		{
			name:         "good/SHA1Signature",
			secret:       "secrete",
			payload:      `{"ola": "amigo"}`,
			hashFunc:     sha1.New,
			prefixheader: "sha1",
		},
		{
			name:         "bad/signature",
			payload:      `{"ciao": "ragazzo"}`,
			hashFunc:     sha256.New,
			prefixheader: "sha1",
			wantErr:      true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := &Provider{}

			hmac := hmac.New(tt.hashFunc, []byte(tt.secret))
			hmac.Write([]byte(tt.payload))
			signature := hex.EncodeToString(hmac.Sum(nil))

			httpHeader := http.Header{}
			httpHeader.Add("X-Hub-Signature", fmt.Sprintf("%s=%s", tt.prefixheader, signature))

			event := info.NewEvent()
			event.Request = &info.Request{
				Header:  httpHeader,
				Payload: []byte(tt.payload),
			}
			event.Provider = &info.Provider{
				WebhookSecret: tt.secret,
			}

			if err := v.Validate(context.TODO(), nil, event); (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRemoveLastSegment(t *testing.T) {
	testCases := []struct {
		name        string
		inputURL    string
		expectedURL string
	}{
		{
			name:        "empty path",
			inputURL:    "http://example.com",
			expectedURL: "http://example.com",
		},
		{
			name:        "root path",
			inputURL:    "http://example.com/",
			expectedURL: "http://example.com/",
		},
		{
			name:        "single segment path",
			inputURL:    "http://example.com/api",
			expectedURL: "http://example.com/",
		},
		{
			name:        "single segment path with trailing slash",
			inputURL:    "http://example.com/api/",
			expectedURL: "http://example.com/api", // Note: Original implementation removes trailing slash segment if it's the last one
		},
		{
			name:        "multiple segments path",
			inputURL:    "http://example.com/api/v1/users",
			expectedURL: "http://example.com/api/v1",
		},
		{
			name:        "multiple segments path with trailing slash",
			inputURL:    "http://example.com/api/v1/users/",
			expectedURL: "http://example.com/api/v1/users", // Note: Original implementation removes trailing slash segment if it's the last one
		},
		{
			name:        "path with query parameters",
			inputURL:    "http://example.com/api/v1/users?param=value",
			expectedURL: "http://example.com/api/v1?param=value",
		},
		{
			name:        "path with fragment",
			inputURL:    "http://example.com/api/v1/users#fragment",
			expectedURL: "http://example.com/api/v1#fragment",
		},
		{
			name:        "path with query parameters and fragment",
			inputURL:    "http://example.com/api/v1/users?param=value#fragment",
			expectedURL: "http://example.com/api/v1?param=value#fragment",
		},
		{
			name:        "https URL",
			inputURL:    "https://example.com/api/v1/users",
			expectedURL: "https://example.com/api/v1",
		},
		{
			name:        "no host, just path",
			inputURL:    "/api/v1/users",
			expectedURL: "/api/v1",
		},
		{
			name:        "just root path",
			inputURL:    "/",
			expectedURL: "/",
		},
		{
			name:        "empty string",
			inputURL:    "",
			expectedURL: "", // Behavior for empty string input might be debatable, but based on the logic, it becomes "/"
		},
		{
			name:        "path with double slashes",
			inputURL:    "http://example.com/api//v1/users", // Double slashes in path
			expectedURL: "http://example.com/api//v1",       // Double slashes are preserved by net/url and strings.Split
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualURL := removeLastSegment(tc.inputURL)
			assert.Equal(t, actualURL, tc.expectedURL)
		})
	}
}

func TestGetFiles(t *testing.T) {
	pushEvent := &info.Event{
		SHA:           "IAMSHA123",
		Organization:  "pac",
		Repository:    "test",
		TriggerTarget: triggertype.Push,
	}
	prEvent := &info.Event{
		Organization:      "pac",
		Repository:        "test",
		TriggerTarget:     triggertype.PullRequest,
		PullRequestNumber: 1,
	}

	pushFiles := []*bbtest.DiffStat{
		{
			Path: bbtest.DiffPath{ToString: "added.md"},
			Type: "ADD",
		},
		{
			Path: bbtest.DiffPath{ToString: "modified.txt"},
			Type: "MODIFY",
		},
		{
			Path: bbtest.DiffPath{ToString: "renamed.yaml"},
			Type: "MOVE",
		},
		{
			Path: bbtest.DiffPath{ToString: "deleted.go"},
			Type: "DELETE",
		},
	}

	pullRequestFiles := []*bbtest.DiffStat{
		{
			Path: bbtest.DiffPath{ToString: "added.go"},
			Type: "ADD",
		},
		{
			Path: bbtest.DiffPath{ToString: "modified.yaml"},
			Type: "MODIFY",
		},
		{
			Path: bbtest.DiffPath{ToString: "renamed.txt"},
			Type: "MOVE",
		},
		{
			Path: bbtest.DiffPath{ToString: "deleted.md"},
			Type: "DELETE",
		},
	}

	tests := []struct {
		name                   string
		event                  *info.Event
		changeFiles            []*bbtest.DiffStat
		wantAddedFilesCount    int
		wantDeletedFilesCount  int
		wantModifiedFilesCount int
		wantRenamedFilesCount  int
		wantError              bool
		errMsg                 string
	}{
		{
			name:                   "good/push event",
			event:                  pushEvent,
			changeFiles:            pushFiles,
			wantAddedFilesCount:    1,
			wantDeletedFilesCount:  1,
			wantModifiedFilesCount: 1,
			wantRenamedFilesCount:  1,
		},
		{
			name:                   "bad/push event",
			event:                  pushEvent,
			wantAddedFilesCount:    0,
			wantDeletedFilesCount:  0,
			wantModifiedFilesCount: 0,
			wantRenamedFilesCount:  0,
			wantError:              true,
			errMsg:                 "failed to list changes for commit IAMSHA123: No message available",
		},
		{
			name:                   "good/pull_request event",
			event:                  prEvent,
			changeFiles:            pullRequestFiles,
			wantAddedFilesCount:    1,
			wantDeletedFilesCount:  1,
			wantModifiedFilesCount: 1,
			wantRenamedFilesCount:  1,
		},
		{
			name:                   "bad/pull_request event",
			event:                  prEvent,
			wantAddedFilesCount:    0,
			wantDeletedFilesCount:  0,
			wantModifiedFilesCount: 0,
			wantRenamedFilesCount:  0,
			wantError:              true,
			errMsg:                 "failed to list changes for pull request: No message available",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, _ := rtesting.SetupFakeContext(t)
			client, mux, tearDown, tURL := bbtest.SetupBBDataCenterClient()
			defer tearDown()

			stats := &bbtest.DiffStats{
				Values: tt.changeFiles,
			}

			if tt.event.TriggerTarget == triggertype.Push {
				mux.HandleFunc("/projects/pac/repos/test/commits/IAMSHA123/changes", func(w http.ResponseWriter, _ *http.Request) {
					if tt.wantError {
						w.WriteHeader(http.StatusUnauthorized)
					} else {
						b, _ := json.Marshal(stats)
						fmt.Fprint(w, string(b))
					}
				})
			}
			if tt.event.TriggerTarget == triggertype.PullRequest {
				mux.HandleFunc("/projects/pac/repos/test/pull-requests/1/changes", func(w http.ResponseWriter, _ *http.Request) {
					if tt.wantError {
						w.WriteHeader(http.StatusUnauthorized)
					} else {
						b, _ := json.Marshal(stats)
						fmt.Fprint(w, string(b))
					}
				})
			}
			v := &Provider{scmClient: client, baseURL: tURL}
			changedFiles, err := v.GetFiles(ctx, tt.event)
			if tt.wantError {
				assert.Equal(t, err.Error(), tt.errMsg)
				return
			}
			assert.NilError(t, err, nil)
			assert.Equal(t, tt.wantAddedFilesCount, len(changedFiles.Added))
			assert.Equal(t, tt.wantDeletedFilesCount, len(changedFiles.Deleted))
			assert.Equal(t, tt.wantModifiedFilesCount, len(changedFiles.Modified))
			assert.Equal(t, tt.wantRenamedFilesCount, len(changedFiles.Renamed))

			if tt.event.TriggerTarget == triggertype.Push {
				for i := range changedFiles.All {
					assert.Equal(t, tt.changeFiles[i].Path.ToString, changedFiles.All[i])
				}
			}

			if tt.event.TriggerTarget == triggertype.PullRequest {
				for i := range changedFiles.All {
					assert.Equal(t, tt.changeFiles[i].Path.ToString, changedFiles.All[i])
				}
			}
		})
	}
}
