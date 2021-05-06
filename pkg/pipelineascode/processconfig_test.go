package pipelineascode

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/test"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/webvcs"

	// hubtest "github.com/tektoncd/hub/api/pkg/cli/test"
	"gotest.tools/v3/assert"
)

func TestProcessTektonYamlNamespace(t *testing.T) {
	namespace := "liguedestalents"
	data := "namespace: " + namespace
	cs := cli.Clients{}
	ret, err := processTektonYaml(&cs, &webvcs.RunInfo{}, data)
	assert.NilError(t, err)
	assert.Equal(t, ret.Namespace, namespace)
}

func TestProcessTektonYamlRemoteTask(t *testing.T) {
	data := `tasks:
- https://foo.bar
- https://hello.moto
`
	httpTestClient := test.MakeHTTPTestClient(t, 200, "HELLO")
	cs := cli.Clients{HTTPClient: *httpTestClient}
	expected := `
---
HELLO

---
HELLO
`

	ret, err := processTektonYaml(&cs, &webvcs.RunInfo{}, data)
	assert.NilError(t, err)
	if d := cmp.Diff(ret.RemoteTasks, expected); d != "" {
		t.Fatalf("-got, +want: %v", d)
	}
}

func TestProcessTektonYamlRefInternal(t *testing.T) {
	ctx := context.Background()
	expected := "EAT YOUR VEGGIES"
	data := `tasks:
- be/healthy
`
	fakeclient, mux, _, teardown := test.SetupGH()
	defer teardown()

	mux.HandleFunc("/repos/contents/be/healthy", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{
  "name": "healthy",
  "path": "be/healthy",
  "sha": "takepicture"}`)
	})
	mux.HandleFunc("/repos/git/blobs/takepicture", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{
  "name": "healthy",
  "path": "be/healthy",
  "sha": "takepicture",
  "size": 68,
  "content": "%s\n",
  "encoding": "base64"}`,
			base64.StdEncoding.EncodeToString([]byte(expected)))
	})
	mux.HandleFunc("/repos/pas/la/contents/.tekton", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	gcvs := webvcs.GithubVCS{
		Client:  fakeclient,
		Context: ctx,
	}
	cs := &cli.Clients{GithubClient: gcvs}

	ret, err := processTektonYaml(cs, &webvcs.RunInfo{}, data)
	assert.NilError(t, err)

	if d := cmp.Diff(ret.RemoteTasks, "\n---\n"+expected+"\n"); d != "" {
		t.Fatalf("-got, +want: %v", d)
	}
}
