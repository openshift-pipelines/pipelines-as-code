package pipelineascode

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"testing"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli"
	testhelper "github.com/openshift-pipelines/pipelines-as-code/pkg/test"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/webvcs"
	"gotest.tools/v3/assert"
)

func TestGetInfo(t *testing.T) {
	fakeclient, mux, _, teardown := testhelper.SetupGH()
	defer teardown()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, ``)
	})
	cs := &cli.Clients{
		GithubClient: webvcs.GithubVCS{
			Client: fakeclient,
		},
	}
	goodRunInfo := webvcs.RunInfo{
		Owner:         "foo",
		Repository:    "bar",
		DefaultBranch: "main",
		SHA:           "d0d0",
		URL:           "http://chmouel.com",
		Branch:        "goodRuninfoBranch",
	}

	b, err := ioutil.ReadFile("testdata/pull_request.json")
	assert.NilError(t, err)
	goodPayload := string(b)
	missingValuesPayload := `
	{
  "pull_request": {
    "head": {
      "sha": "sha"
	}
  },
  "repository": {
    "default_branch": "main",
    "html_url": "https://github.com/openshift/tektoncd-pipeline",
    "name": "tektoncd-pipeline",
    "owner": {
      "login": "openshift"
    }
  }
}
	`

	testCases := []struct {
		desc           string
		runinfo        webvcs.RunInfo
		payload        string
		errmsg         string
		branchShouldBe string
	}{
		{
			desc:    "Error on bad payload",
			runinfo: webvcs.RunInfo{},
			payload: "foo bar",
			errmsg:  "invalid character",
		},
		{
			desc:    "Good json but bad payload",
			runinfo: webvcs.RunInfo{},
			payload: "{}",
			errmsg:  "Cannot parse payload",
		},
		{
			desc:    "No payload no runcheck",
			runinfo: webvcs.RunInfo{},
			payload: "",
			errmsg:  "No payload or not enough params",
		},
		{
			desc: "Bad runinfo with missing infos",
			runinfo: webvcs.RunInfo{
				Owner: "foo",
			},
			payload: "",
			errmsg:  "No payload or not enough params",
		},
		{
			desc:    "Missing values payload",
			runinfo: webvcs.RunInfo{},
			payload: missingValuesPayload,
			errmsg:  "missing some values",
		},
		{
			desc:           "Good runinfo",
			runinfo:        goodRunInfo,
			payload:        "",
			errmsg:         "",
			branchShouldBe: "goodRuninfoBranch",
		},
		{
			desc:           "Good payload",
			runinfo:        webvcs.RunInfo{},
			payload:        goodPayload,
			errmsg:         "",
			branchShouldBe: "goodInfoBranch",
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			runinfo, err := getRunInfoFromArgsOrPayload(cs, tC.payload, &tC.runinfo)
			if tC.errmsg != "" {
				assert.ErrorContains(t, err, tC.errmsg)
			} else {
				assert.NilError(t, err)
				assert.Equal(t, runinfo.Branch, tC.branchShouldBe)
			}
		})
	}
}
