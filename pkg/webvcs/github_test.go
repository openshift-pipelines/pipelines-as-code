package webvcs

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"testing"

	testhelper "github.com/openshift-pipelines/pipelines-as-code/pkg/test"
	"gotest.tools/assert"
)

func TestParsePayload(t *testing.T) {
	b, err := ioutil.ReadFile("testdata/pull_request.json")
	assert.NilError(t, err)

	gvcs := GithubVCS{
		Context: context.Background(),
	}
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

func TestWebVCS(t *testing.T) {
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
				  "sha": "eacad9fa044f3d9039bb04c9452eadf0c43e3195",
				  "size": 233,
				  "type": "file"
		     }]`)
	})
	mux.HandleFunc("/repos/pas/la/contents/.tekton", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	gcvs := GithubVCS{
		Client:  fakeclient,
		Context: ctx,
	}

	runinfo := &RunInfo{
		Owner:         "foo",
		Repository:    "bar",
		DefaultBranch: "master",
		SHA:           "1a2b3c",
		Branch:        "testing",
	}
	repoContent, err := gcvs.GetTektonDir(".tekton", runinfo)
	assert.NilError(t, err)
	assert.Assert(t, repoContent != nil)

	// Do not print error if not here because it's okay to not have a .tekton
	// dir
	runinfo = &RunInfo{
		Owner:         "pas",
		Repository:    "la",
		DefaultBranch: "master",
		SHA:           "1a2b3c",
		Branch:        "testing",
	}
	repoContent, err = gcvs.GetTektonDir(".tekton", runinfo)
	assert.Assert(t, err == nil)
	assert.Assert(t, repoContent == nil)

}
