package github

import (
	"context"
	"crypto/hmac"

	// nolint: gosec
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/google/go-github/v43/github"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	ghtesthelper "github.com/openshift-pipelines/pipelines-as-code/pkg/test/github"
	rtesting "knative.dev/pkg/reconciler/testing"

	"github.com/google/go-cmp/cmp"
	"go.uber.org/zap"
	zapobserver "go.uber.org/zap/zaptest/observer"
	"gotest.tools/v3/assert"
)

func getLogger() *zap.SugaredLogger {
	observer, _ := zapobserver.New(zap.InfoLevel)
	logger := zap.New(observer).Sugar()
	return logger
}

func setupFakesURLS() (client Provider, teardown func()) {
	fakeclient, mux, _, teardown := ghtesthelper.SetupGH()

	mux.HandleFunc("/repos/check/info/check-runs", func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `{"id": 555}`)
	})

	mux.HandleFunc("/repos/check/info/check-runs/555", func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `{"id": 555}`)
	})

	mux.HandleFunc("/repos/check/info/check-runs/2026", func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `{"id": 666}`)
	})

	mux.HandleFunc("/repos/foo/bar/contents/README.md", func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `{
"name": "README.md",
"sha": "readmemdsha",
"type": "file"
}`)
	})

	mux.HandleFunc("/repos/foo/bar/git/blobs/readmemdsha", func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `{
"name": "README.md",
"content": "aGVsbG8gbW90bwo=",
"encoding": "base64"
}`)
	})

	mux.HandleFunc("/repos/tekton/dir/git/blobs/pipelineyaml", func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `{
"name": "README.md",
"content": "aGVsbG8gcGlwZWxpbmV5YW1s",
"encoding": "base64"
}`)
	})

	mux.HandleFunc("/repos/tekton/dir/git/blobs/runyaml", func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `{
"name": "README.md",
"content": "aGVsbG8gcnVueWFtbA==",
"encoding": "base64"
}`)
	})

	mux.HandleFunc("/repos/tekton/dir/git/blobs/tektonyaml", func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `{
"name": "README.md",
"content": "aGVsbG8gdGVrdG9ueWFtbA==",
"encoding": "base64"
}`)
	})

	mux.HandleFunc("/repos/tekton/dir/contents/.tekton", func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `[{

				  "name": "pipeline.yaml",
				  "path": ".tekton/pipeline.yaml",
				  "sha": "pipelineyaml",
				  "type": "file"
				},
				{
				  "name": "run.yaml",
				  "path": ".tekton/run.yaml",
				  "sha": "runyaml",
				  "type": "file"
				}]`)
	})
	mux.HandleFunc("/repos/throw/error/contents/.tekton", func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, "ERRROR")
	})

	mux.HandleFunc("/repos/its/afile/contents/.tekton", func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `{
"name": ".tekton",
"sha": "decfae2653959f7c6c25f21f026c3819bea41ecf",
"type": "file",
"content": "WyFbQ29udGFpbmVyIFJlcG9zaXRvcnkgb24gUXVheV0oaHR0cHM6Ly9xdWF5\nLmlvL3JlcG9zaXRvcnkvY2htb3VlbC90ZWt0b24tYXNhLWNvZGUvc3RhdHVz"}`)
	})
	mux.HandleFunc("/repos/pas/la/contents/.tekton", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	gcvs := Provider{
		Client: fakeclient,
	}
	return gcvs, teardown
}

func TestGetFileInsideRepo(t *testing.T) {
	ctx, _ := rtesting.SetupFakeContext(t)
	gcvs, teardown := setupFakesURLS()
	defer teardown()
	type args struct {
		path      string
		runevent  info.Event
		assertion func(t *testing.T, got string, err error)
	}

	testGetTektonDir := []struct {
		name string
		args args
	}{
		{
			name: "testgood",
			args: args{
				assertion: func(t *testing.T, got string, err error) {
					assert.NilError(t, err)
				},
				path: "README.md",
				runevent: info.Event{
					Organization: "foo",
					Repository:   "bar",
				},
			},
		},
		{
			name: "notfound",
			args: args{
				assertion: func(t *testing.T, got string, err error) {
					assert.ErrorContains(t, err, "404")
				},
				path: ".tekton",
				runevent: info.Event{
					Organization: "pas",
					Repository:   "la",
				},
			},
		},
		{
			name: "file_should_be_a_dir",
			args: args{
				assertion: func(t *testing.T, got string, err error) {
					assert.ErrorContains(t, err, "is a directory")
				},
				path: ".tekton",
				runevent: info.Event{
					Organization: "tekton",
					Repository:   "dir",
				},
			},
		},
		{
			name: "throwerror",
			args: args{
				assertion: func(t *testing.T, got string, err error) {
					assert.ErrorContains(t, err, "invalid character")
				},
				path: ".tekton",
				runevent: info.Event{
					Organization: "throw",
					Repository:   "error",
				},
			},
		},
	}
	for _, tt := range testGetTektonDir {
		t.Run(tt.name, func(t *testing.T) {
			got, err := gcvs.GetFileInsideRepo(ctx, &tt.args.runevent, tt.args.path, "")
			tt.args.assertion(t, got, err)
		})
	}
}

func TestGetTektonDir(t *testing.T) {
	ctx, _ := rtesting.SetupFakeContext(t)
	gcvs, teardown := setupFakesURLS()
	defer teardown()

	type args struct {
		path      string
		runevent  info.Event
		assertion func(t *testing.T, got string, err error)
	}

	testGetTektonDir := []struct {
		name string
		args args
	}{
		{
			name: "testgood",
			args: args{
				assertion: func(t *testing.T, got string, err error) {
					assert.NilError(t, err)
					assert.Assert(t, got != "")
				},
				path: ".tekton",
				runevent: info.Event{
					Organization: "tekton",
					Repository:   "dir",
				},
			},
		},
		{
			name: "notfound",
			args: args{
				assertion: func(t *testing.T, got string, err error) {
					assert.NilError(t, err)
					assert.Assert(t, got == "")
				},
				path: ".tekton",
				runevent: info.Event{
					Organization: "pas",
					Repository:   "la",
				},
			},
		},
		{
			name: "tektondirisafile",
			args: args{
				assertion: func(t *testing.T, got string, err error) {
					assert.Error(t, err, "the object .tekton is a file instead of a directory")
					assert.Assert(t, got == "")
				},
				path: ".tekton",
				runevent: info.Event{
					Organization: "its",
					Repository:   "afile",
				},
			},
		},
		{
			name: "throwerror",
			args: args{
				assertion: func(t *testing.T, got string, err error) {
					assert.ErrorContains(t, err, "invalid character")
					assert.Assert(t, got == "")
				},
				path: ".tekton",
				runevent: info.Event{
					Organization: "throw",
					Repository:   "error",
				},
			},
		},
	}
	for _, tt := range testGetTektonDir {
		t.Run(tt.name, func(t *testing.T) {
			got, err := gcvs.GetTektonDir(ctx, &tt.args.runevent, tt.args.path)
			tt.args.assertion(t, got, err)
		})
	}
}

func TestGetTektonDirTemplate(t *testing.T) {
	const expected = `
hello pipelineyaml
---
hello runyaml
`
	ctx, _ := rtesting.SetupFakeContext(t)
	gcvs, teardown := setupFakesURLS()
	defer teardown()
	runevent := &info.Event{
		Organization: "tekton",
		Repository:   "dir",
	}

	got, err := gcvs.GetTektonDir(ctx, runevent, ".tekton")
	assert.NilError(t, err)
	if d := cmp.Diff(got, expected); d != "" {
		t.Fatalf("-got, +want: %v", d)
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
			err := v.SetClient(ctx, tt.event)
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
			eventType:     "commit_comment",
			wantErrString: "github: event commit_comment is not supported",
			isGH:          true,
			processReq:    false,
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
				Comment: &github.IssueComment{Body: github.String("/ok-to-test \n, don't let me in :)")},
			},
			eventType:  "issue_comment",
			isGH:       true,
			processReq: false,
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
			processReq: false,
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

			header := &http.Header{}
			header.Set("X-GitHub-Event", tt.eventType)

			isGh, processReq, _, err := gprovider.Detect(header, string(jeez), logger)
			if tt.wantErrString != "" {
				assert.ErrorContains(t, err, tt.wantErrString)
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
			v := &Provider{}

			hmac := hmac.New(tt.hashFunc, []byte(tt.secret))
			hmac.Write([]byte(tt.payload))
			signature := hex.EncodeToString(hmac.Sum(nil))

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
