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
	"testing"

	"github.com/google/go-github/v48/github"
	apipac "github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/consoleui"
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

func testSetupCommonGhReplies(t *testing.T, mux *http.ServeMux, runevent info.Event, finalStatus, finalStatusText string, noReplyOrgPublicMembers bool) {
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
			}

			if tt.ProviderInfoFromRepo {
				secrets[repoToken] = "token"
				repo.SecretName = repoToken
				repo.WebhookSecretName = defaultPipelinesAscodeSecretName
				secrets[defaultPipelinesAscodeSecretName] = webhookSecret
			} else {
				secrets[defaultPipelinesAscodeSecretName] = webhookSecret
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
				PipelineRuns: []*pipelinev1beta1.PipelineRun{
					tektontest.MakePR("namespace", "force-me", map[string]*pipelinev1beta1.PipelineRunTaskRunStatus{
						"first":  tektontest.MakePrTrStatus("first", 5),
						"last":   tektontest.MakePrTrStatus("last", 15),
						"middle": tektontest.MakePrTrStatus("middle", 10),
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
						},
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
				prs, err := cs.Clients.Tekton.TektonV1beta1().PipelineRuns("").List(ctx, metav1.ListOptions{})
				assert.NilError(t, err)
				if len(prs.Items) == 0 {
					t.Error("failed to create pipelineRun for case: ", tt.name)
				}
				// validate logURL label
				for _, pr := range prs.Items {
					// skip existing seed pipelineRuns
					if pr.GetName() == "force-me" {
						continue
					}
					logURL, ok := pr.Labels[filepath.Join(apipac.GroupName, "log-url")]
					if !ok {
						logger.Fatalf("failed to find log-url label on pipelinerun: %v/%v", pr.GetNamespace(), pr.GetName())
					}
					assert.Equal(t, logURL, cs.Clients.ConsoleUI.DetailURL(pr.Namespace, pr.Name))
				}
			}
		})
	}
}

func TestPatchPipelineRunWithLogURL(t *testing.T) {
	ctx, _ := rtesting.SetupFakeContext(t)
	observer, _ := zapobserver.New(zap.InfoLevel)
	logger := zap.New(observer).Sugar()

	testPR := tektontest.MakePR("namespace", "force-me", map[string]*pipelinev1beta1.PipelineRunTaskRunStatus{
		"first":  tektontest.MakePrTrStatus("first", 5),
		"last":   tektontest.MakePrTrStatus("last", 15),
		"middle": tektontest.MakePrTrStatus("middle", 10),
	}, nil)

	tdata := testclient.Data{
		PipelineRuns: []*pipelinev1beta1.PipelineRun{testPR},
	}

	stdata, _ := testclient.SeedTestData(t, ctx, tdata)
	fakeClients := clients.Clients{
		Tekton:    stdata.Pipeline,
		ConsoleUI: &consoleui.TektonDashboard{BaseURL: "https://localhost.console"},
	}

	err := patchPipelineRunWithLogURL(ctx, logger, fakeClients, testPR)
	assert.NilError(t, err)

	pr, err := fakeClients.Tekton.TektonV1beta1().PipelineRuns("namespace").Get(ctx, "force-me", metav1.GetOptions{})
	assert.NilError(t, err)
	assert.Equal(t, pr.Annotations[filepath.Join(apipac.GroupName, "log-url")], "https://localhost.console/#/namespaces/namespace/pipelineruns/force-me")
}
