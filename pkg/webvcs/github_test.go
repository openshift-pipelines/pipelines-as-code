package webvcs

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"reflect"
	"testing"

	"github.com/google/go-github/v34/github"
	testhelper "github.com/openshift-pipelines/pipelines-as-code/pkg/test"
	"gotest.tools/assert"
)

func TestParsePayload(t *testing.T) {
	b, err := ioutil.ReadFile("testdata/pull_request.json")
	assert.NilError(t, err)

	gvcs := NewGithubVCS("none")

	runinfo, err := gvcs.ParsePayload(string(b))
	assert.NilError(t, err)
	assert.Assert(t, runinfo.Branch == "master")
	assert.Assert(t, runinfo.Owner == "chmouel")
	assert.Assert(t, runinfo.Repository == "scratchpad")
	assert.Assert(t, runinfo.URL == "https://github.com/chmouel/scratchpad")

	_, err = gvcs.ParsePayload("hello moto")
	assert.ErrorContains(t, err, "invalid character")

	_, err = gvcs.ParsePayload("{\"hello\": \"moto\"}")
	assert.Error(t, err, "Cannot parse payload as PR")
}

func TestGetTektonDir(t *testing.T) {
	ctx := context.Background()
	fakeclient, mux, _, teardown := testhelper.SetupGH()
	defer teardown()

	mux.HandleFunc("/repos/foo/bar/contents/.tekton", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `[{

				  "name": "pipeline.yaml",
				  "path": ".tekton/pipeline.yaml",
				  "sha": "5f44631b24c740288924767c608af932756d6c1a",
				  "size": 1186,
				  "type": "file"
				},
				{
				  "name": "run.yaml",
				  "path": ".tekton/run.yaml",
				  "sha": "9085026cd00516d1db7101191d61a4371933c735",
				  "size": 464,
				  "type": "file"
				},
				{
				  "name": "tekton.yaml",
				  "path": ".tekton/tekton.yaml",
				  "sha": "yolo"
		     }]`)
	})
	mux.HandleFunc("/repos/throw/error/contents/.tekton", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "ERRROR")
	})

	mux.HandleFunc("/repos/its/afile/contents/.tekton", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{
"name": ".tekton",
"sha": "decfae2653959f7c6c25f21f026c3819bea41ecf",
"type": "file",
"content": "WyFbQ29udGFpbmVyIFJlcG9zaXRvcnkgb24gUXVheV0oaHR0cHM6Ly9xdWF5\nLmlvL3JlcG9zaXRvcnkvY2htb3VlbC90ZWt0b24tYXNhLWNvZGUvc3RhdHVz"}`)
	})
	mux.HandleFunc("/repos/pas/la/contents/.tekton", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	gcvs := GithubVCS{
		Client:  fakeclient,
		Context: ctx,
	}

	type args struct {
		path      string
		runinfo   *RunInfo
		assertion func(t *testing.T, got []*github.RepositoryContent, err error)
	}
	tests := []struct {
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
					Owner:      "foo",
					Repository: "bar",
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
					assert.Error(t, err, "The object .tekton is a file instead of a directory")
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
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := gcvs.GetTektonDir(tt.args.path, tt.args.runinfo)
			tt.args.assertion(t, got, err)
		})
	}
}

func TestGetFileInsideRepo(t *testing.T) {
	type fields struct {
		Context context.Context
		Client  *github.Client
	}
	type args struct {
		path    string
		runinfo *RunInfo
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    string
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := GithubVCS{
				Context: tt.fields.Context,
				Client:  tt.fields.Client,
			}
			got, err := v.GetFileInsideRepo(tt.args.path, tt.args.runinfo)
			if (err != nil) != tt.wantErr {
				t.Errorf("GithubVCS.GetFileInsideRepo() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("GithubVCS.GetFileInsideRepo() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGithubVCS_GetTektonDirTemplate(t *testing.T) {
	type fields struct {
		Context context.Context
		Client  *github.Client
	}
	type args struct {
		objects []*github.RepositoryContent
		runinfo *RunInfo
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    string
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := GithubVCS{
				Context: tt.fields.Context,
				Client:  tt.fields.Client,
			}
			got, err := v.GetTektonDirTemplate(tt.args.objects, tt.args.runinfo)
			if (err != nil) != tt.wantErr {
				t.Errorf("GithubVCS.GetTektonDirTemplate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("GithubVCS.GetTektonDirTemplate() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGithubVCS_GetObject(t *testing.T) {
	type fields struct {
		Context context.Context
		Client  *github.Client
	}
	type args struct {
		sha     string
		runinfo *RunInfo
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    []byte
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := GithubVCS{
				Context: tt.fields.Context,
				Client:  tt.fields.Client,
			}
			got, err := v.GetObject(tt.args.sha, tt.args.runinfo)
			if (err != nil) != tt.wantErr {
				t.Errorf("GithubVCS.GetObject() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GithubVCS.GetObject() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGithubVCS_CreateCheckRun(t *testing.T) {
	type fields struct {
		Context context.Context
		Client  *github.Client
	}
	type args struct {
		status  string
		runinfo *RunInfo
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *github.CheckRun
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := GithubVCS{
				Context: tt.fields.Context,
				Client:  tt.fields.Client,
			}
			got, err := v.CreateCheckRun(tt.args.status, tt.args.runinfo)
			if (err != nil) != tt.wantErr {
				t.Errorf("GithubVCS.CreateCheckRun() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GithubVCS.CreateCheckRun() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGithubVCS_CreateStatus(t *testing.T) {
	type fields struct {
		Context context.Context
		Client  *github.Client
	}
	type args struct {
		runinfo    *RunInfo
		status     string
		conclusion string
		text       string
		detailURL  string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *github.CheckRun
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := GithubVCS{
				Context: tt.fields.Context,
				Client:  tt.fields.Client,
			}
			got, err := v.CreateStatus(tt.args.runinfo, tt.args.status, tt.args.conclusion, tt.args.text, tt.args.detailURL)
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
