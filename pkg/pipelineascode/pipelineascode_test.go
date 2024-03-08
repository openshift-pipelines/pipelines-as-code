package pipelineascode

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/google/go-github/v56/github"
	apipac "github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/keys"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/consoleui"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/kubeinteraction"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/clients"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/settings"
	ghprovider "github.com/openshift-pipelines/pipelines-as-code/pkg/provider/github"
	testclient "github.com/openshift-pipelines/pipelines-as-code/pkg/test/clients"
	ghtesthelper "github.com/openshift-pipelines/pipelines-as-code/pkg/test/github"
	kitesthelper "github.com/openshift-pipelines/pipelines-as-code/pkg/test/kubernetestint"
	testnewrepo "github.com/openshift-pipelines/pipelines-as-code/pkg/test/repository"
	tektontest "github.com/openshift-pipelines/pipelines-as-code/pkg/test/tekton"
	pipelinev1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"go.uber.org/zap"
	zapobserver "go.uber.org/zap/zaptest/observer"
	"gotest.tools/v3/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	rtesting "knative.dev/pkg/reconciler/testing"
)

const (
	testHubURL         = "https://mybelovedhub"
	testCatalogHubName = "tekton"
)

func replyString(mux *http.ServeMux, url, body string) {
	mux.HandleFunc(url, func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, body)
	})
}

func testSetupCommonGhReplies(t *testing.T, mux *http.ServeMux, runevent info.Event, finalStatus, finalStatusText string, noReplyOrgPublicMembers bool) {
	t.Helper()
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

	jj := fmt.Sprintf(`{"sha": "%s", "html_url": "https://git.commit.url/%s", "message": "commit message"}`,
		runevent.SHA, runevent.SHA)
	replyString(mux,
		fmt.Sprintf("/repos/%s/%s/git/commits/%s", runevent.Organization, runevent.Repository, runevent.SHA),
		jj)

	if !noReplyOrgPublicMembers {
		mux.HandleFunc("/orgs/"+runevent.Organization+"/members", func(rw http.ResponseWriter, r *http.Request) {
			_, _ = fmt.Fprintf(rw, `[{"login": "%s"}]`, runevent.Sender)
		})
	}

	replyString(mux,
		fmt.Sprintf("/repos/%s/%s/check-runs", runevent.Organization, runevent.Repository),
		`{"id": 26}`)

	mux.HandleFunc(fmt.Sprintf("/repos/%s/%s/check-runs/26", runevent.Organization, runevent.Repository),
		func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			created := github.CreateCheckRunOptions{}
			err := json.Unmarshal(body, &created)
			assert.NilError(t, err)
			// We created multiple status but the last one should be completed.
			// TODO: we could maybe refine this test
			if created.GetStatus() == "completed" {
				assert.Equal(t, created.GetConclusion(), finalStatus, "we got the status `%s` but we should have get the status `%s`", created.GetConclusion(), finalStatus)
				assert.Assert(t, strings.Contains(created.GetOutput().GetText(), finalStatusText),
					"GetStatus/CheckRun %s != %s", created.GetOutput().GetText(), finalStatusText)
			}
		})
}

func TestRun(t *testing.T) {
	var hubCatalogs sync.Map
	hubCatalogs.Store(
		"default", settings.HubCatalog{
			ID:   "default",
			URL:  testHubURL,
			Name: testCatalogHubName,
		})
	hubCatalogs.Store(
		"anotherHub", settings.HubCatalog{
			ID:   "anotherHub",
			URL:  testHubURL,
			Name: testCatalogHubName,
		})
	observer, log := zapobserver.New(zap.InfoLevel)
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
		WebHookSecretValue           string
		PayloadEncodedSecret         string
		concurrencyLimit             int
		expectedLogSnippet           string
	}{
		{
			name: "pull request/fail-to-start-apps",
			runevent: info.Event{
				SHA:           "principale",
				Organization:  "organizationes",
				Repository:    "lagaffe",
				URL:           "https://service/documentation",
				HeadBranch:    "press",
				BaseBranch:    "main",
				Sender:        "fantasio",
				EventType:     "pull_request",
				TriggerTarget: "pull_request",
			},
			tektondir:       "testdata/pull_request",
			finalStatus:     "failure",
			finalStatusText: "we need at least one pipelinerun to start with",
		},
		{
			name: "pull request/unknown-remotetask-but-fail-on-matching",
			runevent: info.Event{
				SHA:           "principale",
				Organization:  "organizationes",
				Repository:    "lagaffe",
				URL:           "https://service/documentation",
				HeadBranch:    "press",
				BaseBranch:    "main",
				Sender:        "fantasio",
				EventType:     "push",
				TriggerTarget: "push",
			},
			tektondir:       "testdata/pull_request-nomatch-remotetask",
			finalStatus:     "failure",
			finalStatusText: "we need at least one pipelinerun to start with",
		},
		{
			name: "pull request/match-but-fail-to-start-on-unknown-remotetask",
			runevent: info.Event{
				Event: &github.PullRequestEvent{
					PullRequest: &github.PullRequest{
						Number: github.Int(666),
					},
				},
				SHA:               "fromwebhook",
				Organization:      "owner",
				Sender:            "owner",
				Repository:        "repo",
				URL:               "https://service/documentation",
				HeadBranch:        "press",
				BaseBranch:        "main",
				EventType:         "pull_request",
				TriggerTarget:     "pull_request",
				PullRequestNumber: 666,
				InstallationID:    1234,
			},
			tektondir:       "testdata/pull_request-nomatch-remotetask",
			finalStatus:     "failure",
			finalStatusText: "error getting remote task",
		},
		{
			name: "pull request/allowed",
			runevent: info.Event{
				Event: &github.PullRequestEvent{
					PullRequest: &github.PullRequest{
						Number: github.Int(666),
					},
				},
				SHA:               "fromwebhook",
				Organization:      "owner",
				Sender:            "owner",
				Repository:        "repo",
				URL:               "https://service/documentation",
				HeadBranch:        "press",
				BaseBranch:        "main",
				EventType:         "pull_request",
				TriggerTarget:     "pull_request",
				PullRequestNumber: 666,
				InstallationID:    1234,
			},
			tektondir:       "testdata/pull_request",
			finalStatus:     "neutral",
			finalStatusText: "<th>Status</th><th>Duration</th><th>Name</th>",
		},
		{
			name: "pull request/concurrency limit",
			runevent: info.Event{
				Event: &github.PullRequestEvent{
					PullRequest: &github.PullRequest{
						Number: github.Int(666),
					},
				},
				SHA:               "fromwebhook",
				Organization:      "owner",
				Sender:            "owner",
				Repository:        "repo",
				URL:               "https://service/documentation",
				HeadBranch:        "press",
				BaseBranch:        "main",
				EventType:         "pull_request",
				TriggerTarget:     "pull_request",
				PullRequestNumber: 666,
				InstallationID:    1234,
			},
			tektondir:        "testdata/pull_request",
			finalStatus:      "neutral",
			finalStatusText:  "<th>Status</th><th>Duration</th><th>Name</th>",
			concurrencyLimit: 1,
		},
		{
			name: "pull request/with webhook",
			runevent: info.Event{
				Event: &github.PullRequestEvent{
					PullRequest: &github.PullRequest{
						Number: github.Int(666),
					},
				},
				SHA:               "fromwebhook",
				Organization:      "organizationes",
				Repository:        "lagaffe",
				URL:               "https://service/documentation",
				HeadBranch:        "press",
				BaseBranch:        "main",
				Sender:            "fantasio",
				EventType:         "pull_request",
				TriggerTarget:     "pull_request",
				PullRequestNumber: 666,
			},
			tektondir:            "testdata/pull_request",
			finalStatus:          "neutral",
			finalStatusText:      "<th>Status</th><th>Duration</th><th>Name</th>",
			ProviderInfoFromRepo: true,
		},
		{
			name: "pull request/webhook secret new line",
			runevent: info.Event{
				Event: &github.PullRequestEvent{
					PullRequest: &github.PullRequest{
						Number: github.Int(666),
					},
				},
				SHA:               "fromwebhook",
				Organization:      "organizationes",
				Repository:        "lagaffe",
				URL:               "https://service/documentation",
				HeadBranch:        "press",
				BaseBranch:        "main",
				Sender:            "fantasio",
				EventType:         "pull_request",
				TriggerTarget:     "pull_request",
				PullRequestNumber: 666,
			},
			tektondir:            "testdata/pull_request",
			finalStatus:          "skipped",
			finalStatusText:      "<th>Status</th><th>Duration</th><th>Name</th>",
			ProviderInfoFromRepo: true,
			WebHookSecretValue:   "secret\n",
			PayloadEncodedSecret: "secret",
			expectedLogSnippet:   "it seems that we have detected a \\n or a space at the end",
		},
		{
			name: "pull request/webhook secret space at the end",
			runevent: info.Event{
				Event: &github.PullRequestEvent{
					PullRequest: &github.PullRequest{
						Number: github.Int(666),
					},
				},
				SHA:               "fromwebhook",
				Organization:      "organizationes",
				Repository:        "lagaffe",
				URL:               "https://service/documentation",
				HeadBranch:        "press",
				BaseBranch:        "main",
				Sender:            "fantasio",
				EventType:         "pull_request",
				TriggerTarget:     "pull_request",
				PullRequestNumber: 666,
			},
			tektondir:            "testdata/pull_request",
			finalStatus:          "skipped",
			finalStatusText:      "<th>Status</th><th>Duration</th><th>Name</th>",
			ProviderInfoFromRepo: true,
			WebHookSecretValue:   "secret ",
			PayloadEncodedSecret: "secret",
			expectedLogSnippet:   "it seems that we have detected a \\n or a space at the end",
		},
		{
			name: "Push/branch",
			runevent: info.Event{
				SHA:           "principale",
				Organization:  "organizationes",
				Repository:    "lagaffe",
				URL:           "https://service/documentation",
				Sender:        "fantasio",
				HeadBranch:    "refs/heads/main",
				BaseBranch:    "refs/heads/main",
				EventType:     "push",
				TriggerTarget: "push",
			},
			tektondir:   "testdata/push_branch",
			finalStatus: "neutral",
		},
		{
			name: "Push/tags",
			runevent: info.Event{
				SHA:           "principale",
				Organization:  "organizationes",
				Repository:    "lagaffe",
				URL:           "https://service/documentation",
				Sender:        "fantasio",
				HeadBranch:    "refs/tags/0.1",
				BaseBranch:    "refs/tags/0.1",
				EventType:     "push",
				TriggerTarget: "push",
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
				SHA:           "principale",
				Organization:  "organizationes",
				Repository:    "lagaffe",
				URL:           "https://service/documentation",
				HeadBranch:    "press",
				Sender:        "fantasio",
				BaseBranch:    "nomatch",
				EventType:     "pull_request",
				TriggerTarget: "pull_request",
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
				SHA:           "principale",
				Organization:  "organizationes",
				Repository:    "lagaffe",
				URL:           "https://service/documentation",
				HeadBranch:    "press",
				Sender:        "evilbro",
				BaseBranch:    "nomatch",
				EventType:     "pull_request",
				TriggerTarget: "pull_request",
			},
			tektondir:                    "testdata/pull_request",
			finalStatus:                  "skipped",
			finalStatusText:              "is not allowed to run CI on this repo",
			skipReplyingOrgPublicMembers: true,
		},
		{
			name: "allowed/push event even from non allowed user",
			runevent: info.Event{
				SHA:           "principale",
				Organization:  "organizationes",
				Repository:    "lagaffe",
				URL:           "https://service/documentation",
				HeadBranch:    "press",
				Sender:        "evilbro",
				BaseBranch:    "main",
				EventType:     "push",
				TriggerTarget: "push",
			},
			tektondir:                    "testdata/push_branch",
			finalStatus:                  "skipped",
			skipReplyingOrgPublicMembers: true,
		},
		{
			name: "Keep max number of pipelineruns",
			runevent: info.Event{
				SHA:           "principale",
				Organization:  "organizationes",
				Repository:    "lagaffe",
				URL:           "https://service/documentation",
				HeadBranch:    "press",
				BaseBranch:    "main",
				Sender:        "fantasio",
				EventType:     "pull_request",
				TriggerTarget: "pull_request",
			},
			tektondir:                "testdata/max-keep-runs",
			finalStatus:              "neutral",
			finalStatusText:          "PipelineRun has no taskruns",
			expectedNumberofCleanups: 10,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, _ := rtesting.SetupFakeContext(t)
			fakeclient, mux, ghTestServerURL, teardown := ghtesthelper.SetupGH()
			defer teardown()

			var providerURL string
			secrets := map[string]string{}
			webhookSecret := "don'tlookatmeplease"
			if tt.WebHookSecretValue != "" {
				webhookSecret = tt.WebHookSecretValue
			}
			payloadEncodedSecret := webhookSecret
			if tt.PayloadEncodedSecret != "" {
				payloadEncodedSecret = tt.PayloadEncodedSecret
			}

			repoToken := "repo-token"

			repo := testnewrepo.RepoTestcreationOpts{
				Name:             "test-run",
				URL:              tt.runevent.URL,
				InstallNamespace: "namespace",
				ProviderURL:      providerURL,
				ConcurrencyLimit: tt.concurrencyLimit,
			}

			if tt.ProviderInfoFromRepo {
				secrets[repoToken] = "token"
				repo.SecretName = repoToken
				repo.WebhookSecretName = info.DefaultPipelinesAscodeSecretName
				secrets[info.DefaultPipelinesAscodeSecretName] = webhookSecret
			} else {
				secrets[info.DefaultPipelinesAscodeSecretName] = webhookSecret
			}

			if tt.repositories == nil {
				tt.repositories = []*v1alpha1.Repository{
					testnewrepo.NewRepo(repo),
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
				PipelineRuns: []*pipelinev1.PipelineRun{
					tektontest.MakePRStatus("namespace", "force-me", []pipelinev1.ChildStatusReference{
						tektontest.MakeChildStatusReference("first"),
						tektontest.MakeChildStatusReference("last"),
						tektontest.MakeChildStatusReference("middle"),
					}, nil),
				},
			}

			testSetupCommonGhReplies(t, mux, tt.runevent, tt.finalStatus, tt.finalStatusText, tt.skipReplyingOrgPublicMembers)
			if tt.tektondir != "" {
				ghtesthelper.SetupGitTree(t, mux, tt.tektondir, &tt.runevent, false)
			}

			stdata, _ := testclient.SeedTestData(t, ctx, tdata)
			cs := &params.Run{
				Clients: clients.Clients{
					PipelineAsCode: stdata.PipelineAsCode,
					Log:            logger,
					Kube:           stdata.Kube,
					Tekton:         stdata.Pipeline,
					ConsoleUI:      consoleui.FallBackConsole{},
				},
				Info: info.Info{
					Pac: &info.PacOpts{
						Settings: &settings.Settings{
							SecretAutoCreation: true,
							RemoteTasks:        true,
							HubCatalogs:        &hubCatalogs,
						},
					},
					Controller: &info.ControllerInfo{
						Secret: info.DefaultPipelinesAscodeSecretName,
					},
				},
			}
			mac := hmac.New(sha256.New, []byte(payloadEncodedSecret))
			payload := []byte(`{"iam": "batman"}`)
			mac.Write(payload)
			hexs := hex.EncodeToString(mac.Sum(nil))

			tt.runevent.Request = &info.Request{
				Header: map[string][]string{
					github.SHA256SignatureHeader: {"sha256=" + hexs},
				},
				Payload: payload,
			}
			tt.runevent.Provider = &info.Provider{
				URL:   ghTestServerURL,
				Token: "NONE",
			}
			ctx = info.StoreCurrentControllerName(ctx, "default")
			ctx = info.StoreNS(ctx, repo.InstallNamespace)

			k8int := &kitesthelper.KinterfaceTest{
				ConsoleURL:               "https://console.url",
				ExpectedNumberofCleanups: tt.expectedNumberofCleanups,
				GetSecretResult:          secrets,
			}

			// InstallationID > 0 is used to detect if we are a GitHub APP
			tt.runevent.InstallationID = 12345
			if tt.ProviderInfoFromRepo {
				tt.runevent.InstallationID = 0
			}

			vcx := &ghprovider.Provider{
				Client: fakeclient,
				Run:    cs,
				Token:  github.String("None"),
				Logger: logger,
			}
			p := NewPacs(&tt.runevent, vcx, cs, k8int, logger)
			err := p.Run(ctx)

			if tt.wantErr != "" {
				assert.ErrorContains(t, err, tt.wantErr)
				return
			}

			assert.NilError(t, err)

			if tt.expectedLogSnippet != "" {
				logmsg := log.FilterMessageSnippet(tt.expectedLogSnippet).TakeAll()
				assert.Assert(t, len(logmsg) > 0, "log messages", logmsg, tt.expectedLogSnippet)
			}

			if tt.finalStatus != "skipped" {
				prs, err := cs.Clients.Tekton.TektonV1().PipelineRuns("").List(ctx, metav1.ListOptions{})
				assert.NilError(t, err)
				if len(prs.Items) == 0 {
					t.Error("failed to create pipelineRun for case: ", tt.name)
				}
				// validate logURL label
				for i := range prs.Items {
					pr := prs.Items[i]
					// skip existing seeded PipelineRuns
					if pr.GetName() == "force-me" {
						continue
					}
					logURL, ok := pr.Annotations[filepath.Join(apipac.GroupName, "log-url")]
					assert.Assert(t, ok, "failed to find log-url label on pipelinerun: %s/%s", pr.GetNamespace(), pr.GetGenerateName())
					assert.Equal(t, logURL, cs.Clients.ConsoleUI.DetailURL(&pr))

					if cs.Info.Pac.SecretAutoCreation {
						secretName, ok := pr.GetAnnotations()[keys.GitAuthSecret]
						assert.Assert(t, ok, "Cannot find secret %s on annotations", secretName)
					}
					if tt.concurrencyLimit > 0 {
						concurrencyLimit, ok := pr.GetAnnotations()[keys.State]
						assert.Assert(t, ok, "State hasn't been set on PR", concurrencyLimit)
						assert.Equal(t, concurrencyLimit, kubeinteraction.StateQueued)
					}
				}
			}
		})
	}
}

func TestGetLogURLMergePatch(t *testing.T) {
	con := consoleui.FallBackConsole{}
	clients := clients.Clients{
		ConsoleUI: con,
	}
	pr := &pipelinev1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-pipeline-run",
		},
	}
	result := getLogURLMergePatch(clients, pr)
	m, ok := result["metadata"].(map[string]interface{})
	assert.Assert(t, ok)
	a, ok := m["annotations"].(map[string]string)
	assert.Assert(t, ok)
	assert.Equal(t, a[filepath.Join(apipac.GroupName, "log-url")], con.URL())
}
