package github

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/google/go-github/v81/github"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/keys"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/clients"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/settings"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider"
	testclient "github.com/openshift-pipelines/pipelines-as-code/pkg/test/clients"
	ghtesthelper "github.com/openshift-pipelines/pipelines-as-code/pkg/test/github"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/test/logger"
	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	rtesting "knative.dev/pkg/reconciler/testing"
)

func TestGithubProviderCreateCheckRun(t *testing.T) {
	ctx, _ := rtesting.SetupFakeContext(t)
	fakeclient, mux, _, teardown := ghtesthelper.SetupGH()
	cnx := Provider{
		ghClient: fakeclient,
		Run:      params.New(),
		pacInfo: &info.PacOpts{
			Settings: settings.Settings{
				ApplicationName: settings.PACApplicationNameDefaultValue,
			},
		},
	}
	defer teardown()
	mux.HandleFunc("/repos/check/info/check-runs", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, `{"id": 555}`)
	})

	mux.HandleFunc("/repos/check/info/check-runs/555", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, `{"id": 555}`)
	})

	event := &info.Event{
		Organization: "check",
		Repository:   "info",
		SHA:          "createCheckRunSHA",
	}

	err := cnx.getOrUpdateCheckRunStatus(ctx, event, provider.StatusOpts{
		PipelineRunName: "pr1",
		Status:          "hello moto",
	})
	assert.NilError(t, err)
}

func TestGetOrUpdateCheckRunStatusForMultipleFailedPipelineRun(t *testing.T) {
	ctx, _ := rtesting.SetupFakeContext(t)
	fakeclient, mux, _, teardown := ghtesthelper.SetupGH()
	cnx := Provider{
		ghClient: fakeclient,
		Run:      params.New(),
		pacInfo:  &info.PacOpts{},
	}
	defer teardown()
	statusOptionData := []provider.StatusOpts{{
		PipelineRunName:          "",
		Title:                    "Failed",
		InstanceCountForCheckRun: 0,
	}, {
		PipelineRunName:          "",
		Title:                    "Failed",
		InstanceCountForCheckRun: 1,
	}}
	mux.HandleFunc("/repos/check/info/check-runs", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, `{"id": 555}`)
	})

	mux.HandleFunc("/repos/check/info/check-runs/555", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, `{"id": 555}`)
	})

	event := &info.Event{
		Organization: "check",
		Repository:   "info",
		SHA:          "createCheckRunSHA",
	}

	for i := range statusOptionData {
		err := cnx.getOrUpdateCheckRunStatus(ctx, event, statusOptionData[i])
		assert.NilError(t, err)
	}
}

func TestGetExistingCheckRunIDFromMultiple(t *testing.T) {
	ctx, _ := rtesting.SetupFakeContext(t)
	client, mux, _, teardown := ghtesthelper.SetupGH()
	defer teardown()

	cnx := &Provider{
		ghClient:      client,
		PaginedNumber: 1,
	}
	event := &info.Event{
		Organization: "owner",
		Repository:   "repository",
		SHA:          "sha",
	}

	chosenOne := "chosenOne"
	chosenID := int64(55555)
	url := fmt.Sprintf("/repos/%v/%v/commits/%v/check-runs", event.Organization, event.Repository, event.SHA)
	mux.HandleFunc(url, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("page") == "" || r.URL.Query().Get("page") == "1" {
			w.Header().Add("Link", `<https://api.github.com`+url+`?page=2&per_page=1>; rel="next"`)
			fmt.Fprint(w, `{}`)
		} else {
			_, _ = fmt.Fprintf(w, `{
			"total_count": 2,
			"check_runs": [
				{
					"id": %v,
					"external_id": "%s"
				},
				{
					"id": 123456,
					"external_id": "notworthy"
				}
			]
		}`, chosenID, chosenOne)
		}
	})

	id, err := cnx.getExistingCheckRunID(ctx, event, provider.StatusOpts{
		PipelineRunName: chosenOne,
	})
	assert.NilError(t, err)
	assert.Assert(t, id != nil)
	assert.Equal(t, *id, chosenID)
}

func TestGetExistingPendingApprovalCheckRunID(t *testing.T) {
	ctx, _ := rtesting.SetupFakeContext(t)
	client, mux, _, teardown := ghtesthelper.SetupGH()
	defer teardown()

	cnx := New()
	cnx.SetGithubClient(client)

	event := &info.Event{
		Organization: "owner",
		Repository:   "repository",
		SHA:          "sha",
	}

	chosenOne := "chosenOne"
	chosenID := int64(55555)
	mux.HandleFunc(fmt.Sprintf("/repos/%v/%v/commits/%v/check-runs", event.Organization, event.Repository, event.SHA), func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprintf(w, `{
			"total_count": 1,
			"check_runs": [
				{
					"id": %v,
					"external_id": "%s",
					"output": {
						"title": "%s",
						"summary": "My CI is waiting for approval"
					}
				}
			]
		}`, chosenID, chosenOne, pendingApproval)
	})

	id, err := cnx.getExistingCheckRunID(ctx, event, provider.StatusOpts{
		PipelineRunName: chosenOne,
	})
	assert.NilError(t, err)
	assert.Equal(t, *id, chosenID)
}

func TestGetExistingFailedCheckRunID(t *testing.T) {
	ctx, _ := rtesting.SetupFakeContext(t)
	client, mux, _, teardown := ghtesthelper.SetupGH()
	defer teardown()

	cnx := New()
	cnx.SetGithubClient(client)

	event := &info.Event{
		Organization: "owner",
		Repository:   "repository",
		SHA:          "sha",
	}

	chosenOne := "chosenOne"
	chosenID := int64(55555)
	mux.HandleFunc(fmt.Sprintf("/repos/%v/%v/commits/%v/check-runs", event.Organization, event.Repository, event.SHA), func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprintf(w, `{
			"total_count": 1,
			"check_runs": [
				{
					"id": %v,
					"external_id": "%s",
					"output": {
						"title": "Failed",
						"summary": "CI is failed to run"
					}
				}
			]
		}`, chosenID, chosenOne)
	})

	id, err := cnx.getExistingCheckRunID(ctx, event, provider.StatusOpts{
		PipelineRunName: chosenOne,
	})
	assert.NilError(t, err)
	assert.Equal(t, *id, chosenID)
}

func TestGithubProviderCreateStatus(t *testing.T) {
	checkrunid := int64(2026)
	resultid := int64(666)
	runEvent := info.NewEvent()
	prname := "pr1"
	pr := &tektonv1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name: prname,
			Annotations: map[string]string{
				keys.CheckRunID: strconv.Itoa(int(checkrunid)),
			},
		},
	}
	runEvent.Organization = "check"
	runEvent.Repository = "run"

	type args struct {
		runevent           *info.Event
		status             string
		conclusion         string
		text               string
		detailsURL         string
		titleSubstr        string
		nilCompletedAtDate bool
		githubApps         bool
		accessDenied       bool
		isBot              bool
	}
	tests := []struct {
		name                 string
		args                 args
		pr                   *tektonv1.PipelineRun
		want                 *github.CheckRun
		wantErr              bool
		notoken              bool
		addExistingCheckruns bool
	}{
		{
			name: "success",
			args: args{
				runevent:    runEvent,
				status:      "completed",
				conclusion:  "success",
				text:        "Yay",
				detailsURL:  "https://cireport.com",
				titleSubstr: "Success",
				githubApps:  true,
			},
			want:    &github.CheckRun{ID: &resultid},
			wantErr: false,
		},
		{
			name: "success with using existing pending approval run checkrun",
			args: args{
				runevent:    runEvent,
				status:      "completed",
				conclusion:  "success",
				text:        "Yay",
				detailsURL:  "https://cireport.com",
				titleSubstr: "Success",
				githubApps:  true,
			},
			pr: &tektonv1.PipelineRun{
				ObjectMeta: metav1.ObjectMeta{
					Name: prname,
				},
			},
			addExistingCheckruns: true,
			want:                 &github.CheckRun{ID: &resultid},
			wantErr:              false,
		},
		{
			name: "success coming from webhook",
			args: args{
				runevent:    runEvent,
				status:      "completed",
				conclusion:  "success",
				text:        "Yay",
				detailsURL:  "https://cireport.com",
				titleSubstr: "Success",
				githubApps:  false,
			},
			want:    &github.CheckRun{ID: &resultid},
			wantErr: false,
		},
		{
			name: "in_progress",
			args: args{
				runevent:           runEvent,
				status:             "in_progress",
				conclusion:         "",
				text:               "Yay",
				detailsURL:         "https://cireport.com",
				nilCompletedAtDate: true,
				githubApps:         true,
			},
			want:    &github.CheckRun{ID: &resultid},
			wantErr: false,
		},
		{
			name: "failure",
			args: args{
				runevent:    runEvent,
				status:      "completed",
				conclusion:  "failure",
				text:        "Nay",
				detailsURL:  "https://cireport.com",
				titleSubstr: "Failed",
				githubApps:  true,
			},
			want:    &github.CheckRun{ID: &resultid},
			wantErr: false,
		},
		{
			name: "validation failure",
			args: args{
				runevent:    runEvent,
				status:      "completed",
				conclusion:  "failure",
				text:        "There was an error creating the PipelineRun: ```admission webhook \"validation.webhook.pipeline.tekton.dev\" denied the request: validation failed: invalid value: 0s```",
				detailsURL:  "https://cireport.com",
				titleSubstr: "Failed",
				githubApps:  true,
			},
			want:    &github.CheckRun{ID: &resultid},
			wantErr: false,
		},
		{
			name: "failure from bot",
			args: args{
				runevent:     runEvent,
				status:       "completed",
				conclusion:   "failure",
				text:         "Nay",
				detailsURL:   "https://cireport.com",
				titleSubstr:  "Failed",
				githubApps:   true,
				accessDenied: true,
				isBot:        true,
			},
			wantErr: false,
		},
		{
			name: "success from bot",
			args: args{
				runevent:    runEvent,
				status:      "completed",
				conclusion:  "failure",
				text:        "Nay",
				detailsURL:  "https://cireport.com",
				titleSubstr: "Failed",
				githubApps:  true,
				isBot:       true,
			},
			wantErr: false,
			want:    &github.CheckRun{ID: &resultid},
		},
		{
			name: "skipped",
			args: args{
				runevent:    runEvent,
				status:      "queued",
				conclusion:  "pending",
				text:        "Skipit",
				detailsURL:  "https://cireport.com",
				titleSubstr: "Pending",
				githubApps:  true,
			},
			want:    &github.CheckRun{ID: &resultid},
			wantErr: false,
		},
		{
			name: "unknown",
			args: args{
				runevent:    runEvent,
				status:      "completed",
				conclusion:  "neutral",
				text:        "Je says pas ce qui se passe wesh",
				detailsURL:  "https://cireport.com",
				titleSubstr: "Unknown",
				githubApps:  true,
			},
			want:    &github.CheckRun{ID: &resultid},
			wantErr: false,
		},
		{
			name:    "no token set",
			wantErr: true,
			notoken: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeclient, mux, _, teardown := ghtesthelper.SetupGH()
			defer teardown()

			ctx, _ := rtesting.SetupFakeContext(t)
			gcvs := New()
			gcvs.SetGithubClient(fakeclient)
			gcvs.Logger, _ = logger.GetLogger()
			gcvs.Run = params.New()
			if tt.args.isBot {
				gcvs.userType = "Bot"
			}

			checkRunCreated := false
			mux.HandleFunc("/repos/check/run/statuses/sha", func(_ http.ResponseWriter, _ *http.Request) {})
			mux.HandleFunc(fmt.Sprintf("/repos/check/run/check-runs/%d", checkrunid), func(rw http.ResponseWriter, r *http.Request) {
				bit, _ := io.ReadAll(r.Body)
				checkRun := &github.CheckRun{}
				err := json.Unmarshal(bit, checkRun)
				assert.NilError(t, err)
				checkRunCreated = true
				if tt.args.nilCompletedAtDate {
					// I guess that's the way you check for an undefined year,
					// or maybe i don't understand fully how go works😅
					assert.Assert(t, checkRun.GetCompletedAt().Year() == 0o001)
				}
				assert.Equal(t, checkRun.GetStatus(), tt.args.status)
				// pending status is not provided by GitHub its something added to handle skipped part from PAC side
				if tt.args.conclusion != "pending" {
					assert.Equal(t, checkRun.GetConclusion(), tt.args.conclusion)
				}
				assert.Equal(t, checkRun.Output.GetText(), tt.args.text)
				assert.Equal(t, checkRun.GetDetailsURL(), tt.args.detailsURL)
				assert.Assert(t, strings.Contains(checkRun.Output.GetTitle(), tt.args.titleSubstr))
				_, err = fmt.Fprintf(rw, `{"id": %d}`, resultid)
				assert.NilError(t, err)
			})

			if tt.addExistingCheckruns {
				tt.args.runevent.SHA = "sha"
				mux.HandleFunc(fmt.Sprintf("/repos/%v/%v/commits/%v/check-runs", tt.args.runevent.Organization, tt.args.runevent.Repository, tt.args.runevent.SHA), func(w http.ResponseWriter, _ *http.Request) {
					_, _ = fmt.Fprintf(w, `{
						"total_count": 1,
						"check_runs": [
							{
								"id": %v,
								"external_id": "%v",
                                "status": "queued",
                                "conclusion": "pending", 
								"output": {
									"title": "Pending approval, waiting for an /ok-to-test",
									"summary": "My CI is waiting for approval"
								}
							}
						]
					}`, checkrunid, resultid)
				})
			}

			status := provider.StatusOpts{
				PipelineRunName: prname,
				PipelineRun:     pr,
				Status:          tt.args.status,
				Conclusion:      tt.args.conclusion,
				Text:            tt.args.text,
				DetailsURL:      tt.args.detailsURL,
				AccessDenied:    tt.args.accessDenied,
			}
			if tt.pr != nil {
				status.PipelineRun = tt.pr
			}
			if tt.notoken {
				tt.args.runevent = info.NewEvent()
			} else {
				tt.args.runevent.Provider = &info.Provider{
					Token: "hello",
					URL:   "moto",
				}
				if tt.args.githubApps {
					tt.args.runevent.InstallationID = 12345
				} else {
					tt.args.runevent.SHA = "sha"
				}
			}

			testData := testclient.Data{}
			if tt.pr != nil {
				testData = testclient.Data{
					PipelineRuns: []*tektonv1.PipelineRun{tt.pr},
				}
			}
			stdata, _ := testclient.SeedTestData(t, ctx, testData)
			fakeClients := clients.Clients{
				Tekton: stdata.Pipeline,
			}
			gcvs.Run.Clients = fakeClients
			gcvs.SetPacInfo(&info.PacOpts{
				Settings: settings.Settings{
					ApplicationName: settings.PACApplicationNameDefaultValue,
				},
			})
			err := gcvs.CreateStatus(ctx, tt.args.runevent, status)
			if (err != nil) != tt.wantErr {
				t.Errorf("GithubProvider.CreateStatus() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.want == nil && checkRunCreated {
				t.Errorf("Check run should have not be created for this test")
				return
			}
			if tt.want != nil && !checkRunCreated {
				t.Errorf("Check run should have been created for this test")
				return
			}
		})
	}
}

func TestGithubProvidercreateStatusCommit(t *testing.T) {
	issuenumber := 666
	anevent := &info.Event{
		Event:             &github.PullRequestEvent{PullRequest: &github.PullRequest{Number: github.Ptr(issuenumber)}},
		Organization:      "owner",
		Repository:        "repository",
		SHA:               "createStatusCommitSHA",
		EventType:         "pull_request",
		PullRequestNumber: issuenumber,
	}
	tests := []struct {
		name               string
		event              *info.Event
		wantErr            bool
		status             provider.StatusOpts
		expectedConclusion string
	}{
		{
			name:  "completed",
			event: anevent,
			status: provider.StatusOpts{
				Status:     "completed",
				Summary:    "I just wanna say",
				Text:       "Finito amigo",
				Conclusion: "completed",
			},
			expectedConclusion: "completed",
		},
		{
			name:  "in_progress",
			event: anevent,
			status: provider.StatusOpts{
				Status: "in_progress",
			},
			expectedConclusion: "pending",
		},
		{
			name:  "pull_request status pending",
			event: anevent,
			status: provider.StatusOpts{
				Conclusion: "pending",
			},
			expectedConclusion: "pending",
		},
		{
			name:  "pull_request status neutral",
			event: anevent,
			status: provider.StatusOpts{
				Conclusion: "neutral",
			},
			expectedConclusion: "success",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeclient, mux, _, teardown := ghtesthelper.SetupGH()
			defer teardown()
			mux.HandleFunc(fmt.Sprintf("/repos/%s/%s/statuses/%s",
				tt.event.Organization, tt.event.Repository, tt.event.SHA), func(_ http.ResponseWriter, r *http.Request) {
				body, _ := io.ReadAll(r.Body)
				assert.Check(t, strings.Contains(string(body), fmt.Sprintf(`"state":"%s"`, tt.expectedConclusion)))
			})
			if tt.status.Status == "completed" {
				mux.HandleFunc(fmt.Sprintf("/repos/%s/%s/issues/%d/comments",
					tt.event.Organization, tt.event.Repository, issuenumber), func(_ http.ResponseWriter, r *http.Request) {
					body, _ := io.ReadAll(r.Body)
					assert.Equal(t, fmt.Sprintf(`{"body":"%s<br>%s"}`, tt.status.Summary, tt.status.Text)+"\n", string(body))
				})
			}

			ctx, _ := rtesting.SetupFakeContext(t)
			provider := &Provider{
				ghClient: fakeclient,
				Run:      params.New(),
				pacInfo: &info.PacOpts{
					Settings: settings.Settings{
						ApplicationName: settings.PACApplicationNameDefaultValue,
					},
				},
			}

			if err := provider.createStatusCommit(ctx, tt.event, tt.status); (err != nil) != tt.wantErr {
				t.Errorf("GetCommitInfo() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestProviderGetExistingCheckRunID(t *testing.T) {
	idd := int64(55555)
	tests := []struct {
		name       string
		jsonret    string
		expectedID *int64
		wantErr    bool
		prname     string
	}{
		{
			name: "has check runs",
			jsonret: `{
			"total_count": 1,
			"check_runs": [
				{
					"id": 55555,
					"external_id": "blahpr"
				}
			]
		}`,
			expectedID: &idd,
			prname:     "blahpr",
		},
		{
			name:       "no check runs",
			jsonret:    `{"total_count": 0,"check_runs": []}`,
			expectedID: nil,
		},
		{
			name:       "error it",
			jsonret:    `BLAHALALACLCALWA`,
			expectedID: nil,
			wantErr:    true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, _ := rtesting.SetupFakeContext(t)
			client, mux, _, teardown := ghtesthelper.SetupGH()
			defer teardown()
			event := &info.Event{
				Organization: "owner",
				Repository:   "repository",
				SHA:          "sha",
			}
			v := &Provider{
				ghClient: client,
			}
			mux.HandleFunc(fmt.Sprintf("/repos/%v/%v/commits/%v/check-runs", event.Organization, event.Repository, event.SHA), func(w http.ResponseWriter, _ *http.Request) {
				_, _ = fmt.Fprintf(w, "%s", tt.jsonret)
			})

			got, err := v.getExistingCheckRunID(ctx, event, provider.StatusOpts{
				PipelineRunName: tt.prname,
			})
			if (err != nil) != tt.wantErr {
				t.Errorf("getExistingCheckRunID() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.expectedID) {
				t.Errorf("getExistingCheckRunID() got = %v, want %v", got, tt.expectedID)
			}
		})
	}
}

func TestFormatPipelineComment(t *testing.T) {
	tests := []struct {
		name       string
		sha        string
		marker     string
		statusOpts provider.StatusOpts
		wantEmoji  string
		wantTitle  string
	}{
		{
			name:   "queued status",
			sha:    "abc123",
			marker: "<!-- pac-status-test -->",
			statusOpts: provider.StatusOpts{
				Status:                  "queued",
				OriginalPipelineRunName: "test-pipeline",
				Summary:                 "Pipeline queued",
				Text:                    "Waiting to start",
			},
			wantEmoji: "⏳",
			wantTitle: "Queued",
		},
		{
			name:   "running status",
			sha:    "abc123",
			marker: "<!-- pac-status-test -->",
			statusOpts: provider.StatusOpts{
				Status:                  "in_progress",
				OriginalPipelineRunName: "test-pipeline",
				Summary:                 "Pipeline running",
				Text:                    "In progress",
			},
			wantEmoji: "🚀",
			wantTitle: "Running",
		},
		{
			name:   "success status",
			sha:    "abc123",
			marker: "<!-- pac-status-test -->",
			statusOpts: provider.StatusOpts{
				Status:                  "completed",
				Conclusion:              "success",
				OriginalPipelineRunName: "test-pipeline",
				Summary:                 "Pipeline succeeded",
				Text:                    "All tasks passed",
			},
			wantEmoji: "✅",
			wantTitle: "Success",
		},
		{
			name:   "failure status",
			sha:    "abc123",
			marker: "<!-- pac-status-test -->",
			statusOpts: provider.StatusOpts{
				Status:                  "completed",
				Conclusion:              "failure",
				OriginalPipelineRunName: "test-pipeline",
				Summary:                 "Pipeline failed",
				Text:                    "Some tasks failed",
			},
			wantEmoji: "❌",
			wantTitle: "Failed",
		},
		{
			name:   "cancelled status",
			sha:    "abc123",
			marker: "<!-- pac-status-test -->",
			statusOpts: provider.StatusOpts{
				Status:                  "completed",
				Conclusion:              "cancelled",
				OriginalPipelineRunName: "test-pipeline",
				Summary:                 "Pipeline cancelled",
				Text:                    "Cancelled by user",
			},
			wantEmoji: "⚠️",
			wantTitle: "Cancelled",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ghProvider := &Provider{}
			result := ghProvider.formatPipelineComment(tt.sha, tt.marker, tt.statusOpts)

			assert.Assert(t, strings.Contains(result, tt.marker), "Result should contain marker")
			assert.Assert(t, strings.Contains(result, tt.wantEmoji), "Result should contain expected emoji")
			assert.Assert(t, strings.Contains(result, tt.wantTitle), "Result should contain expected title")
			assert.Assert(t, strings.Contains(result, tt.statusOpts.OriginalPipelineRunName), "Result should contain pipeline name")
			assert.Assert(t, strings.Contains(result, tt.sha), "Result should contain SHA")
		})
	}
}

func TestCreateOrUpdatePipelineComment(t *testing.T) {
	const (
		testOrg      = "test-org"
		testRepo     = "test-repo"
		testPRNum    = 42
		testSHA      = "abc123def456"
		pipelineName = "my-pipeline"
	)

	tests := []struct {
		name               string
		status             provider.StatusOpts
		event              *info.Event
		pr                 *tektonv1.PipelineRun
		mockResponses      map[string]func(rw http.ResponseWriter, r *http.Request)
		wantErr            bool
		wantErrContains    string
		wantCommentCreated bool
		wantCommentUpdated bool
		wantCacheUpdated   bool
		skipGitHubClient   bool
		cachedCommentID    int64
		returnedCommentID  int64
	}{
		{
			name: "skip when OriginalPipelineRunName is empty",
			status: provider.StatusOpts{
				OriginalPipelineRunName: "",
			},
			event: &info.Event{
				Organization:      testOrg,
				Repository:        testRepo,
				PullRequestNumber: testPRNum,
				SHA:               testSHA,
			},
			wantErr:            false,
			skipGitHubClient:   true,
			wantCommentCreated: false,
		},
		{
			name: "create new comment when no cache exists",
			status: provider.StatusOpts{
				OriginalPipelineRunName: pipelineName,
				Status:                  "in_progress",
				Summary:                 "Pipeline is running",
				Text:                    "Details here",
				PipelineRun: &tektonv1.PipelineRun{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pr-run-123",
						Namespace: "test-ns",
					},
				},
			},
			event: &info.Event{
				Organization:      testOrg,
				Repository:        testRepo,
				PullRequestNumber: testPRNum,
				SHA:               testSHA,
			},
			pr: &tektonv1.PipelineRun{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pr-run-123",
					Namespace: "test-ns",
				},
			},
			mockResponses: map[string]func(rw http.ResponseWriter, r *http.Request){
				fmt.Sprintf("/repos/%s/%s/issues/%d/comments", testOrg, testRepo, testPRNum): func(rw http.ResponseWriter, r *http.Request) {
					if r.Method == http.MethodGet {
						// No existing comments with marker
						fmt.Fprint(rw, `[]`)
						return
					}
					if r.Method == http.MethodPost {
						rw.WriteHeader(http.StatusCreated)
						fmt.Fprint(rw, `{"id": 12345}`)
						return
					}
				},
			},
			wantErr:            false,
			wantCommentCreated: true,
			returnedCommentID:  12345,
			wantCacheUpdated:   true,
		},
		{
			name: "update existing comment using cached comment ID",
			status: provider.StatusOpts{
				OriginalPipelineRunName: pipelineName,
				Status:                  "completed",
				Conclusion:              "success",
				Summary:                 "Pipeline completed successfully",
				Text:                    "All steps passed",
				PipelineRun: &tektonv1.PipelineRun{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pr-run-456",
						Namespace: "test-ns",
						Annotations: map[string]string{
							fmt.Sprintf("pipelinesascode.tekton.dev/status-comment-id-%s", pipelineName): "99999",
						},
					},
				},
			},
			event: &info.Event{
				Organization:      testOrg,
				Repository:        testRepo,
				PullRequestNumber: testPRNum,
				SHA:               testSHA,
			},
			pr: &tektonv1.PipelineRun{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pr-run-456",
					Namespace: "test-ns",
					Annotations: map[string]string{
						fmt.Sprintf("pipelinesascode.tekton.dev/status-comment-id-%s", pipelineName): "99999",
					},
				},
			},
			mockResponses: map[string]func(rw http.ResponseWriter, r *http.Request){
				fmt.Sprintf("/repos/%s/%s/issues/comments/99999", testOrg, testRepo): func(rw http.ResponseWriter, r *http.Request) {
					if r.Method == http.MethodPatch {
						rw.WriteHeader(http.StatusOK)
						fmt.Fprint(rw, `{"id": 99999}`)
						return
					}
				},
			},
			cachedCommentID:    99999,
			wantErr:            false,
			wantCommentUpdated: true,
			returnedCommentID:  99999,
			wantCacheUpdated:   true,
		},
		{
			name: "fallback to marker search when cached comment not found",
			status: provider.StatusOpts{
				OriginalPipelineRunName: pipelineName,
				Status:                  "completed",
				Conclusion:              "failure",
				Summary:                 "Pipeline failed",
				Text:                    "Step X failed",
				PipelineRun: &tektonv1.PipelineRun{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pr-run-789",
						Namespace: "test-ns",
						Annotations: map[string]string{
							fmt.Sprintf("pipelinesascode.tekton.dev/status-comment-id-%s", pipelineName): "88888",
						},
					},
				},
			},
			event: &info.Event{
				Organization:      testOrg,
				Repository:        testRepo,
				PullRequestNumber: testPRNum,
				SHA:               testSHA,
			},
			pr: &tektonv1.PipelineRun{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pr-run-789",
					Namespace: "test-ns",
					Annotations: map[string]string{
						fmt.Sprintf("pipelinesascode.tekton.dev/status-comment-id-%s", pipelineName): "88888",
					},
				},
			},
			mockResponses: map[string]func(rw http.ResponseWriter, r *http.Request){
				fmt.Sprintf("/repos/%s/%s/issues/comments/88888", testOrg, testRepo): func(rw http.ResponseWriter, _ *http.Request) {
					// Cached comment was deleted, return 404
					rw.WriteHeader(http.StatusNotFound)
					fmt.Fprint(rw, `{"message": "Not Found"}`)
				},
				fmt.Sprintf("/repos/%s/%s/issues/%d/comments", testOrg, testRepo, testPRNum): func(rw http.ResponseWriter, r *http.Request) {
					if r.Method == http.MethodGet {
						// Return existing comment with marker
						marker := fmt.Sprintf("<!-- pac-status-%s -->", pipelineName)
						fmt.Fprintf(rw, `[{"id": 77777, "body": "%s old content"}]`, marker)
						return
					}
				},
				fmt.Sprintf("/repos/%s/%s/issues/comments/77777", testOrg, testRepo): func(rw http.ResponseWriter, r *http.Request) {
					if r.Method == http.MethodPatch {
						rw.WriteHeader(http.StatusOK)
						fmt.Fprint(rw, `{"id": 77777}`)
						return
					}
				},
			},
			cachedCommentID:    88888,
			wantErr:            false,
			wantCommentUpdated: true,
			returnedCommentID:  77777,
			wantCacheUpdated:   true,
		},
		{
			name: "error when GitHub API fails",
			status: provider.StatusOpts{
				OriginalPipelineRunName: pipelineName,
				Status:                  "in_progress",
				Summary:                 "Pipeline is running",
				Text:                    "Details here",
				PipelineRun: &tektonv1.PipelineRun{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pr-run-error",
						Namespace: "test-ns",
					},
				},
			},
			event: &info.Event{
				Organization:      testOrg,
				Repository:        testRepo,
				PullRequestNumber: testPRNum,
				SHA:               testSHA,
			},
			pr: &tektonv1.PipelineRun{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pr-run-error",
					Namespace: "test-ns",
				},
			},
			mockResponses: map[string]func(rw http.ResponseWriter, r *http.Request){
				fmt.Sprintf("/repos/%s/%s/issues/%d/comments", testOrg, testRepo, testPRNum): func(rw http.ResponseWriter, r *http.Request) {
					if r.Method == http.MethodGet {
						fmt.Fprint(rw, `[]`)
						return
					}
					if r.Method == http.MethodPost {
						rw.WriteHeader(http.StatusInternalServerError)
						fmt.Fprint(rw, `{"message": "Internal Server Error"}`)
						return
					}
				},
			},
			wantErr:         true,
			wantErrContains: "500",
		},
		{
			name: "nil client error",
			status: provider.StatusOpts{
				OriginalPipelineRunName: pipelineName,
				Status:                  "in_progress",
			},
			event: &info.Event{
				Organization:      testOrg,
				Repository:        testRepo,
				PullRequestNumber: testPRNum,
				SHA:               testSHA,
			},
			skipGitHubClient: true,
			wantErr:          true,
			wantErrContains:  "no github client has been initialized",
		},
		{
			name: "not a pull request error",
			status: provider.StatusOpts{
				OriginalPipelineRunName: pipelineName,
				Status:                  "in_progress",
			},
			event: &info.Event{
				Organization:      testOrg,
				Repository:        testRepo,
				PullRequestNumber: 0, // Not a PR
				SHA:               testSHA,
			},
			wantErr:         true,
			wantErrContains: "create comment only works on pull requests",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, _ := rtesting.SetupFakeContext(t)
			testLogger, _ := logger.GetLogger()

			var ghProvider *Provider
			var teardown func()

			if !tt.skipGitHubClient {
				fakeclient, mux, _, td := ghtesthelper.SetupGH()
				teardown = td

				for pattern, handler := range tt.mockResponses {
					mux.HandleFunc(pattern, handler)
				}

				ghProvider = &Provider{
					ghClient: fakeclient,
					Logger:   testLogger,
					Run:      params.New(),
				}

				// Set up Tekton client if we have a PipelineRun
				if tt.pr != nil {
					testData := testclient.Data{
						PipelineRuns: []*tektonv1.PipelineRun{tt.pr},
					}
					stdata, _ := testclient.SeedTestData(t, ctx, testData)
					ghProvider.Run.Clients = clients.Clients{
						Tekton: stdata.Pipeline,
					}
				}
			} else {
				ghProvider = &Provider{
					Logger: testLogger,
				}
			}

			if teardown != nil {
				defer teardown()
			}

			// Update status with actual PipelineRun from test data if available
			status := tt.status
			if tt.pr != nil && status.PipelineRun != nil {
				status.PipelineRun = tt.pr
			}

			err := ghProvider.createOrUpdatePipelineComment(ctx, tt.event, status)

			if tt.wantErr {
				assert.Assert(t, err != nil, "Expected error but got nil")
				if tt.wantErrContains != "" {
					assert.Assert(t, strings.Contains(err.Error(), tt.wantErrContains),
						"Expected error to contain %q, got %q", tt.wantErrContains, err.Error())
				}
				return
			}
			assert.NilError(t, err)

			// Verify cache was updated if expected
			if tt.wantCacheUpdated && tt.pr != nil && ghProvider.Run != nil && ghProvider.Run.Clients.Tekton != nil {
				// Fetch the updated PipelineRun and verify the annotation
				updatedPR, err := ghProvider.Run.Clients.Tekton.TektonV1().PipelineRuns(tt.pr.Namespace).Get(ctx, tt.pr.Name, metav1.GetOptions{})
				assert.NilError(t, err)
				commentIDKey := fmt.Sprintf("pipelinesascode.tekton.dev/status-comment-id-%s", pipelineName)
				cachedID, ok := updatedPR.Annotations[commentIDKey]
				assert.Assert(t, ok, "Expected comment ID to be cached in annotation %s", commentIDKey)
				assert.Assert(t, cachedID != "", "Expected cached comment ID to be non-empty")
			}
		})
	}
}
