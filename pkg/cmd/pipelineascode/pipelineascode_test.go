package pipelineascode

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"testing"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli"
	pacpkg "github.com/openshift-pipelines/pipelines-as-code/pkg/pipelineascode"
	testclient "github.com/openshift-pipelines/pipelines-as-code/pkg/test/clients"
	ghtesthelper "github.com/openshift-pipelines/pipelines-as-code/pkg/test/github"
	kitesthelper "github.com/openshift-pipelines/pipelines-as-code/pkg/test/kubernetestint"
	tparams "github.com/openshift-pipelines/pipelines-as-code/pkg/test/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/webvcs"
	"go.uber.org/zap"
	zapobserver "go.uber.org/zap/zaptest/observer"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/fs"
	rtesting "knative.dev/pkg/reconciler/testing"
)

func TestCommandTokenSetProperly(t *testing.T) {
	params := tparams.FakeParams{}
	cmd := Command(params)
	e := bytes.NewBufferString("")
	o := bytes.NewBufferString("")
	cmd.SetErr(e)
	cmd.SetOut(o)
	cmd.SetArgs([]string{"--trigger-target", "push"})
	err := cmd.Execute()
	assert.ErrorContains(t, err, "token option is not set properly")
}

func TestRunWrapPR(t *testing.T) {
	ctx, _ := rtesting.SetupFakeContext(t)
	fakeghclient, mux, _, teardown := ghtesthelper.SetupGH()
	stdata, _ := testclient.SeedTestData(t, ctx, testclient.Data{})
	observer, _ := zapobserver.New(zap.InfoLevel)
	fakelogger := zap.New(observer).Sugar()
	checkid := 1234
	defer teardown()

	mux.HandleFunc("/repos/chmouel/scratchmyback/check-runs", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"id": %d}`, checkid)
	})
	mux.HandleFunc("/repos/chmouel/scratchmyback/commits/ref", func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `{"commit": {"message": "HELLO"}}`)
	})
	mux.HandleFunc("/repos/chmouel/scratchmyback/git/commits/ref", func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `{"commit": {"message": "HELLO"}}`)
	})
	mux.HandleFunc(fmt.Sprintf("/repos/chmouel/scratchmyback/check-runs/%d", checkid), func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"id": %d}`, checkid)
	})
	cs := &cli.Clients{
		GithubClient: webvcs.GithubVCS{
			Client: fakeghclient,
		},
		Log:            fakelogger,
		PipelineAsCode: stdata.PipelineAsCode,
	}
	k8int := &kitesthelper.KinterfaceTest{
		ConsoleURL: "https://console.url",
	}

	options := &pacpkg.Options{
		RunInfo: webvcs.RunInfo{
			EventType:     "pull_request",
			TriggerTarget: "pull_request",
		},
	}
	err := runWrap(ctx, options, cs, k8int)
	assert.ErrorContains(t, err, "no payload")

	options.PayloadFile = "testdata/pull_request.json"
	err = runWrap(ctx, options, cs, k8int)
	assert.NilError(t, err)
}

func TestGetInfo(t *testing.T) {
	fakeclient, mux, _, teardown := ghtesthelper.SetupGH()

	defer teardown()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, ``)
	})
	ctx, _ := rtesting.SetupFakeContext(t)
	cs := &cli.Clients{
		GithubClient: webvcs.GithubVCS{
			Client: fakeclient,
		},
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
			runinfo: webvcs.RunInfo{EventType: "pull_request", TriggerTarget: "pull_request"},
			payload: "foo bar",
			errmsg:  "invalid character",
		},
		{
			desc:    "No payload no runcheck",
			runinfo: webvcs.RunInfo{},
			payload: "",
			errmsg:  "unknown X-Github-Event in message:",
		},
		{
			desc: "Bad runinfo with missing infos",
			runinfo: webvcs.RunInfo{
				Owner:     "foo",
				EventType: "pull_request",
			},
			payload: "",
			errmsg:  "unexpected end of JSON input",
		},
		{
			desc:    "Missing values payload",
			runinfo: webvcs.RunInfo{EventType: "pull_request", TriggerTarget: "pull_request"},
			payload: missingValuesPayload,
			errmsg:  "missing some values",
		},
		{
			desc:           "Good payload",
			runinfo:        webvcs.RunInfo{EventType: "pull_request", TriggerTarget: "pull_request"},
			payload:        goodPayload,
			errmsg:         "",
			branchShouldBe: "goodInfoBranch",
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			file := fs.NewFile(t, "test-payload", fs.WithContent(tC.payload))
			defer file.Remove()

			opts := &pacpkg.Options{
				PayloadFile: file.Path(),
				RunInfo:     tC.runinfo,
			}
			runinfo, err := parsePayload(ctx, cs, opts)
			if tC.errmsg != "" {
				assert.ErrorContains(t, err, tC.errmsg)
			} else {
				assert.NilError(t, err)
				assert.Equal(t, runinfo.BaseBranch, tC.branchShouldBe)
			}
		})
	}
}

func TestRunWrap(t *testing.T) {
	ctx, _ := rtesting.SetupFakeContext(t)
	fakeghclient, mux, _, teardown := ghtesthelper.SetupGH()
	stdata, _ := testclient.SeedTestData(t, ctx, testclient.Data{})
	observer, _ := zapobserver.New(zap.InfoLevel)
	fakelogger := zap.New(observer).Sugar()
	checkid := 1234
	mux.HandleFunc("/repos/chmouel/scratchmyback/check-runs", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"id": %d}`, checkid)
	})
	mux.HandleFunc(fmt.Sprintf("/repos/chmouel/scratchmyback/check-runs/%d", checkid), func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"id": %d}`, checkid)
	})
	mux.HandleFunc("/repos/chmouel/scratchmyback/commits/ref", func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `{"commit": {"message": "HELLO"}}`)
	})
	mux.HandleFunc("/repos/chmouel/scratchmyback/git/commits/ref", func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `{"commit": {"message": "HELLO"}}`)
	})

	kinteract := &kitesthelper.KinterfaceTest{
		ConsoleURL: "https://console.url",
	}

	defer teardown()
	cs := &cli.Clients{
		GithubClient: webvcs.GithubVCS{
			Client: fakeghclient,
		},
		Log:            fakelogger,
		PipelineAsCode: stdata.PipelineAsCode,
	}

	tests := []struct {
		name      string
		opts      *pacpkg.Options
		substrErr string
	}{
		{
			name: "good",
			opts: &pacpkg.Options{
				RunInfo:     webvcs.RunInfo{EventType: "pull_request", TriggerTarget: "pull_request"},
				PayloadFile: "testdata/pull_request.json",
			},
		},
		{
			name:      "bad",
			opts:      &pacpkg.Options{},
			substrErr: "no payload",
		},
		{
			name:      "file 404",
			opts:      &pacpkg.Options{PayloadFile: "/nowhere"},
			substrErr: "no such file or directory",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := runWrap(ctx, tt.opts, cs, kinteract)
			if tt.substrErr != "" {
				assert.ErrorContains(t, err, tt.substrErr)
			} else {
				assert.NilError(t, err)
			}
		})
	}
}
