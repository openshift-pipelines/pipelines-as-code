package github

import (
	"context"
	"crypto/hmac"
	"strings"

	//nolint: gosec
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/google/go-github/v47/github"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	ghtesthelper "github.com/openshift-pipelines/pipelines-as-code/pkg/test/github"
	rtesting "knative.dev/pkg/reconciler/testing"

	"go.uber.org/zap"
	zapobserver "go.uber.org/zap/zaptest/observer"
	"gotest.tools/v3/assert"
	"knative.dev/pkg/ptr"
)

func getLogger() *zap.SugaredLogger {
	observer, _ := zapobserver.New(zap.InfoLevel)
	logger := zap.New(observer).Sugar()
	return logger
}

func TestGithubSplitURL(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		wantOrg  string
		wantRepo string
		wantRef  string
		wantPath string
		wantErr  bool
	}{
		{
			name:     "Split URL",
			url:      "https://github.com/openshift-pipelines/pipelines-as-code/blob/main/testdatas/remote_task.yaml",
			wantOrg:  "openshift-pipelines",
			wantRepo: "pipelines-as-code",
			wantRef:  "main",
			wantPath: "testdatas/remote_task.yaml",
		},
		{
			name:     "Split raw URL",
			url:      "https://raw.githubusercontent.com/openshift-pipelines/pipelines-as-code/main/testdatas/remote_task.yaml",
			wantOrg:  "openshift-pipelines",
			wantRepo: "pipelines-as-code",
			wantRef:  "main",
			wantPath: "testdatas/remote_task.yaml",
		},
		{
			name:     "Split raw URL2",
			url:      "https://raw.githubusercontent.com/openshift-pipelines/pipelines-as-code/main/remote_task.yaml",
			wantOrg:  "openshift-pipelines",
			wantRepo: "pipelines-as-code",
			wantRef:  "main",
			wantPath: "remote_task.yaml",
		},
		{
			name:    "Too small URL",
			url:     "https://raw.githubusercontent.com/openshift-pipelines/pipelines-as-code",
			wantErr: true,
		},
		{
			name:    "Invalid no path URL",
			url:     "https://raw.githubusercontent.com/openshift-pipelines/pipelines-as-code/main",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			org, repo, path, ref, err := splitGithubURL(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("SplitURL() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			assert.Equal(t, tt.wantOrg, org)
			assert.Equal(t, tt.wantRepo, repo)
			assert.Equal(t, tt.wantRef, ref)
			assert.Equal(t, tt.wantPath, path)
		})
	}
}

func TestGetTektonDir(t *testing.T) {
	testGetTektonDir := []struct {
		treepath       string
		event          *info.Event
		name           string
		expectedString string
	}{
		{
			name: "test no subtree",
			event: &info.Event{
				Organization: "tekton",
				Repository:   "cat",
				SHA:          "123",
			},
			expectedString: "PipelineRun",
			treepath:       "testdata/tree/simple",
		},
		{
			name: "test with subtree",
			event: &info.Event{
				Organization: "tekton",
				Repository:   "cat",
				SHA:          "123",
			},
			expectedString: "FROMSUBTREE",
			treepath:       "testdata/tree/subdir",
		},
	}
	for _, tt := range testGetTektonDir {
		t.Run(tt.name, func(t *testing.T) {
			ctx, _ := rtesting.SetupFakeContext(t)
			fakeclient, mux, _, teardown := ghtesthelper.SetupGH()
			defer teardown()
			gvcs := Provider{
				Client: fakeclient,
			}
			shaDir := fmt.Sprintf("%x", sha256.Sum256([]byte(tt.treepath)))
			tt.event.SHA = shaDir
			ghtesthelper.SetupGitTree(t, mux, tt.treepath, tt.event, false)

			got, err := gvcs.GetTektonDir(ctx, tt.event, ".tekton")
			assert.NilError(t, err)
			assert.Assert(t, strings.Contains(got, tt.expectedString), "expected %s, got %s", tt.expectedString, got)
		})
	}
}

func TestGetFileInsideRepo(t *testing.T) {
	testGetTektonDir := []struct {
		name       string
		rets       map[string]func(w http.ResponseWriter, r *http.Request)
		filepath   string
		wantErrStr string
	}{
		{
			name:     "fail/trying to get a subdir",
			filepath: "retdir",
			rets: map[string]func(w http.ResponseWriter, r *http.Request){
				"/repos/tekton/thecat/contents/retdir": func(w http.ResponseWriter, _ *http.Request) {
					fmt.Fprint(w, `[{"name": "directory", "path": "a/directory"}]`)
				},
			},

			wantErrStr: "referenced file inside the Github Repository retdir is a directory",
		},
		{
			name:     "fail/bad json",
			filepath: "retfile",
			rets: map[string]func(w http.ResponseWriter, r *http.Request){
				"/repos/tekton/thecat/contents/retfile": func(w http.ResponseWriter, _ *http.Request) {
					fmt.Fprint(w, `nonono`)
				},
			},

			wantErrStr: "invalid character",
		},
		{
			name:     "fail/bad encoding",
			filepath: "retfile",
			rets: map[string]func(w http.ResponseWriter, r *http.Request){
				"/repos/tekton/thecat/contents/retfile": func(w http.ResponseWriter, _ *http.Request) {
					fmt.Fprint(w, `{"name": "file", "path": "a/file", "sha": "shafile"}`)
				},
				"/repos/tekton/thecat/git/blobs/shafile": func(w http.ResponseWriter, _ *http.Request) {
					fmt.Fprint(w, `{"content": "xxxxxx", "sha": "shafile"}`)
				},
			},

			wantErrStr: "illegal base64 data",
		},
		{
			name:     "error/cannot get blob",
			filepath: "retfile",
			rets: map[string]func(w http.ResponseWriter, r *http.Request){
				"/repos/tekton/thecat/contents/retfile": func(w http.ResponseWriter, _ *http.Request) {
					fmt.Fprint(w, `{"name": "file", "path": "a/file", "sha": "shafile"}`)
				},
			},

			wantErrStr: "404",
		},
	}
	for _, tt := range testGetTektonDir {
		t.Run(tt.name, func(t *testing.T) {
			ctx, _ := rtesting.SetupFakeContext(t)
			fakeclient, mux, _, teardown := ghtesthelper.SetupGH()
			defer teardown()
			gvcs := Provider{
				Client: fakeclient,
			}
			for s, f := range tt.rets {
				mux.HandleFunc(s, f)
			}
			event := &info.Event{
				Organization: "tekton",
				Repository:   "thecat",
				SHA:          "123",
			}
			got, err := gvcs.GetFileInsideRepo(ctx, event, tt.filepath, "")
			if tt.wantErrStr != "" {
				assert.Assert(t, err != nil, "we should have get an error here")
				assert.Assert(t, strings.Contains(err.Error(), tt.wantErrStr), err.Error(), tt.wantErrStr)
				return
			}
			assert.NilError(t, err)
			assert.Assert(t, got != "")
		})
	}
}

func TestCheckSenderOrgMembership(t *testing.T) {
	tests := []struct {
		name      string
		apiReturn string
		allowed   bool
		wantErr   bool
		runevent  info.Event
	}{
		{
			name: "Check Sender Org Membership",
			runevent: info.Event{
				Organization: "organization",
				Sender:       "me",
			},
			apiReturn: `[{"login": "me"}]`,
			allowed:   true,
			wantErr:   false,
		},
		{
			name: "Check Sender not in Org Membership",
			runevent: info.Event{
				Organization: "organization",
				Sender:       "me",
			},
			apiReturn: `[{"login": "not"}]`,
			allowed:   false,
		},
		{
			name: "Not found on organization",
			runevent: info.Event{
				Organization: "notfound",
				Sender:       "me",
			},
			allowed: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeclient, mux, _, teardown := ghtesthelper.SetupGH()
			defer teardown()
			ctx, _ := rtesting.SetupFakeContext(t)
			gprovider := Provider{
				Client: fakeclient,
			}
			mux.HandleFunc(fmt.Sprintf("/orgs/%s/public_members", tt.runevent.Organization), func(rw http.ResponseWriter, r *http.Request) {
				fmt.Fprint(rw, tt.apiReturn)
			})

			allowed, err := gprovider.checkSenderOrgMembership(ctx, &tt.runevent)
			if tt.wantErr && err == nil {
				t.Error("We didn't get an error when we wanted one")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("We got an error when we didn't want it: %s", err)
			}
			assert.Equal(t, tt.allowed, allowed)
		})
	}
}

func TestGetStringPullRequestComment(t *testing.T) {
	regexp := `(^|\n)/retest(\r\n|$)`
	tests := []struct {
		name, apiReturn string
		wantErr         bool
		runevent        info.Event
		wantRet         bool
	}{
		{
			name:      "Get String from comments",
			runevent:  info.Event{URL: "http://1"},
			apiReturn: `[{"body": "/retest"}]`,
			wantRet:   true,
		},
		{
			name:      "Not matching string in comments",
			runevent:  info.Event{URL: "http://1"},
			apiReturn: `[{"body": ""}]`,
			wantRet:   false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeclient, mux, _, teardown := ghtesthelper.SetupGH()
			defer teardown()
			ctx, _ := rtesting.SetupFakeContext(t)
			gprovider := Provider{
				Client: fakeclient,
			}
			mux.HandleFunc(fmt.Sprintf("/repos/issues/%s/comments", filepath.Base(tt.runevent.URL)), func(rw http.ResponseWriter, r *http.Request) {
				fmt.Fprint(rw, tt.apiReturn)
			})

			ret, err := gprovider.GetStringPullRequestComment(ctx, &tt.runevent, regexp)
			if tt.wantErr && err == nil {
				t.Error("We didn't get an error when we wanted one")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("We got an error when we didn't want it: %s", err)
			}

			if tt.wantRet {
				assert.Assert(t, ret != nil)
			}
		})
	}
}

func TestGithubGetCommitInfo(t *testing.T) {
	tests := []struct {
		name              string
		event             *info.Event
		noclient          bool
		apiReply, wantErr string
		shaurl, shatitle  string
	}{
		{
			name: "good",
			event: &info.Event{
				Organization: "owner",
				Repository:   "repository",
				SHA:          "shacommitinfo",
			},
			shaurl:   "https://git.provider/commit/info",
			shatitle: "My beautiful pony",
		},
		{
			name: "error",
			event: &info.Event{
				Organization: "owner",
				Repository:   "repository",
				SHA:          "shacommitinfo",
			},
			apiReply: "hello moto",
		},
		{
			name:     "noclient",
			event:    &info.Event{},
			wantErr:  "no github client has been initiliazed",
			noclient: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeclient, mux, _, teardown := ghtesthelper.SetupGH()
			defer teardown()
			mux.HandleFunc(fmt.Sprintf("/repos/%s/%s/git/commits/%s",
				tt.event.Organization, tt.event.Repository, tt.event.SHA), func(rw http.ResponseWriter, r *http.Request) {
				if tt.apiReply != "" {
					fmt.Fprintf(rw, tt.apiReply)
					return
				}
				fmt.Fprintf(rw, `{"html_url": "%s", "message": "%s"}`, tt.shaurl, tt.shatitle)
			})
			ctx, _ := rtesting.SetupFakeContext(t)
			provider := &Provider{Client: fakeclient}
			if tt.noclient {
				provider = &Provider{}
			}
			err := provider.GetCommitInfo(ctx, tt.event)
			if tt.wantErr != "" {
				assert.ErrorContains(t, err, tt.wantErr)
				return
			}
			assert.Equal(t, tt.shatitle, tt.event.SHATitle)
			assert.Equal(t, tt.shaurl, tt.event.SHAURL)
		})
	}
}

func TestGithubSetClient(t *testing.T) {
	tests := []struct {
		name        string
		event       *info.Event
		expectedURL string
		isGHE       bool
	}{
		{
			name: "api url set",
			event: &info.Event{
				Provider: &info.Provider{
					URL: "foo.com",
				},
			},
			expectedURL: "https://foo.com",
			isGHE:       true,
		},
		{
			name:        "default to public github",
			expectedURL: "https://api.github.com/",
			event:       info.NewEvent(),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, _ := rtesting.SetupFakeContext(t)
			v := Provider{}
			err := v.SetClient(ctx, nil, tt.event)
			assert.NilError(t, err)
			assert.Equal(t, tt.expectedURL, *v.APIURL)
			assert.Equal(t, "https", v.Client.BaseURL.Scheme)
			if tt.isGHE {
				assert.Equal(t, "/api/v3/", v.Client.BaseURL.Path)
			} else {
				assert.Equal(t, "/", v.Client.BaseURL.Path)
			}
		})
	}
}

func TestProvider_Detect(t *testing.T) {
	tests := []struct {
		name          string
		wantErrString string
		isGH          bool
		processReq    bool
		event         interface{}
		eventType     string
		wantReason    string
	}{
		{
			name:       "not a github Event",
			eventType:  "",
			isGH:       false,
			processReq: false,
		},
		{
			name:          "invalid github Event",
			eventType:     "validator",
			wantErrString: "unknown X-Github-Event in message: validator",
			isGH:          false,
			processReq:    false,
		},
		{
			name: "valid check run Event",
			event: github.CheckRunEvent{
				Action: github.String("rerequested"),
				CheckRun: &github.CheckRun{
					ID: github.Int64(123),
				},
			},
			eventType:  "check_run",
			isGH:       true,
			processReq: true,
		},
		{
			name: "unsupported Event",
			event: github.CommitCommentEvent{
				Action: github.String("something"),
			},
			eventType:  "commit_comment",
			wantReason: "event \"commit_comment\" is not supported",
			isGH:       true,
			processReq: false,
		},
		{
			name: "invalid check run Event",
			event: github.CheckRunEvent{
				Action: github.String("not rerequested"),
			},
			eventType:  "check_run",
			isGH:       true,
			processReq: false,
		},
		{
			name: "invalid issue comment Event",
			event: github.IssueCommentEvent{
				Action: github.String("deleted"),
			},
			eventType:  "issue_comment",
			isGH:       true,
			processReq: false,
		},
		{
			name: "issue comment Event with no valid comment",
			event: github.IssueCommentEvent{
				Action: github.String("created"),
				Issue: &github.Issue{
					PullRequestLinks: &github.PullRequestLinks{
						URL: github.String("url"),
					},
					State: github.String("open"),
				},
				Installation: &github.Installation{
					ID: github.Int64(123),
				},
				Comment: &github.IssueComment{Body: github.String("abc")},
			},
			eventType:  "issue_comment",
			isGH:       true,
			processReq: false,
		},
		{
			name: "issue comment Event with ok-to-test comment",
			event: github.IssueCommentEvent{
				Action: github.String("created"),
				Issue: &github.Issue{
					PullRequestLinks: &github.PullRequestLinks{
						URL: github.String("url"),
					},
					State: github.String("open"),
				},
				Installation: &github.Installation{
					ID: github.Int64(123),
				},
				Comment: &github.IssueComment{Body: github.String("/ok-to-test")},
			},
			eventType:  "issue_comment",
			isGH:       true,
			processReq: true,
		},
		{
			name: "issue comment Event with ok-to-test and some string",
			event: github.IssueCommentEvent{
				Action: github.String("created"),
				Issue: &github.Issue{
					PullRequestLinks: &github.PullRequestLinks{
						URL: github.String("url"),
					},
					State: github.String("open"),
				},
				Installation: &github.Installation{
					ID: github.Int64(123),
				},
				Comment: &github.IssueComment{Body: github.String("/ok-to-test \n let me in :)")},
			},
			eventType:  "issue_comment",
			isGH:       true,
			processReq: true,
		},
		{
			name: "issue comment Event with retest",
			event: github.IssueCommentEvent{
				Action: github.String("created"),
				Issue: &github.Issue{
					PullRequestLinks: &github.PullRequestLinks{
						URL: github.String("url"),
					},
					State: github.String("open"),
				},
				Installation: &github.Installation{
					ID: github.Int64(123),
				},
				Comment: &github.IssueComment{Body: github.String("/retest")},
			},
			eventType:  "issue_comment",
			isGH:       true,
			processReq: true,
		},
		{
			name: "issue comment Event with retest with some string",
			event: github.IssueCommentEvent{
				Action: github.String("created"),
				Issue: &github.Issue{
					PullRequestLinks: &github.PullRequestLinks{
						URL: github.String("url"),
					},
					State: github.String("open"),
				},
				Installation: &github.Installation{
					ID: github.Int64(123),
				},
				Comment: &github.IssueComment{Body: github.String("/retest \n will you retest?")},
			},
			eventType:  "issue_comment",
			isGH:       true,
			processReq: true,
		},
		{
			name: "push event",
			event: github.PushEvent{
				Pusher: &github.User{ID: github.Int64(11)},
			},
			eventType:  "push",
			isGH:       true,
			processReq: true,
		},
		{
			name: "pull request event",
			event: github.PullRequestEvent{
				Action: github.String("opened"),
			},
			eventType:  "pull_request",
			isGH:       true,
			processReq: true,
		},
		{
			name: "pull request event not supported action",
			event: github.PullRequestEvent{
				Action: github.String("deleted"),
			},
			eventType:  "pull_request",
			isGH:       true,
			processReq: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gprovider := Provider{}
			logger := getLogger()

			jeez, err := json.Marshal(tt.event)
			if err != nil {
				assert.NilError(t, err)
			}

			header := http.Header{}
			header.Set("X-GitHub-Event", tt.eventType)

			req := &http.Request{Header: header}
			isGh, processReq, _, reason, err := gprovider.Detect(req, string(jeez), logger)
			if tt.wantErrString != "" {
				assert.ErrorContains(t, err, tt.wantErrString)
				return
			}
			if tt.wantReason != "" {
				assert.Assert(t, strings.Contains(reason, tt.wantReason), reason, tt.wantReason)
				return
			}
			assert.NilError(t, err)
			assert.Equal(t, tt.isGH, isGh)
			assert.Equal(t, tt.processReq, processReq)
		})
	}
}

func TestValidate(t *testing.T) {
	header := http.Header{}
	header.Set(github.SHA256SignatureHeader, "hello")
	tests := []struct {
		name         string
		wantErr      bool
		secret       string
		payload      string
		hashFunc     func() hash.Hash
		header       string
		prefixheader string
	}{
		{
			name:         "good/SHA256Signature",
			secret:       "secrete",
			payload:      `{"hello": "moto"}`,
			hashFunc:     sha256.New,
			header:       github.SHA256SignatureHeader,
			prefixheader: "sha256",
		},
		{
			name:         "good/SHA1Signature",
			secret:       "secrete",
			payload:      `{"ola": "amigo"}`,
			hashFunc:     sha1.New,
			header:       github.SHA1SignatureHeader,
			prefixheader: "sha1",
		},
		{
			name:         "bad/signature",
			payload:      `{"ciao": "ragazzo"}`,
			hashFunc:     sha256.New,
			header:       github.SHA1SignatureHeader,
			prefixheader: "sha1",
			wantErr:      true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := getLogger()
			v := &Provider{Logger: logger}

			hm := hmac.New(tt.hashFunc, []byte(tt.secret))
			hm.Write([]byte(tt.payload))
			signature := hex.EncodeToString(hm.Sum(nil))

			httpHeader := http.Header{}
			httpHeader.Add(tt.header, fmt.Sprintf("%s=%s", tt.prefixheader, signature))

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

func TestGetFiles(t *testing.T) {
	tests := []struct {
		name        string
		event       *info.Event
		commitFiles []*github.CommitFile
		commit      *github.RepositoryCommit
	}{
		{
			name: "pull-request",
			event: &info.Event{
				TriggerTarget:     "pull_request",
				Organization:      "pullrequestowner",
				Repository:        "pullrequestrepository",
				PullRequestNumber: 10,
			},
			commitFiles: []*github.CommitFile{
				{
					Filename: ptr.String("first.yaml"),
				}, {
					Filename: ptr.String("second.doc"),
				},
			},
		},
		{
			name: "push",
			event: &info.Event{
				TriggerTarget: "push",
				Organization:  "pushrequestowner",
				Repository:    "pushrequestrepository",
				SHA:           "shacommitinfo",
			},
			commit: &github.RepositoryCommit{
				Files: []*github.CommitFile{
					{
						Filename: ptr.String("first.yaml"),
					}, {
						Filename: ptr.String("second.doc"),
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeclient, mux, _, teardown := ghtesthelper.SetupGH()
			defer teardown()
			commitFiles := []*github.CommitFile{
				{
					Filename: ptr.String("first.yaml"),
				}, {
					Filename: ptr.String("second.doc"),
				},
			}
			if tt.event.TriggerTarget == "pull_request" {
				mux.HandleFunc(fmt.Sprintf("/repos/%s/%s/pulls/%d/files",
					tt.event.Organization, tt.event.Repository, tt.event.PullRequestNumber), func(rw http.ResponseWriter, r *http.Request) {
					b, _ := json.Marshal(commitFiles)
					fmt.Fprint(rw, string(b))
				})
			}
			if tt.event.TriggerTarget == "push" {
				mux.HandleFunc(fmt.Sprintf("/repos/%s/%s/commits/%s",
					tt.event.Organization, tt.event.Repository, tt.event.SHA), func(rw http.ResponseWriter, r *http.Request) {
					c := &github.RepositoryCommit{
						Files: commitFiles,
					}
					b, _ := json.Marshal(c)
					fmt.Fprint(rw, string(b))
				})
			}

			ctx, _ := rtesting.SetupFakeContext(t)
			provider := &Provider{Client: fakeclient}
			fileData, err := provider.GetFiles(ctx, tt.event)
			assert.NilError(t, err, nil)
			if tt.event.TriggerTarget == "pull_request" {
				for i := range fileData {
					assert.Equal(t, *tt.commitFiles[i].Filename, fileData[i])
				}
			}
			if tt.event.TriggerTarget == "push" {
				for i := range fileData {
					assert.Equal(t, *tt.commit.Files[i].Filename, fileData[i])
				}
			}
		})
	}
}
