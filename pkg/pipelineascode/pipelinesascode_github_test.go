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

	"github.com/google/go-github/v39/github"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/consoleui"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/clients"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	ghprovider "github.com/openshift-pipelines/pipelines-as-code/pkg/provider/github"
	testclient "github.com/openshift-pipelines/pipelines-as-code/pkg/test/clients"
	testDynamic "github.com/openshift-pipelines/pipelines-as-code/pkg/test/dynamic"
	ghtesthelper "github.com/openshift-pipelines/pipelines-as-code/pkg/test/github"
	kitesthelper "github.com/openshift-pipelines/pipelines-as-code/pkg/test/kubernetestint"
	testnewrepo "github.com/openshift-pipelines/pipelines-as-code/pkg/test/repository"
	tektontest "github.com/openshift-pipelines/pipelines-as-code/pkg/test/tekton"
	pipelinev1beta1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"go.uber.org/zap"
	zapobserver "go.uber.org/zap/zaptest/observer"
	"gotest.tools/v3/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	rtesting "knative.dev/pkg/reconciler/testing"
)

func replyString(mux *http.ServeMux, url, body string) {
	mux.HandleFunc(url, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, body)
	})
}

func testSetupTektonDir(mux *http.ServeMux, runevent info.Event, directory string) {
	var tektonDirContent string
	_ = filepath.Walk(directory, func(path string, info os.FileInfo, err error) error {
		basename := filepath.Base(path)
		trimmed := strings.TrimSuffix(basename, filepath.Ext(basename))
		if info == nil {
			return fmt.Errorf("should not be nil: %s", directory)
		}
		tektonDirContent += fmt.Sprintf(`{
			"name": "%s",
			"path": ".tekton/%s",
			"sha": "shaof%s",
			"size": %d,
			"type": "file"
		},`, basename, basename, trimmed, info.Size())

		contentB, _ := ioutil.ReadFile(path)
		replyString(mux,
			fmt.Sprintf("/repos/%s/%s/git/blobs/shaof%s", runevent.Organization, runevent.Repository, trimmed),
			fmt.Sprintf(`{"encoding": "base64","content": "%s"}`,
				base64.StdEncoding.EncodeToString(contentB)))

		return nil
	})

	replyString(mux,
		fmt.Sprintf("/repos/%s/%s/contents/.tekton", runevent.Organization, runevent.Repository),
		fmt.Sprintf("[%s]", strings.TrimSuffix(tektonDirContent, ",")))
}

func testSetupCommonGhReplies(t *testing.T, mux *http.ServeMux, runevent info.Event,
	finalStatus, finalStatusText string, noReplyOrgPublicMembers bool) {
	// Take a directory and generate replies as Github for it
	replyString(mux,
		fmt.Sprintf("/repos/%s/%s/contents/internal/task", runevent.Organization, runevent.Repository),
		`{"sha": "internaltasksha"}`)

	replyString(mux,
		fmt.Sprintf("/repos/%s/%s/collaborators", runevent.Organization, runevent.Repository), `[]`)

	replyString(mux,
		fmt.Sprintf("/repos/%s/%s/statuses/%s", runevent.Organization, runevent.Repository, runevent.SHA),
		"{}")

	// using 666 as pull request number
	replyString(mux,
		fmt.Sprintf("/repos/%s/%s/issues/666/comments", runevent.Organization, runevent.Repository),
		"{}")

	replyString(mux,
		fmt.Sprintf("/repos/%s/%s/git/commits/%s", runevent.Organization, runevent.Repository, runevent.SHA),
		`{}`)

	if !noReplyOrgPublicMembers {
		mux.HandleFunc("/orgs/"+runevent.Organization+"/public_members", func(rw http.ResponseWriter, r *http.Request) {
			fmt.Fprintf(rw, `[{"login": "%s"}]`, runevent.Sender)
		})
	}

	replyString(mux,
		fmt.Sprintf("/repos/%s/%s/check-runs", runevent.Organization, runevent.Repository),
		`{"id": 26}`)

	mux.HandleFunc(fmt.Sprintf("/repos/%s/%s/check-runs/26", runevent.Organization, runevent.Repository),
		func(w http.ResponseWriter, r *http.Request) {
			body, _ := ioutil.ReadAll(r.Body)
			created := github.CreateCheckRunOptions{}
			err := json.Unmarshal(body, &created)
			assert.NilError(t, err)
			// We created multiple status but the last one should be completed.
			// TODO: we could maybe refine this test
			if created.GetStatus() == "completed" {
				assert.Equal(t, created.GetConclusion(), finalStatus)
				assert.Assert(t, strings.Contains(created.GetOutput().GetText(), finalStatusText),
					"GetStatus/CheckRun %s != %s", created.GetOutput().GetText(), finalStatusText)
			}
		})
}

func TestRun(t *testing.T) {
	observer, _ := zapobserver.New(zap.InfoLevel)
	logger := zap.New(observer).Sugar()
	tests := []struct {
		name                         string
		runevent                     info.Event
		tektondir                    string
		wantErr                      string
		finalStatus                  string
		finalStatusText              string
		repositories                 []*v1alpha1.Repository
		skipReplyingOrgPublicMembers bool
		expectedNumberofCleanups     int
		ProviderInfoFromRepo         bool
	}{
		{
			name: "pull request/apps",
			runevent: info.Event{
				SHA:          "principale",
				Organization: "organizationes",
				Repository:   "lagaffe",
				URL:          "https://service/documentation",
				HeadBranch:   "press",
				BaseBranch:   "main",
				Sender:       "fantasio",
				EventType:    "pull_request",
			},
			tektondir:       "testdata/pull_request",
			finalStatus:     "neutral",
			finalStatusText: "<th>Status</th><th>Duration</th><th>Name</th>",
		},
		{
			name: "pull request/with webhook",
			runevent: info.Event{
				Event: &github.PullRequestEvent{
					PullRequest: &github.PullRequest{
						Number: github.Int(666),
					},
				},
				SHA:          "fromwebhook",
				Organization: "organizationes",
				Repository:   "lagaffe",
				URL:          "https://service/documentation",
				HeadBranch:   "press",
				BaseBranch:   "main",
				Sender:       "fantasio",
				EventType:    "pull_request",
			},
			tektondir:            "testdata/pull_request",
			finalStatus:          "neutral",
			finalStatusText:      "<th>Status</th><th>Duration</th><th>Name</th>",
			ProviderInfoFromRepo: true,
		},
		{
			name: "Push/branch",
			runevent: info.Event{
				SHA:          "principale",
				Organization: "organizationes",
				Repository:   "lagaffe",
				URL:          "https://service/documentation",
				Sender:       "fantasio",
				HeadBranch:   "refs/heads/main",
				BaseBranch:   "refs/heads/main",
				EventType:    "push",
			},
			tektondir:   "testdata/push_branch",
			finalStatus: "neutral",
		},
		{
			name: "Push/tags",
			runevent: info.Event{
				SHA:          "principale",
				Organization: "organizationes",
				Repository:   "lagaffe",
				URL:          "https://service/documentation",
				Sender:       "fantasio",
				HeadBranch:   "refs/tags/0.1",
				BaseBranch:   "refs/tags/0.1",
				EventType:    "push",
			},
			tektondir:   "testdata/push_tags",
			finalStatus: "neutral",
		},

		// Skipped
		{
			name: "Skipped/Test no tekton dir",
			runevent: info.Event{
				SHA:          "principale",
				Organization: "organizationes",
				Repository:   "lagaffe",
				URL:          "https://service/documentation",
				HeadBranch:   "press",
				Sender:       "fantasio",
				BaseBranch:   "nomatch",
				EventType:    "pull_request",
			},
			tektondir:       "",
			finalStatus:     "skipped",
			finalStatusText: "directory for this repository",
		},
		// Skipped
		{
			name: "Skipped/Test on check_run",
			runevent: info.Event{
				SHA:           "principale",
				Organization:  "organizationes",
				Repository:    "lagaffe",
				URL:           "https://service/documentation",
				HeadBranch:    "press",
				Sender:        "fantasio",
				BaseBranch:    "nomatch",
				TriggerTarget: "check_run",
				EventType:     "push",
			},
			tektondir:       "",
			finalStatus:     "skipped",
			finalStatusText: "directory for this repository",
		},
		{
			name: "Skipped/Test no repositories match",
			runevent: info.Event{
				SHA:          "principale",
				Organization: "organizationes",
				Repository:   "lagaffe",
				URL:          "https://service/documentation",
				HeadBranch:   "press",
				Sender:       "fantasio",
				BaseBranch:   "nomatch",
				EventType:    "pull_request",
			},
			tektondir:       "",
			finalStatus:     "skipped",
			finalStatusText: "not find a namespace match",
			repositories: []*v1alpha1.Repository{
				testnewrepo.NewRepo(
					testnewrepo.RepoTestcreationOpts{
						Name:             "test-run",
						URL:              "https://nowhere.com",
						InstallNamespace: "namespace",
					},
				),
			},
		},

		{
			name: "Skipped/User is not allowed",
			runevent: info.Event{
				SHA:          "principale",
				Organization: "organizationes",
				Repository:   "lagaffe",
				URL:          "https://service/documentation",
				HeadBranch:   "press",
				Sender:       "evilbro",
				BaseBranch:   "nomatch",
				EventType:    "pull_request",
			},
			tektondir:                    "testdata/pull_request",
			finalStatus:                  "skipped",
			finalStatusText:              "is not allowed to run CI on this repo",
			skipReplyingOrgPublicMembers: true,
		},
		{
			name: "Keep max number of pipelineruns",
			runevent: info.Event{
				SHA:          "principale",
				Organization: "organizationes",
				Repository:   "lagaffe",
				URL:          "https://service/documentation",
				HeadBranch:   "press",
				BaseBranch:   "main",
				Sender:       "fantasio",
				EventType:    "pull_request",
			},
			tektondir:                "testdata/max-keep-runs",
			finalStatus:              "neutral",
			finalStatusText:          "<th>Status</th><th>Duration</th><th>Name</th>",
			expectedNumberofCleanups: 10,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, _ := rtesting.SetupFakeContext(t)
			fakeclient, mux, ghTestServerURL, teardown := ghtesthelper.SetupGH()
			defer teardown()

			var secretName, providerURL string
			if tt.ProviderInfoFromRepo {
				providerURL = ghTestServerURL
				secretName = "ziesecretee"
			}

			if tt.repositories == nil {
				tt.repositories = []*v1alpha1.Repository{
					testnewrepo.NewRepo(
						testnewrepo.RepoTestcreationOpts{
							Name:             "test-run",
							URL:              tt.runevent.URL,
							InstallNamespace: "namespace",
							SecretName:       secretName,
							ProviderURL:      providerURL,
						},
					),
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
				PipelineRuns: []*pipelinev1beta1.PipelineRun{
					tektontest.MakePR("namespace", "force-me", map[string]*pipelinev1beta1.PipelineRunTaskRunStatus{
						"first":  tektontest.MakePrTrStatus("first", 5),
						"last":   tektontest.MakePrTrStatus("last", 15),
						"middle": tektontest.MakePrTrStatus("middle", 10),
					}),
				},
			}

			testSetupCommonGhReplies(t, mux, tt.runevent, tt.finalStatus, tt.finalStatusText, tt.skipReplyingOrgPublicMembers)
			if tt.tektondir != "" {
				testSetupTektonDir(mux, tt.runevent, tt.tektondir)
			}

			stdata, _ := testclient.SeedTestData(t, ctx, tdata)
			tdc := testDynamic.Options{}
			dc, _ := tdc.Client()
			cs := &params.Run{
				Clients: clients.Clients{
					PipelineAsCode: stdata.PipelineAsCode,
					Log:            logger,
					Kube:           stdata.Kube,
					Tekton:         stdata.Pipeline,
					Dynamic:        dc,
					ConsoleUI:      consoleui.FallBackConsole{},
				},
				Info: info.Info{
					Event: &tt.runevent,
					Pac: &info.PacOpts{
						ProviderInfoFromRepo: tt.ProviderInfoFromRepo,
						SecretAutoCreation:   true,
						ProviderURL:          ghTestServerURL,
						ProviderToken:        "NONE",
					},
				},
			}

			k8int := &kitesthelper.KinterfaceTest{
				ConsoleURL:               "https://console.url",
				ExpectedNumberofCleanups: tt.expectedNumberofCleanups,
				GetSecretResult:          secretName,
			}

			err := Run(ctx, cs, &ghprovider.Provider{
				Client: fakeclient,
				Token:  github.String("None"),
			}, k8int)

			if tt.wantErr != "" {
				assert.ErrorContains(t, err, tt.wantErr)
				return
			}

			assert.NilError(t, err)

			if tt.finalStatus != "skipped" {
				got, err := stdata.PipelineAsCode.PipelinesascodeV1alpha1().Repositories("namespace").Get(
					ctx, "test-run", metav1.GetOptions{})
				assert.NilError(t, err)
				assert.Assert(t, got.Status[len(got.Status)-1].PipelineRunName != "pipelinerun1", "'%s'!='%s'",
					got.Status[len(got.Status)-1].PipelineRunName, "pipelinerun1")
			}
		})
	}
}
