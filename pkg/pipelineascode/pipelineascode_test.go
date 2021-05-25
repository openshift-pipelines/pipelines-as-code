package pipelineascode

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	testDynamic "github.com/tektoncd/cli/pkg/test/dynamic"

	"github.com/google/go-github/v34/github"
	"github.com/jonboulle/clockwork"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli"
	testclient "github.com/openshift-pipelines/pipelines-as-code/pkg/test/clients"
	ghtesthelper "github.com/openshift-pipelines/pipelines-as-code/pkg/test/github"
	kitesthelper "github.com/openshift-pipelines/pipelines-as-code/pkg/test/kubernetestint"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/webvcs"
	"go.uber.org/zap"
	zapobserver "go.uber.org/zap/zaptest/observer"
	"gotest.tools/v3/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis/duck/v1beta1"
	rtesting "knative.dev/pkg/reconciler/testing"
)

const (
	mainBranch      = "mainBranch"
	targetNamespace = "targetNamespace"
	targetURL       = "http://nowhere.togo"
)

func newRepo(name, url, branch, installNamespace, namespace, eventtype string) *v1alpha1.Repository {
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
			Branch:    branch,
			EventType: eventtype,
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
	branch := mainBranch
	d := testclient.Data{
		Repositories: []*v1alpha1.Repository{
			newRepo("test-good", targetURL, branch, targetNamespace, targetNamespace, "pull_request"),
		},
	}
	cs, _ := testclient.SeedTestData(t, ctx, d)
	client := &cli.Clients{PipelineAsCode: cs.PipelineAsCode}
	repo, err := getRepoByCR(ctx, client, &webvcs.RunInfo{URL: targetURL, BaseBranch: branch, EventType: "pull_request"})
	assert.NilError(t, err)
	assert.Equal(t, repo.Spec.Namespace, targetNamespace)
}

func TestFilterByNotMatch(t *testing.T) {
	ctx, _ := rtesting.SetupFakeContext(t)

	// Not matching
	branch := mainBranch
	urlnomatch := "https://psg.com"
	otherurl := "https://marseille.com"
	d := testclient.Data{
		Repositories: []*v1alpha1.Repository{
			newRepo("test-notmatch", urlnomatch, branch, targetNamespace, targetNamespace, "pull_request"),
		},
	}
	cs, _ := testclient.SeedTestData(t, ctx, d)
	client := &cli.Clients{PipelineAsCode: cs.PipelineAsCode}

	repo, err := getRepoByCR(ctx, client, &webvcs.RunInfo{URL: otherurl, BaseBranch: branch})
	assert.NilError(t, err)
	assert.Equal(t, repo.Spec.Namespace, "")
}

func TestFilterByNotInItsNamespace(t *testing.T) {
	// The CR should belong to the namespace it target
	ctx, _ := rtesting.SetupFakeContext(t)
	testname := "test-not-in-its-namespace"
	branch := mainBranch
	installNamespace := "olympiquelyon"
	d := testclient.Data{
		Repositories: []*v1alpha1.Repository{
			newRepo(testname, targetURL, branch, installNamespace, targetNamespace, "pull_request"),
		},
	}
	cs, _ := testclient.SeedTestData(t, ctx, d)
	client := &cli.Clients{PipelineAsCode: cs.PipelineAsCode}
	repo, err := getRepoByCR(ctx, client, &webvcs.RunInfo{URL: targetURL, EventType: "pull_request", BaseBranch: branch})
	assert.ErrorContains(t, err, fmt.Sprintf("repo CR %s matches but belongs to", testname))
	assert.Equal(t, repo.Spec.Namespace, "")
}

func TestFilterForceNamespace(t *testing.T) {
	// The CR should belong to the namespace it target
	t.Skip("TODO: to renable when we get there")
	ctx, _ := rtesting.SetupFakeContext(t)
	testname := "test-not-in-its-namespace"
	branch := mainBranch

	d := testclient.Data{
		Repositories: []*v1alpha1.Repository{
			newRepo(testname, targetURL, branch, targetNamespace, targetNamespace, "pull_request"),
		},
	}
	cs, _ := testclient.SeedTestData(t, ctx, d)
	client := &cli.Clients{PipelineAsCode: cs.PipelineAsCode}
	_, err := getRepoByCR(ctx, client, &webvcs.RunInfo{URL: targetURL, BaseBranch: branch})
	assert.ErrorContains(t, err, "as configured from tekton.yaml on the main branch")
}

func TestRunDeniedFromForcedNamespace(t *testing.T) {
	t.Skip("TODO: Disabled until we reimplement this as annotation")
	ctx, _ := rtesting.SetupFakeContext(t)
	fakeclient, mux, _, teardown := ghtesthelper.SetupGH()
	defer teardown()
	defaultBranch := "default"
	forcedNamespace := "laotronamspace"
	installedNamespace := "nomespace"
	runinfo := &webvcs.RunInfo{
		SHA:           "sha",
		Owner:         "organizatione",
		Repository:    "repo",
		URL:           "https://service/documentation",
		HeadBranch:    "press",
		BaseBranch:    "main",
		DefaultBranch: defaultBranch,
		Sender:        "carlos_valderama",
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

	forcedNamespaceContent := base64.StdEncoding.EncodeToString([]byte("namespace: " + forcedNamespace + "\n"))
	mux.HandleFunc(fmt.Sprintf("/repos/%s/%s/git/blobs/shaofmaintektonyaml", runinfo.Owner, runinfo.Repository),
		func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintf(w, `{"content": "%s"}`, forcedNamespaceContent)
		})

	mux.HandleFunc("/orgs/"+runinfo.Owner+"/public_members", func(rw http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(rw, `[{"login": "%s"}]`, runinfo.Sender)
	})

	gcvs := webvcs.GithubVCS{
		Client: fakeclient,
	}
	datas := testclient.Data{
		Repositories: []*v1alpha1.Repository{
			newRepo("repo", runinfo.URL, runinfo.BaseBranch, installedNamespace, installedNamespace, "pull_request"),
		},
	}
	stdata, _ := testclient.SeedTestData(t, ctx, datas)
	cs := &cli.Clients{
		GithubClient:   gcvs,
		PipelineAsCode: stdata.PipelineAsCode,
	}
	k8int := kitesthelper.KinterfaceTest{}

	err := Run(ctx, cs, &k8int, runinfo)

	assert.Error(t, err,
		fmt.Sprintf("repo CR matches but should be installed in %s as configured from tekton.yaml on the main branch",
			forcedNamespace))
}

func testSetupTektonDir(mux *http.ServeMux, runinfo *webvcs.RunInfo, directory string) {
	var tektonDirContent string
	_ = filepath.Walk(directory, func(path string, info os.FileInfo, err error) error {
		basename := filepath.Base(path)
		trimmed := strings.TrimSuffix(basename, filepath.Ext(basename))
		tektonDirContent += fmt.Sprintf(`{
			"name": "%s",
			"path": ".tekton/%s",
			"sha": "shaof%s",
			"size": %d,
			"type": "file"
		},`, basename, basename, trimmed, info.Size())

		contentB, _ := ioutil.ReadFile(path)
		replyString(mux,
			fmt.Sprintf("/repos/%s/%s/git/blobs/shaof%s", runinfo.Owner, runinfo.Repository, trimmed),
			fmt.Sprintf(`{"encoding": "base64","content": "%s"}`,
				base64.StdEncoding.EncodeToString(contentB)))

		return nil
	})

	replyString(mux,
		fmt.Sprintf("/repos/%s/%s/contents/.tekton", runinfo.Owner, runinfo.Repository),
		fmt.Sprintf("[%s]", strings.TrimSuffix(tektonDirContent, ",")))
}

func testSetupCommonGhReplies(t *testing.T, mux *http.ServeMux, runinfo *webvcs.RunInfo, finalStatus, finalStatusText string,
	noReplyOrgPublicMembers bool) {
	// Take a directory and geneate replies as Github for it
	replyString(mux,
		fmt.Sprintf("/repos/%s/%s/contents/internal/task", runinfo.Owner, runinfo.Repository),
		`{"sha": "internaltasksha"}`)

	if !noReplyOrgPublicMembers {
		mux.HandleFunc("/orgs/"+runinfo.Owner+"/public_members", func(rw http.ResponseWriter, r *http.Request) {
			fmt.Fprintf(rw, `[{"login": "%s"}]`, runinfo.Sender)
		})
	}

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
				assert.Equal(t, created.GetConclusion(), finalStatus)
				assert.Assert(t, strings.Contains(created.GetOutput().GetText(), finalStatusText), "GetStatus/CheckRun %s != %s", created.GetOutput().GetText(), finalStatusText)
			}
		})
}

func TestRun(t *testing.T) {
	observer, log := zapobserver.New(zap.InfoLevel)
	logger := zap.New(observer).Sugar()
	tests := []struct {
		name                         string
		runinfo                      *webvcs.RunInfo
		tektondir                    string
		wantErr                      string
		finalStatus                  string
		finalStatusText              string
		repositories                 []*v1alpha1.Repository
		skipReplyingOrgPublicMembers bool
	}{
		{
			name: "pull request",
			runinfo: &webvcs.RunInfo{
				SHA:        "principale",
				Owner:      "organizationes",
				Repository: "lagaffe",
				URL:        "https://service/documentation",
				HeadBranch: "press",
				BaseBranch: "main",
				Sender:     "fantasio",
				EventType:  "pull_request",
			},
			tektondir:       "testdata/pull_request",
			finalStatus:     "neutral",
			finalStatusText: "More detailed status",
		},
		{
			name: "No match",
			runinfo: &webvcs.RunInfo{
				SHA:        "principale",
				Owner:      "organizationes",
				Repository: "lagaffe",
				URL:        "https://service/documentation",
				HeadBranch: "press",
				Sender:     "fantasio",
				BaseBranch: "nomatch",
				EventType:  "pull_request",
			},
			tektondir:   "testdata/pull_request",
			wantErr:     "cannot match any pipeline",
			finalStatus: "neutral",
		},
		{
			name: "Push/branch",
			runinfo: &webvcs.RunInfo{
				SHA:        "principale",
				Owner:      "organizationes",
				Repository: "lagaffe",
				URL:        "https://service/documentation",
				Sender:     "fantasio",
				HeadBranch: "refs/heads/main",
				BaseBranch: "refs/heads/main",
				EventType:  "push",
			},
			tektondir:   "testdata/push_branch",
			finalStatus: "neutral",
		},
		{
			name: "Push/tags",
			runinfo: &webvcs.RunInfo{
				SHA:        "principale",
				Owner:      "organizationes",
				Repository: "lagaffe",
				URL:        "https://service/documentation",
				Sender:     "fantasio",
				HeadBranch: "refs/tags/0.1",
				BaseBranch: "refs/tags/0.1",
				EventType:  "push",
			},
			tektondir:   "testdata/push_tags",
			finalStatus: "neutral",
		},

		// Skipped
		{
			name: "Skipped/Test no tekton dir",
			runinfo: &webvcs.RunInfo{
				SHA:        "principale",
				Owner:      "organizationes",
				Repository: "lagaffe",
				URL:        "https://service/documentation",
				HeadBranch: "press",
				Sender:     "fantasio",
				BaseBranch: "nomatch",
				EventType:  "pull_request",
			},
			tektondir:       "",
			finalStatus:     "skipped",
			finalStatusText: "directory for this repository",
		},
		{
			name: "Skipped/Test no repositories match",
			runinfo: &webvcs.RunInfo{
				SHA:        "principale",
				Owner:      "organizationes",
				Repository: "lagaffe",
				URL:        "https://service/documentation",
				HeadBranch: "press",
				Sender:     "fantasio",
				BaseBranch: "nomatch",
				EventType:  "pull_request",
			},
			tektondir:       "",
			finalStatus:     "skipped",
			finalStatusText: "not find a namespace match",
			repositories: []*v1alpha1.Repository{
				newRepo("test-run", "https://nowhere.com",
					"a branch", "namespace", "namespace", "pull_request"),
			},
		},
		{
			name: "Skipped/User is not allowed",
			runinfo: &webvcs.RunInfo{
				SHA:        "principale",
				Owner:      "organizationes",
				Repository: "lagaffe",
				URL:        "https://service/documentation",
				HeadBranch: "press",
				Sender:     "evilbro",
				BaseBranch: "nomatch",
				EventType:  "pull_request",
			},
			tektondir:                    "testdata/pull_request",
			finalStatus:                  "skipped",
			finalStatusText:              "is not allowed to run CI on this repo",
			skipReplyingOrgPublicMembers: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, _ := rtesting.SetupFakeContext(t)
			if tt.repositories == nil {
				tt.repositories = []*v1alpha1.Repository{
					newRepo("test-run", tt.runinfo.URL, tt.runinfo.BaseBranch, "namespace", "namespace", tt.runinfo.EventType),
				}
			}
			tdata := testclient.Data{
				Namespaces: []*corev1.Namespace{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "namespace",
						},
					},
				},
				Repositories: tt.repositories,
			}

			fakeclient, mux, _, teardown := ghtesthelper.SetupGH()
			defer teardown()

			testSetupCommonGhReplies(t, mux, tt.runinfo, tt.finalStatus, tt.finalStatusText, tt.skipReplyingOrgPublicMembers)
			if tt.tektondir != "" {
				testSetupTektonDir(mux, tt.runinfo, tt.tektondir)
			}

			stdata, _ := testclient.SeedTestData(t, ctx, tdata)
			tdc := testDynamic.Options{}
			dc, _ := tdc.Client()
			cs := &cli.Clients{
				GithubClient: webvcs.GithubVCS{
					Client: fakeclient,
				},
				PipelineAsCode: stdata.PipelineAsCode,
				Log:            logger,
				Kube:           stdata.Kube,
				Tekton:         stdata.Pipeline,
				Dynamic:        dc,
			}
			k8int := &kitesthelper.KinterfaceTest{
				ConsoleURL: "https://console.url",
			}
			err := Run(ctx, cs, k8int, tt.runinfo)

			if tt.wantErr != "" {
				assert.ErrorContains(t, err, tt.wantErr)
				return
			}

			assert.NilError(t, err)
			assert.Assert(t, len(log.TakeAll()) > 0)

			if tt.finalStatus != "skipped" {
				got, err := stdata.PipelineAsCode.PipelinesascodeV1alpha1().Repositories("namespace").Get(
					ctx, "test-run", metav1.GetOptions{})
				assert.NilError(t, err)
				assert.Assert(t, got.Status[len(got.Status)-1].PipelineRunName != "pipelinerun1")
			}
		})
	}
}
