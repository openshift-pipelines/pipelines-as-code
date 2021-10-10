package github

import (
	"fmt"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	ghtesthelper "github.com/openshift-pipelines/pipelines-as-code/pkg/test/github"
	rtesting "knative.dev/pkg/reconciler/testing"

	"github.com/google/go-cmp/cmp"
	"go.uber.org/zap"
	zapobserver "go.uber.org/zap/zaptest/observer"
	"gotest.tools/v3/assert"
)

func getLogger() (*zap.SugaredLogger, *zapobserver.ObservedLogs) {
	observer, logobserver := zapobserver.New(zap.InfoLevel)
	logger := zap.New(observer).Sugar()
	return logger, logobserver
}

func setupFakesURLS() (client VCS, teardown func()) {
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

	gcvs := VCS{
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
					Owner:      "foo",
					Repository: "bar",
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
					Owner:      "pas",
					Repository: "la",
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
					Owner:      "tekton",
					Repository: "dir",
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
					Owner:      "throw",
					Repository: "error",
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
					Owner:      "tekton",
					Repository: "dir",
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
					Owner:      "pas",
					Repository: "la",
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
					Owner:      "its",
					Repository: "afile",
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
					Owner:      "throw",
					Repository: "error",
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
		Owner:      "tekton",
		Repository: "dir",
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
				Owner:  "organization",
				Sender: "me",
			},
			apiReturn: `[{"login": "me"}]`,
			allowed:   true,
			wantErr:   false,
		},
		{
			name: "Check Sender not in Org Membership",
			runevent: info.Event{
				Owner:  "organization",
				Sender: "me",
			},
			apiReturn: `[{"login": "not"}]`,
			allowed:   false,
		},
		{
			name: "Not found on organization",
			runevent: info.Event{
				Owner:  "notfound",
				Sender: "me",
			},
			allowed: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeclient, mux, _, teardown := ghtesthelper.SetupGH()
			defer teardown()
			ctx, _ := rtesting.SetupFakeContext(t)
			gvcs := VCS{
				Client: fakeclient,
			}
			mux.HandleFunc(fmt.Sprintf("/orgs/%s/public_members", tt.runevent.Owner), func(rw http.ResponseWriter, r *http.Request) {
				fmt.Fprint(rw, tt.apiReturn)
			})

			allowed, err := gvcs.checkSenderOrgMembership(ctx, &tt.runevent)
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
			gvcs := VCS{
				Client: fakeclient,
			}
			mux.HandleFunc(fmt.Sprintf("/repos/issues/%s/comments", filepath.Base(tt.runevent.URL)), func(rw http.ResponseWriter, r *http.Request) {
				fmt.Fprint(rw, tt.apiReturn)
			})

			ret, err := gvcs.GetStringPullRequestComment(ctx, &tt.runevent, regexp)
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

func TestVCS_GetCommitInfo(t *testing.T) {
	tests := []struct {
		name             string
		event            *info.Event
		wantErr          bool
		shaurl, shatitle string
	}{
		{
			name: "good",
			event: &info.Event{
				Owner:      "owner",
				Repository: "repository",
				SHA:        "shacommitinfo",
			},
			shaurl:   "https://git.vcs/commit/info",
			shatitle: "My beautiful pony",
		},
		{
			name: "error",
			event: &info.Event{
				Owner:      "owner",
				Repository: "repository",
				SHA:        "shacommitinfo",
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeclient, mux, _, teardown := ghtesthelper.SetupGH()
			defer teardown()
			mux.HandleFunc(fmt.Sprintf("/repos/%s/%s/git/commits/%s",
				tt.event.Owner, tt.event.Repository, tt.event.SHA), func(rw http.ResponseWriter, r *http.Request) {
				if tt.wantErr {
					fmt.Fprint(rw, "XXXXX")
				} else {
					fmt.Fprintf(rw, `{"html_url": "%s", "message": "%s"}`, tt.shaurl, tt.shatitle)
				}
			})
			ctx, _ := rtesting.SetupFakeContext(t)
			gvcs := &VCS{
				Client: fakeclient,
			}

			if err := gvcs.GetCommitInfo(ctx, tt.event); (err != nil) != tt.wantErr {
				t.Errorf("GetCommitInfo() error = %v, wantErr %v", err, tt.wantErr)
			}
			assert.Equal(t, tt.shatitle, tt.event.SHATitle)
			assert.Equal(t, tt.shaurl, tt.event.SHAURL)
		})
	}
}

func TestVCS_SetClient(t *testing.T) {
	tests := []struct {
		name        string
		info        *info.PacOpts
		expectedURL string
		isGHE       bool
	}{
		{
			name: "api url set",
			info: &info.PacOpts{
				VCSAPIURL: "foo.com",
			},
			expectedURL: "https://foo.com",
			isGHE:       true,
		},
		{
			name:        "default to public github",
			expectedURL: "https://api.github.com/",
			info:        &info.PacOpts{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, _ := rtesting.SetupFakeContext(t)
			v := VCS{}
			v.SetClient(ctx, tt.info)
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
