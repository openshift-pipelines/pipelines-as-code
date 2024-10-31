package bitbucketserver

import (
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	bbv1 "github.com/gfleury/go-bitbucket-v1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/settings"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider"
	bbtest "github.com/openshift-pipelines/pipelines-as-code/pkg/provider/bitbucketserver/test"
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
		wantErr         bool
		removeSuffix    bool
	}{
		{
			name:            "Get Tekton Directory",
			event:           bbtest.MakeEvent(nil),
			path:            ".tekton",
			testDirPath:     "../../pipelineascode/testdata/pull_request/.tekton",
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
			observer, _ := zapobserver.New(zap.InfoLevel)
			logger := zap.New(observer).Sugar()
			ctx, _ := rtesting.SetupFakeContext(t)
			client, mux, tearDown, tURL := bbtest.SetupBBServerClient(ctx)
			defer tearDown()
			v := &Provider{Logger: logger, baseURL: tURL, Client: client, projectKey: tt.event.Organization}
			bbtest.MuxDirContent(t, mux, tt.event, tt.testDirPath, tt.path)
			content, err := v.GetTektonDir(ctx, tt.event, tt.path, "")
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
		Settings: settings.Settings{
			ApplicationName: "HELLO APP",
		},
	}
	pullRequestNumber := 10

	tests := []struct {
		name                  string
		status                provider.StatusOpts
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
			status: provider.StatusOpts{
				Conclusion: "skipped",
			},
			expectedDescSubstr: "Skipping",
			pacOpts:            pacopts,
		},
		{
			name: "good/neutral",
			status: provider.StatusOpts{
				Conclusion: "neutral",
			},
			expectedDescSubstr: "stopped",
			pacOpts:            pacopts,
		},
		{
			name: "good/completed with comment",
			status: provider.StatusOpts{
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
			status: provider.StatusOpts{
				Conclusion: "failure",
			},
			expectedDescSubstr: "Failed",
			pacOpts:            pacopts,
		},
		{
			name: "good/details url",
			status: provider.StatusOpts{
				Conclusion: "failure",
				DetailsURL: "http://fail.com",
			},
			expectedDescSubstr: "Failed",
			pacOpts:            pacopts,
		},
		{
			name: "good/pending",
			status: provider.StatusOpts{
				Conclusion: "pending",
			},
			expectedDescSubstr: "started",
			pacOpts:            pacopts,
		},
		{
			name: "good/success",
			status: provider.StatusOpts{
				Conclusion: "success",
			},
			expectedDescSubstr: "validated",
			pacOpts:            pacopts,
		},
		{
			name: "good/completed",
			status: provider.StatusOpts{
				Conclusion: "completed",
			},
			expectedDescSubstr: "Completed",
			pacOpts:            pacopts,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, _ := rtesting.SetupFakeContext(t)
			client, mux, tearDown, tURL := bbtest.SetupBBServerClient(ctx)
			defer tearDown()
			if tt.nilClient {
				client = nil
			}
			event := bbtest.MakeEvent(nil)
			event.EventType = "pull_request"
			event.Provider.Token = "token"
			v := &Provider{
				baseURL:           tURL,
				Client:            client,
				pullRequestNumber: pullRequestNumber,
				projectKey:        event.Organization,
				run:               &params.Run{},
				pacInfo:           &tt.pacOpts,
			}
			bbtest.MuxCreateAndTestCommitStatus(t, mux, event, tt.expectedDescSubstr, tt.status)
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
			client, mux, tearDown, tURL := bbtest.SetupBBServerClient(ctx)
			defer tearDown()
			v := &Provider{Client: client, baseURL: tURL, defaultBranchLatestCommit: "1234", projectKey: tt.event.Organization}
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
			wantErrSubstr: "cannot get user foo: Status: 500",
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
			bbclient, mux, tearDown, tURL := bbtest.SetupBBServerClient(ctx)
			defer tearDown()
			if tt.muxUser != nil {
				mux.HandleFunc("/users/foo", tt.muxUser)
			}
			v := &Provider{Client: bbclient, baseURL: tURL}
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
		commit        bbv1.Commit
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
			commit: bbv1.Commit{
				Message: "hello moto",
			},
			latestCommit: "latestcommit",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, _ := rtesting.SetupFakeContext(t)
			bbclient, mux, tearDown, tURL := bbtest.SetupBBServerClient(ctx)
			bbtest.MuxCommitInfo(t, mux, tt.event, tt.commit)
			bbtest.MuxDefaultBranch(t, mux, tt.event, tt.defaultBranch, tt.latestCommit)
			defer tearDown()
			v := &Provider{Client: bbclient, baseURL: tURL, projectKey: tt.event.Organization}
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
