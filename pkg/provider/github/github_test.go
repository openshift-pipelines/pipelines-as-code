package github

import (
	"context"
	"crypto/hmac"

	//nolint: gosec
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
	"time"

	"github.com/google/go-github/v49/github"
	"github.com/jonboulle/clockwork"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	ghtesthelper "github.com/openshift-pipelines/pipelines-as-code/pkg/test/github"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/test/logger"
	"gotest.tools/v3/assert"
	"knative.dev/pkg/ptr"
	rtesting "knative.dev/pkg/reconciler/testing"
)

func TestGithubSplitURL(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		wantOrg  string
		wantRepo string
		wantRef  string
		wantPath string
		gheURL   string
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
		{
			name:     "raw GHE URL",
			url:      "https://raw.ghe.domain.com/owner/repo/branch/file?token=TOKEN",
			gheURL:   "https://ghe.domain.com",
			wantOrg:  "owner",
			wantRepo: "repo",
			wantRef:  "branch",
			wantPath: "file",
		},
		{
			name:     "not matching ghe but allowed from public gh",
			url:      fmt.Sprintf("https://%s/owner/repo/branch/file?token=TOKEN", publicRawURLHost),
			gheURL:   "https://foo.com",
			wantOrg:  "owner",
			wantRepo: "repo",
			wantRef:  "branch",
			wantPath: "file",
		},
		{
			name:    "not matching raw",
			url:     "https://bar.com/owner/repo/branch/file?token=TOKEN",
			gheURL:  "https://foo.com",
			wantErr: true,
		},
		{
			name:    "not a full direct url",
			url:     "https://raw.ghe/owner/repo/branch?token=TOKEN",
			gheURL:  "https://raw.ghe",
			wantErr: true,
		},
		{
			name:    "bad formatted ghe url",
			url:     "https://raw.ghe/owner/repo/branch?token=TOKEN",
			gheURL:  "https:raw.ghe",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := info.NewEvent()
			event.GHEURL = tt.gheURL
			org, repo, path, ref, err := splitGithubURL(event, tt.url)
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
			logger, _ := logger.GetLogger()
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

func TestProvider_checkWebhookSecretValidity(t *testing.T) {
	cw := clockwork.NewFakeClock()
	tests := []struct {
		name           string
		wantSubErr     string
		remaining      int
		expTime        time.Time
		expHeaderSet   bool
		apiNotEnabled  bool
		wantLogSnippet string
	}{
		{
			name:         "remaining scim calls",
			remaining:    1,
			expHeaderSet: true,
			expTime:      cw.Now().Add(1 * time.Minute),
		},
		{
			name:         "no remaining scim calls",
			wantSubErr:   "token is ratelimited",
			remaining:    0,
			expHeaderSet: true,
			expTime:      cw.Now().Add(1 * time.Minute),
		},
		{
			name:         "expired",
			wantSubErr:   "token has expired",
			expTime:      cw.Now().Add(-1 * time.Minute),
			expHeaderSet: true,
		},
		{
			name:      "no header mean unlimited",
			remaining: 5,
		},
		{
			name:       "no header but no remaining scim calls",
			remaining:  0,
			wantSubErr: "token is ratelimited",
		},
		{
			name:           "not enabled",
			apiNotEnabled:  true,
			wantLogSnippet: "skipping checking",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, _ := rtesting.SetupFakeContext(t)
			fakeclient, mux, _, teardown := ghtesthelper.SetupGH()
			logger, observer := logger.GetLogger()

			if !tt.apiNotEnabled {
				mux.HandleFunc("/rate_limit", func(rw http.ResponseWriter, r *http.Request) {
					s := &github.RateLimits{
						SCIM: &github.Rate{
							Remaining: tt.remaining,
						},
					}
					st := new(struct {
						Resources *github.RateLimits `json:"resources"`
					})
					st.Resources = s
					b, _ := json.Marshal(st)
					rw.Header().Set("Content-Type", "application/json")
					if tt.expHeaderSet {
						rw.Header().Set("GitHub-Authentication-Token-Expiration", tt.expTime.Format("2006-01-02 03:04:05 MST"))
					}
					fmt.Fprint(rw, string(b))
				})
			}
			defer teardown()
			v := &Provider{
				Client: fakeclient,
				Logger: logger,
			}
			err := v.checkWebhookSecretValidity(ctx, cw)
			if tt.wantSubErr != "" {
				assert.ErrorContains(t, err, tt.wantSubErr)
			} else {
				assert.NilError(t, err)
			}

			if tt.wantLogSnippet != "" {
				assert.Assert(t, observer.FilterMessageSnippet(tt.wantLogSnippet).Len() > 0)
			}
		})
	}
}

func TestParseTS(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
		expTS   string
	}{
		{
			name:  "valid as defined by go-github",
			expTS: "2023-01-31 03:30:00 UTC",
		},
		{
			name:  "valid with UTC and eu time",
			expTS: "2023-01-31 15:30:00 UTC",
		},
		{
			name:  "valid with timezone inside",
			expTS: "2023-04-26 23:23:26 +2000",
		},

		{
			name:    "invalid",
			expTS:   "Mon 2023-04-26 23:23:26 +2000",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseTS(tt.expTS)
			if tt.wantErr {
				assert.Assert(t, err != nil)
			} else {
				assert.NilError(t, err)
				assert.Assert(t, got.Year() != 0o1)
			}
		})
	}
}
