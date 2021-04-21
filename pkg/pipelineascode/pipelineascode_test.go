package pipelineascode

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"testing"

	"github.com/google/go-github/v34/github"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/test"
	testhelper "github.com/openshift-pipelines/pipelines-as-code/pkg/test"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/webvcs"
	"go.uber.org/zap"
	zapobserver "go.uber.org/zap/zaptest/observer"
	"gotest.tools/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	rtesting "knative.dev/pkg/reconciler/testing"
)

type FakeTektonClient struct {
	logOutput      string
	describeOutput string
}

func (t *FakeTektonClient) FollowLogs(string, string) (string, error) {
	return t.logOutput, nil
}

func (t *FakeTektonClient) PipelineRunDescribe(string, string) (string, error) {
	return t.describeOutput, nil
}

func newRepo(name, url, branch, eventType, namespace string) *v1alpha1.Repository {
	return &v1alpha1.Repository{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: v1alpha1.RepositorySpec{
			Namespace: namespace,
			URL:       url,
			Branch:    branch,
			EventType: eventType,
		},
	}
}

func replyString(mux *http.ServeMux, url, body string) {
	mux.HandleFunc(url, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, body)
	})
}

func TestFilterBy(t *testing.T) {
	ctx, _ := rtesting.SetupFakeContext(t)
	testParams := []struct {
		name, namespace, url, branch, eventType string
		nomatch                                 bool
		repositories                            []*v1alpha1.Repository
	}{
		{
			name:         "test-good",
			repositories: []*v1alpha1.Repository{newRepo("test-good", "https://foo/bar", "lovedone", "pull_request", "namespace")},
			url:          "https://foo/bar",
			eventType:    "pull_request",
			branch:       "lovedone",
			namespace:    "namespace",
		},
		{
			name:         "test-notmatch",
			repositories: []*v1alpha1.Repository{newRepo("test-notmatch", "https://foo/bar", "lovedone", "pull_request", "namespace")},
			url:          "https://xyz/vlad",
			eventType:    "pull_request",
			branch:       "lovedone",
			namespace:    "namespace",
			nomatch:      true,
		},
	}

	for _, tp := range testParams {
		t.Run(tp.name, func(t *testing.T) {
			d := test.Data{
				Repositories: tp.repositories,
			}
			cs, _ := test.SeedTestData(t, ctx, d)
			repo, err := getRepoByCRD(&cli.Clients{
				PipelineAsCode: cs.PipelineAsCode,
			}, tp.url, tp.branch, tp.eventType)
			if err != nil {
				t.Fatal(err)
			}

			if tp.nomatch {
				assert.Equal(t, repo.Spec.Namespace, "")
			} else {
				assert.Equal(t, repo.Spec.Namespace, tp.namespace)
			}
		})

	}

}

func TestRun(t *testing.T) {
	ctx, _ := rtesting.SetupFakeContext(t)
	fakeclient, mux, _, teardown := testhelper.SetupGH()
	defer teardown()
	runinfo := &webvcs.RunInfo{
		SHA:        "principale",
		Owner:      "gaston",
		Repository: "lagaffe",
		URL:        "https://service/documentation",
		Branch:     "press",
	}

	replyString(mux,
		fmt.Sprintf("/repos/%s/%s/contents/.tekton", runinfo.Owner, runinfo.Repository),
		`[{

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

	replyString(mux,
		fmt.Sprintf("/repos/%s/%s/contents/internal/task", runinfo.Owner, runinfo.Repository),
		`{"sha": "internaltasksha"}`)

	// Internal task referenced in tekton.yaml
	taskB, err := ioutil.ReadFile("testdata/task.yaml")
	assert.NilError(t, err)
	replyString(mux,
		fmt.Sprintf("/repos/%s/%s/git/blobs/internaltasksha", runinfo.Owner, runinfo.Repository),
		fmt.Sprintf(`{"encoding": "base64","content": "%s="}`,
			base64.RawStdEncoding.EncodeToString(taskB)))

	// Tekton.yaml
	tlB, err := ioutil.ReadFile("testdata/tekton.yaml")
	assert.NilError(t, err)
	replyString(mux,
		fmt.Sprintf("/repos/%s/%s/git/blobs/eacad9fa044f3d9039bb04c9452eadf0c43e3195", runinfo.Owner, runinfo.Repository),
		fmt.Sprintf(`{"encoding": "base64","content": "%s="}`,
			base64.RawStdEncoding.EncodeToString(tlB)))

	// Run.yaml
	prB, err := ioutil.ReadFile("testdata/run.yaml")
	assert.NilError(t, err)
	replyString(mux,
		fmt.Sprintf("/repos/%s/%s/git/blobs/9085026cd00516d1db7101191d61a4371933c735", runinfo.Owner, runinfo.Repository),
		fmt.Sprintf(`{"encoding": "base64","content": "%s"}`,
			base64.RawStdEncoding.EncodeToString(prB)))

	// Pipeline.yaml
	pB, err := ioutil.ReadFile("testdata/pipeline.yaml")
	assert.NilError(t, err)
	replyString(mux,
		fmt.Sprintf("/repos/%s/%s/git/blobs/5f44631b24c740288924767c608af932756d6c1a", runinfo.Owner, runinfo.Repository),
		fmt.Sprintf(`{"encoding": "base64","content": "%s=="}`,
			base64.RawStdEncoding.EncodeToString(pB)))

	replyString(mux,
		fmt.Sprintf("/repos/%s/%s/check-runs", runinfo.Owner, runinfo.Repository),
		`{"id": 26}`)

	mux.HandleFunc(fmt.Sprintf("/repos/%s/%s/check-runs/26", runinfo.Owner, runinfo.Repository),
		func(w http.ResponseWriter, r *http.Request) {
			body, _ := ioutil.ReadAll(r.Body)
			created := github.CreateCheckRunOptions{}
			err := json.Unmarshal(body, &created)
			assert.NilError(t, err)
			assert.Equal(t, created.GetConclusion(), "neutral")
		})

	gcvs := webvcs.GithubVCS{
		Client:  fakeclient,
		Context: ctx,
	}

	repo := test.Data{
		Namespaces: []*corev1.Namespace{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "namespace",
				},
			},
		},
		Repositories: []*v1alpha1.Repository{newRepo(
			"test-run",
			runinfo.URL,
			runinfo.Branch,
			"pull_request",
			"namespace"),
		},
	}
	stdata, _ := test.SeedTestData(t, ctx, repo)

	observer, log := zapobserver.New(zap.InfoLevel)
	logger := zap.New(observer).Sugar()

	cs := &cli.Clients{
		GithubClient:   gcvs,
		PipelineAsCode: stdata.PipelineAsCode,
		Log:            logger,
		Kube:           stdata.Kube,
		Tekton:         stdata.Pipeline,
		TektonCli:      &FakeTektonClient{logOutput: "HELLO MOTO", describeOutput: "DESCRIBE ZEMODO"},
	}

	err = Run(cs, runinfo)
	assert.NilError(t, err)
	assert.Assert(t, len(log.TakeAll()) > 0)
}
