package pipelineascode

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"testing"
	"time"

	testDynamic "github.com/tektoncd/cli/pkg/test/dynamic"

	"github.com/google/go-github/v34/github"
	"github.com/jonboulle/clockwork"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/test"
	testhelper "github.com/openshift-pipelines/pipelines-as-code/pkg/test"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/webvcs"
	"go.uber.org/zap"
	zapobserver "go.uber.org/zap/zaptest/observer"
	"gotest.tools/v3/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis/duck/v1beta1"
	rtesting "knative.dev/pkg/reconciler/testing"
)

func newRepo(name, url, branch, eventType, installNamespace, namespace string) *v1alpha1.Repository {
	cw := clockwork.NewFakeClock()
	return &v1alpha1.Repository{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: installNamespace,
		},
		Spec: v1alpha1.RepositorySpec{
			Namespace: namespace,
			URL:       url,
			EventType: eventType,
			Branch:    branch,
		},
		Status: []v1alpha1.RepositoryRunStatus{
			{
				Status:          v1beta1.Status{},
				PipelineRunName: "pipelinerun5",
				StartTime:       &metav1.Time{Time: cw.Now().Add(-56 * time.Minute)},
				CompletionTime:  &metav1.Time{Time: cw.Now().Add(-55 * time.Minute)},
			},
			{
				Status:          v1beta1.Status{},
				PipelineRunName: "pipelinerun4",
				StartTime:       &metav1.Time{Time: cw.Now().Add(-46 * time.Minute)},
				CompletionTime:  &metav1.Time{Time: cw.Now().Add(-45 * time.Minute)},
			},
			{
				Status:          v1beta1.Status{},
				PipelineRunName: "pipelinerun3",
				StartTime:       &metav1.Time{Time: cw.Now().Add(-36 * time.Minute)},
				CompletionTime:  &metav1.Time{Time: cw.Now().Add(-35 * time.Minute)},
			},
			{
				Status:          v1beta1.Status{},
				PipelineRunName: "pipelinerun2",
				StartTime:       &metav1.Time{Time: cw.Now().Add(-26 * time.Minute)},
				CompletionTime:  &metav1.Time{Time: cw.Now().Add(-25 * time.Minute)},
			},
			{
				Status:          v1beta1.Status{},
				PipelineRunName: "pipelinerun1",
				StartTime:       &metav1.Time{Time: cw.Now().Add(-16 * time.Minute)},
				CompletionTime:  &metav1.Time{Time: cw.Now().Add(-15 * time.Minute)},
			},
		},
	}
}

func replyString(mux *http.ServeMux, url, body string) {
	mux.HandleFunc(url, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, body)
	})
}

func TestFilterByGood(t *testing.T) {
	ctx, _ := rtesting.SetupFakeContext(t)

	// Good and matching
	eventType := "pull_request"
	branch := "mainone"
	targetNamespace := "namespace"
	url := "https://psg.fr"
	d := test.Data{
		Repositories: []*v1alpha1.Repository{
			newRepo("test-good", url, branch, eventType, targetNamespace, targetNamespace),
		},
	}
	cs, _ := test.SeedTestData(t, ctx, d)
	client := &cli.Clients{PipelineAsCode: cs.PipelineAsCode}
	repo, err := getRepoByCR(client, url, branch, eventType, "")
	assert.NilError(t, err)
	assert.Equal(t, repo.Spec.Namespace, targetNamespace)
}

func TestFilterByNotMatch(t *testing.T) {
	ctx, _ := rtesting.SetupFakeContext(t)

	// Not matching
	eventType := "pull_request"
	branch := "mainone"
	targetNamespace := "namespace"
	url := "https://psg.com"
	otherurl := "https://marseille.com"
	d := test.Data{
		Repositories: []*v1alpha1.Repository{
			newRepo("test-notmatch", url, branch, eventType, targetNamespace, targetNamespace),
		},
	}
	cs, _ := test.SeedTestData(t, ctx, d)
	client := &cli.Clients{PipelineAsCode: cs.PipelineAsCode}
	repo, err := getRepoByCR(client, otherurl, branch, eventType, "")
	assert.NilError(t, err)
	assert.Equal(t, repo.Spec.Namespace, "")
}

func TestFilterByNotInItsNamespace(t *testing.T) {
	// The CR should belong to the namespace it target
	ctx, _ := rtesting.SetupFakeContext(t)
	testname := "test-not-in-its-namespace"
	eventType := "pull_request"
	branch := "mainone"
	targetNamespace := "namespace"
	installNamespace := "olympiquelyon"
	url := "https://psg.fr"
	d := test.Data{
		Repositories: []*v1alpha1.Repository{
			newRepo(testname, url, branch, eventType, installNamespace, targetNamespace),
		},
	}
	cs, _ := test.SeedTestData(t, ctx, d)
	client := &cli.Clients{PipelineAsCode: cs.PipelineAsCode}
	repo, err := getRepoByCR(client, url, branch, eventType, "")
	assert.ErrorContains(t, err, fmt.Sprintf("Repo CR %s matches but belongs to", testname))
	assert.Equal(t, repo.Spec.Namespace, "")
}

func TestFilterForceNamespace(t *testing.T) {
	// The CR should belong to the namespace it target
	ctx, _ := rtesting.SetupFakeContext(t)
	testname := "test-not-in-its-namespace"
	eventType := "pull_request"
	branch := "mainone"
	targetNamespace := "namespace"
	forcedNamespace := "asmonaco"

	url := "https://psg.fr"
	d := test.Data{
		Repositories: []*v1alpha1.Repository{
			newRepo(testname, url, branch, eventType, targetNamespace, targetNamespace),
		},
	}
	cs, _ := test.SeedTestData(t, ctx, d)
	client := &cli.Clients{PipelineAsCode: cs.PipelineAsCode}
	_, err := getRepoByCR(client, url, branch, eventType, forcedNamespace)
	assert.ErrorContains(t, err, "as configured from tekton.yaml on the main branch")
}

func TestRunDeniedFromForcedNamespace(t *testing.T) {
	ctx, _ := rtesting.SetupFakeContext(t)
	fakeclient, mux, _, teardown := testhelper.SetupGH()
	defer teardown()
	defaultBranch := "default"
	forcedNamespace := "laotronamspace"
	installedNamespace := "nomespace"
	runinfo := &webvcs.RunInfo{
		SHA:           "principale",
		Owner:         "chmouel",
		Repository:    "repo",
		URL:           "https://service/documentation",
		Branch:        "press",
		DefaultBranch: defaultBranch,
	}
	replyString(mux,
		fmt.Sprintf("/repos/%s/%s/check-runs", runinfo.Owner, runinfo.Repository),
		`{"id": 26}`)

	mux.HandleFunc(fmt.Sprintf("/repos/%s/%s/contents/.tekton/tekton.yaml", runinfo.Owner, runinfo.Repository),
		func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Query().Get("ref") != defaultBranch {
				fmt.Fprint(w, `{}`)
			} else {
				fmt.Fprint(w, `{"sha": "shaofmaintektonyaml"}`)
			}
		})

	forcedNamespaceContent := base64.RawStdEncoding.EncodeToString([]byte("namespace: "+forcedNamespace+"\n")) + "="
	mux.HandleFunc(fmt.Sprintf("/repos/%s/%s/git/blobs/shaofmaintektonyaml", runinfo.Owner, runinfo.Repository),
		func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintf(w, `{"content": "%s"}`, forcedNamespaceContent)
		})

	gcvs := webvcs.GithubVCS{
		Client:  fakeclient,
		Context: ctx,
	}
	datas := test.Data{
		Repositories: []*v1alpha1.Repository{
			newRepo("repo", runinfo.URL, runinfo.Branch, "pull_request", installedNamespace, installedNamespace),
		},
	}
	stdata, _ := test.SeedTestData(t, ctx, datas)
	cs := &cli.Clients{
		GithubClient:   gcvs,
		PipelineAsCode: stdata.PipelineAsCode,
	}
	k8int := test.KinterfaceTest{}

	err := Run(cs, &k8int, runinfo)

	assert.Error(t, err,
		fmt.Sprintf("Repo CR matches but should be installed in \"%s\" as configured from tekton.yaml on the main branch",
			forcedNamespace))
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

			// We created multiple status but the last one should be completed.
			// TODO: we could maybe refine this test
			if created.GetStatus() == "completed" {
				assert.Equal(t, created.GetConclusion(), "neutral")
			}
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
		Repositories: []*v1alpha1.Repository{
			newRepo(
				"test-run",
				runinfo.URL,
				runinfo.Branch,
				"pull_request",
				"namespace",
				"namespace"),
		},
	}
	stdata, _ := test.SeedTestData(t, ctx, repo)

	observer, log := zapobserver.New(zap.InfoLevel)
	logger := zap.New(observer).Sugar()

	tdc := testDynamic.Options{}
	dc, _ := tdc.Client()

	cs := &cli.Clients{
		GithubClient:   gcvs,
		PipelineAsCode: stdata.PipelineAsCode,
		Log:            logger,
		Kube:           stdata.Kube,
		Tekton:         stdata.Pipeline,
		Dynamic:        dc,
	}

	k8int := test.KinterfaceTest{
		ConsoleURL: "https://console.url",
	}

	err = Run(cs, &k8int, runinfo)
	assert.NilError(t, err)
	assert.Assert(t, len(log.TakeAll()) > 0)
	got, err := stdata.PipelineAsCode.PipelinesascodeV1alpha1().Repositories("namespace").Get(
		ctx, "test-run", metav1.GetOptions{})
	assert.NilError(t, err)
	assert.Assert(t, got.Status[len(got.Status)-1].PipelineRunName != "pipelinerun1")
}
