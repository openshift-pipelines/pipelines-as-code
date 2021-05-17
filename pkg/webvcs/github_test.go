package webvcs

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"reflect"
	"strings"
	"testing"

	ghtesthelper "github.com/openshift-pipelines/pipelines-as-code/pkg/test/github"
	rtesting "knative.dev/pkg/reconciler/testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-github/v34/github"
	"go.uber.org/zap"
	zapobserver "go.uber.org/zap/zaptest/observer"
	"gotest.tools/v3/assert"
)

func getLogger() (*zap.SugaredLogger, *zapobserver.ObservedLogs) {
	observer, logobserver := zapobserver.New(zap.InfoLevel)
	logger := zap.New(observer).Sugar()
	return logger, logobserver
}

func setupFakesURLS() (client GithubVCS, teardown func()) {
	fakeclient, mux, _, teardown := ghtesthelper.SetupGH()

	mux.HandleFunc("/repos/check/run/check-runs", func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `{"id": 555}`)
	})

	mux.HandleFunc("/repos/check/run/check-runs/2026", func(w http.ResponseWriter, r *http.Request) {
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
				},
				{
				  "name": "tekton.yaml",
				  "path": ".tekton/tekton.yaml",
				  "sha": "tektonyaml",
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

	gcvs := GithubVCS{
		Client: fakeclient,
	}
	return gcvs, teardown
}

func TestPayLoadFix(t *testing.T) {
	b, err := ioutil.ReadFile("testdata/pull_request_with_newlines.json")
	assert.NilError(t, err)
	ctx, _ := rtesting.SetupFakeContext(t)
	gvcs := NewGithubVCS("none")
	logger, _ := getLogger()
	_, err = gvcs.ParsePayload(ctx, logger, "pull_request", payloadFix(string(b)))
	// would bomb out on "assertion failed: error is not nil: invalid character
	// '\n' in string literal" if we don't payloadfix
	assert.NilError(t, err)
}

func TestParsePayloadRerequest(t *testing.T) {
	checkrunSender := "jean-pierre"
	prOwner := "openshift"
	repoName := "pipelines"
	prNumber := "123"
	checkrunEvent := fmt.Sprintf(`{"action": "rerequested", 
	"sender": {"login": "%s"},
	"check_run": {"check_suite": {"pull_requests": [{"number": %s}]}}, 
	"repository": {"name": "%s", "owner": {"login": "%s"}}}`,
		checkrunSender, prNumber, repoName, prOwner)
	fakeclient, mux, _, teardown := ghtesthelper.SetupGH()
	defer teardown()
	mux.HandleFunc("/repos/"+prOwner+"/"+repoName+"/pulls/"+prNumber, func(rw http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprintf(rw, `{"head": {"ref": "123"}, "user": {"login": "%s"}}`, prOwner)
	})
	ctx, _ := rtesting.SetupFakeContext(t)
	gvcs := GithubVCS{
		Client: fakeclient,
	}
	logger, observer := getLogger()
	runinfo, err := gvcs.ParsePayload(ctx, logger, "check_run", checkrunEvent)
	assert.NilError(t, err)

	assert.Equal(t, prOwner, runinfo.Owner)
	assert.Equal(t, repoName, runinfo.Repository)
	assert.Assert(t, checkrunSender != runinfo.Sender)

	assert.Assert(t, strings.Contains(observer.TakeAll()[0].Message, "Recheck of PR"))
}

func TestParsePayLoadRetest(t *testing.T) {
	issueSender := "tartanpion"
	prOwner := "user1"
	repoOwner := "openshift"
	repoName := "pipelines"
	prNumber := "123"

	fakeclient, mux, _, teardown := ghtesthelper.SetupGH()
	defer teardown()
	mux.HandleFunc("/repos/"+prOwner+"/"+repoName+"/pulls/"+prNumber, func(rw http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprintf(rw, `{"head": {"ref": "123"}, "user": {"login": "%s"}}`, prOwner)
	})

	issueEvent := fmt.Sprintf(`{
  "sender": {
    "login": "%s"
  },
  "repository": {
    "name": "%s",
    "owner": {
      "login": "%s"
    }
  },
  "issue": {
    "pull_request": {
      "html_url": "https://github.com/%s/%s/pull/%s"
    }
  }
}`, issueSender, repoName, prOwner, repoName, repoOwner, prNumber)

	ctx, _ := rtesting.SetupFakeContext(t)
	logger, observer := getLogger()
	gvcs := GithubVCS{
		Client: fakeclient,
	}

	runinfo, err := gvcs.ParsePayload(ctx, logger, "issue_comment", issueEvent)
	assert.NilError(t, err)
	assert.Equal(t, prOwner, runinfo.Owner)
	// Make sure the PR owner is the runinfo.Owner and not the issueSender
	assert.Assert(t, issueSender != runinfo.Owner)
	firstObservedMessage := observer.TakeAll()[0].Message
	assert.Assert(t, strings.Contains(firstObservedMessage, "recheck"))
}

func TestParsePayload(t *testing.T) {
	b, err := ioutil.ReadFile("testdata/pull_request.json")
	assert.NilError(t, err)

	ctx, _ := rtesting.SetupFakeContext(t)
	gvcs := NewGithubVCS("none")
	logger, _ := getLogger()

	runinfo, err := gvcs.ParsePayload(ctx, logger, "pull_request", string(b))
	assert.NilError(t, err)
	assert.Assert(t, runinfo.BaseBranch == "master")
	assert.Assert(t, runinfo.Owner == "chmouel")
	assert.Assert(t, runinfo.Repository == "scratchpad")
	assert.Assert(t, runinfo.URL == "https://github.com/chmouel/scratchpad")
}

func TestParsePayloadInvalid(t *testing.T) {
	ctx, _ := rtesting.SetupFakeContext(t)
	gvcs := NewGithubVCS("none")
	logger, _ := getLogger()
	_, err := gvcs.ParsePayload(ctx, logger, "pull_request", "hello moto")
	assert.ErrorContains(t, err, "invalid character")
}

func TestParsePayloadUnkownEvent(t *testing.T) {
	ctx, _ := rtesting.SetupFakeContext(t)
	gvcs := NewGithubVCS("none")
	logger, _ := getLogger()
	_, err := gvcs.ParsePayload(ctx, logger, "foo", "{\"hello\": \"moto\"}")
	assert.ErrorContains(t, err, "unknown X-Github-Event")
}

func TestParsePayCannotParse(t *testing.T) {
	gvcs := NewGithubVCS("none")
	ctx, _ := rtesting.SetupFakeContext(t)
	logger, _ := getLogger()
	_, err := gvcs.ParsePayload(ctx, logger, "gollum", "{}")
	assert.Error(t, err, "this event is not supported")
}

func TestGetFileInsideRepo(t *testing.T) {
	ctx, _ := rtesting.SetupFakeContext(t)
	gcvs, teardown := setupFakesURLS()
	defer teardown()
	type args struct {
		path      string
		runinfo   *RunInfo
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
				runinfo: &RunInfo{
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
				runinfo: &RunInfo{
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
				runinfo: &RunInfo{
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
				runinfo: &RunInfo{
					Owner:      "throw",
					Repository: "error",
				},
			},
		},
	}
	for _, tt := range testGetTektonDir {
		t.Run(tt.name, func(t *testing.T) {
			got, err := gcvs.GetFileInsideRepo(ctx, tt.args.path, false, tt.args.runinfo)
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
		runinfo   *RunInfo
		assertion func(t *testing.T, got []*github.RepositoryContent, err error)
	}

	testGetTektonDir := []struct {
		name string
		args args
	}{
		{
			name: "testgood",
			args: args{
				assertion: func(t *testing.T, got []*github.RepositoryContent, err error) {
					assert.NilError(t, err)
					assert.Assert(t, got != nil)
				},
				path: ".tekton",
				runinfo: &RunInfo{
					Owner:      "tekton",
					Repository: "dir",
				},
			},
		},
		{
			name: "notfound",
			args: args{
				assertion: func(t *testing.T, got []*github.RepositoryContent, err error) {
					assert.NilError(t, err)
					assert.Assert(t, got == nil)
				},
				path: ".tekton",
				runinfo: &RunInfo{
					Owner:      "pas",
					Repository: "la",
				},
			},
		},
		{
			name: "tektondirisafile",
			args: args{
				assertion: func(t *testing.T, got []*github.RepositoryContent, err error) {
					assert.Error(t, err, "the object .tekton is a file instead of a directory")
					assert.Assert(t, got == nil)
				},
				path: ".tekton",
				runinfo: &RunInfo{
					Owner:      "its",
					Repository: "afile",
				},
			},
		},
		{
			name: "throwerror",
			args: args{
				assertion: func(t *testing.T, got []*github.RepositoryContent, err error) {
					assert.ErrorContains(t, err, "invalid character")
					assert.Assert(t, got == nil)
				},
				path: ".tekton",
				runinfo: &RunInfo{
					Owner:      "throw",
					Repository: "error",
				},
			},
		},
	}
	for _, tt := range testGetTektonDir {
		t.Run(tt.name, func(t *testing.T) {
			got, err := gcvs.GetTektonDir(ctx, tt.args.path, tt.args.runinfo)
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
	runinfo := &RunInfo{
		Owner:      "tekton",
		Repository: "dir",
	}

	ghr, err := gcvs.GetTektonDir(ctx, ".tekton", runinfo)
	assert.NilError(t, err)

	got, err := gcvs.GetTektonDirTemplate(ctx, ghr, runinfo)
	assert.NilError(t, err)
	if d := cmp.Diff(got, expected); d != "" {
		t.Fatalf("-got, +want: %v", d)
	}
}

func TestGithubVCS_CreateCheckRun(t *testing.T) {
	ctx, _ := rtesting.SetupFakeContext(t)
	gcvs, teardown := setupFakesURLS()
	defer teardown()
	runinfo := &RunInfo{
		Owner:      "check",
		Repository: "run",
	}
	cr, err := gcvs.CreateCheckRun(ctx, "hello moto", runinfo)
	assert.NilError(t, err)
	assert.Equal(t, cr.GetID(), int64(555))
}

func TestRunInfoCheck(t *testing.T) {
	type fields struct {
		Owner         string
		Repository    string
		DefaultBranch string
		SHA           string
		URL           string
		Sender        string
		CheckRunID    *int64
		BaseBranch    string
		HeadBranch    string
	}
	tests := []struct {
		name    string
		fields  fields
		wantErr bool
	}{
		{
			name:    "testerr",
			fields:  fields{Owner: "hello"},
			wantErr: true,
		},
		{
			name: "testgood",
			fields: fields{
				Owner:         "hello",
				Repository:    "moto",
				DefaultBranch: "default",
				SHA:           "1d1",
				URL:           "https://anywhere",
				BaseBranch:    "main",
				Sender:        "beeboo",
				HeadBranch:    "pr",
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := RunInfo{
				Owner:         tt.fields.Owner,
				Repository:    tt.fields.Repository,
				DefaultBranch: tt.fields.DefaultBranch,
				SHA:           tt.fields.SHA,
				URL:           tt.fields.URL,
				BaseBranch:    tt.fields.BaseBranch,
				HeadBranch:    tt.fields.HeadBranch,
				CheckRunID:    tt.fields.CheckRunID,
				Sender:        tt.fields.Sender,
				EventType:     "pull_request",
			}
			if err := r.Check(); (err != nil) != tt.wantErr {
				t.Errorf("RunInfo.Check() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCreateStatus(t *testing.T) {
	ctx, _ := rtesting.SetupFakeContext(t)
	gcvs, teardown := setupFakesURLS()
	checkrunid := int64(2026)
	defer teardown()
	runinfo := &RunInfo{
		Owner:      "check",
		Repository: "run",
		CheckRunID: &checkrunid,
	}
	cr, err := gcvs.CreateStatus(ctx, runinfo, "completed", "success", "Yay", "https://foo/bar")
	assert.NilError(t, err)
	assert.Equal(t, cr.GetID(), int64(666))
}

func TestGithubVCS_CreateStatus(t *testing.T) {
	checkrunid := int64(2026)
	resultid := int64(666)
	runinfo := &RunInfo{Owner: "check", Repository: "run", CheckRunID: &checkrunid}

	type args struct {
		runinfo            *RunInfo
		status             string
		conclusion         string
		text               string
		detailsURL         string
		titleSubstr        string
		nilCompletedAtDate bool
	}
	tests := []struct {
		name    string
		args    args
		want    *github.CheckRun
		wantErr bool
	}{
		{
			name: "success",
			args: args{
				runinfo:     runinfo,
				status:      "completed",
				conclusion:  "success",
				text:        "Yay",
				detailsURL:  "https://cireport.com",
				titleSubstr: "Success",
			},
			want:    &github.CheckRun{ID: &resultid},
			wantErr: false,
		},
		{
			name: "in_progress",
			args: args{
				runinfo:            runinfo,
				status:             "in_progress",
				conclusion:         "",
				text:               "Yay",
				detailsURL:         "https://cireport.com",
				nilCompletedAtDate: true,
			},
			want:    &github.CheckRun{ID: &resultid},
			wantErr: false,
		},
		{
			name: "failure",
			args: args{
				runinfo:     runinfo,
				status:      "completed",
				conclusion:  "failure",
				text:        "Nay",
				detailsURL:  "https://cireport.com",
				titleSubstr: "Failed",
			},
			want:    &github.CheckRun{ID: &resultid},
			wantErr: false,
		},
		{
			name: "skipped",
			args: args{
				runinfo:     runinfo,
				status:      "completed",
				conclusion:  "skipped",
				text:        "Skipit",
				detailsURL:  "https://cireport.com",
				titleSubstr: "Skipped",
			},
			want:    &github.CheckRun{ID: &resultid},
			wantErr: false,
		},
		{
			name: "unknown",
			args: args{
				runinfo:     runinfo,
				status:      "completed",
				conclusion:  "neutral",
				text:        "Je sais pas ce qui se passe wesh",
				detailsURL:  "https://cireport.com",
				titleSubstr: "Unknown",
			},
			want:    &github.CheckRun{ID: &resultid},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeclient, mux, _, teardown := ghtesthelper.SetupGH()
			defer teardown()

			ctx, _ := rtesting.SetupFakeContext(t)
			gcvs := GithubVCS{
				Client: fakeclient,
			}
			mux.HandleFunc(fmt.Sprintf("/repos/check/run/check-runs/%d", checkrunid), func(rw http.ResponseWriter, r *http.Request) {
				bit, _ := ioutil.ReadAll(r.Body)
				checkRun := &github.CheckRun{}
				err := json.Unmarshal(bit, checkRun)
				assert.NilError(t, err)

				if tt.args.nilCompletedAtDate {
					// I guess that's the way you check for an undefined year,
					// or maybe i don't understand fully how go works😅
					assert.Assert(t, checkRun.GetCompletedAt().Year() == 0o001)
				}
				assert.Equal(t, checkRun.GetStatus(), tt.args.status)
				assert.Equal(t, checkRun.GetConclusion(), tt.args.conclusion)
				assert.Equal(t, checkRun.Output.GetText(), tt.args.text)
				assert.Equal(t, checkRun.GetDetailsURL(), tt.args.detailsURL)
				assert.Assert(t, strings.Contains(checkRun.Output.GetTitle(), tt.args.titleSubstr))
				_, err = fmt.Fprintf(rw, `{"id": %d}`, resultid)
				assert.NilError(t, err)
			})

			got, err := gcvs.CreateStatus(ctx, tt.args.runinfo, tt.args.status, tt.args.conclusion, tt.args.text, tt.args.detailsURL)
			if (err != nil) != tt.wantErr {
				t.Errorf("GithubVCS.CreateStatus() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GithubVCS.CreateStatus() = %v, want %v", got, tt.want)
			}
		})
	}
}
