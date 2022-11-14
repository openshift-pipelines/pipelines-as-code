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

	"github.com/google/go-github/v47/github"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/keys"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/settings"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider"
	testclient "github.com/openshift-pipelines/pipelines-as-code/pkg/test/clients"
	ghtesthelper "github.com/openshift-pipelines/pipelines-as-code/pkg/test/github"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	rtesting "knative.dev/pkg/reconciler/testing"
)

func TestGithubProviderCreateCheckRun(t *testing.T) {
	ctx, _ := rtesting.SetupFakeContext(t)
	fakeclient, mux, _, teardown := ghtesthelper.SetupGH()
	cnx := Provider{
		Client: fakeclient,
	}
	defer teardown()
	mux.HandleFunc("/repos/check/info/check-runs", func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `{"id": 555}`)
	})

	mux.HandleFunc("/repos/check/info/check-runs/555", func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `{"id": 555}`)
	})

	event := &info.Event{
		Organization: "check",
		Repository:   "info",
		SHA:          "createCheckRunSHA",
	}

	err := cnx.getOrUpdateCheckRunStatus(ctx, nil, event, &info.PacOpts{LogURL: "http://nowhere", Settings: &settings.Settings{}}, provider.StatusOpts{
		PipelineRunName: "pr1",
		Status:          "hello moto",
	})
	assert.NilError(t, err)
}

func TestGetExistingCheckRunIDFromMultiple(t *testing.T) {
	ctx, _ := rtesting.SetupFakeContext(t)
	client, mux, _, teardown := ghtesthelper.SetupGH()
	defer teardown()

	cnx := &Provider{
		Client: client,
	}
	event := &info.Event{
		Organization: "owner",
		Repository:   "repository",
		SHA:          "sha",
	}

	chosenOne := "chosenOne"
	chosenID := int64(55555)
	mux.HandleFunc(fmt.Sprintf("/repos/%v/%v/commits/%v/check-runs", event.Organization, event.Repository, event.SHA), func(w http.ResponseWriter, r *http.Request) {
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
	})

	id, err := cnx.getExistingCheckRunID(ctx, event, provider.StatusOpts{
		PipelineRunName: chosenOne,
	})
	assert.NilError(t, err)
	assert.Equal(t, *id, chosenID)
}

func TestGetExistingSkippedCheckRunID(t *testing.T) {
	ctx, _ := rtesting.SetupFakeContext(t)
	client, mux, _, teardown := ghtesthelper.SetupGH()
	defer teardown()

	cnx := New()
	cnx.Client = client

	event := &info.Event{
		Organization: "owner",
		Repository:   "repository",
		SHA:          "sha",
	}

	chosenOne := "chosenOne"
	chosenID := int64(55555)
	mux.HandleFunc(fmt.Sprintf("/repos/%v/%v/commits/%v/check-runs", event.Organization, event.Repository, event.SHA), func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprintf(w, `{
			"total_count": 1,
			"check_runs": [
				{
					"id": %v,
					"external_id": "%s",
					"output": {
						"title": "Skipped",
						"summary": "My CI is skipping this commit"
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
	pr := &v1beta1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name: prname,
			Labels: map[string]string{
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
	}
	tests := []struct {
		name                 string
		args                 args
		pr                   *v1beta1.PipelineRun
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
			name: "success with using existing skipped run checkrun",
			args: args{
				runevent:    runEvent,
				status:      "completed",
				conclusion:  "success",
				text:        "Yay",
				detailsURL:  "https://cireport.com",
				titleSubstr: "Success",
				githubApps:  true,
			},
			pr: &v1beta1.PipelineRun{
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
			name: "skipped",
			args: args{
				runevent:    runEvent,
				status:      "completed",
				conclusion:  "skipped",
				text:        "Skipit",
				detailsURL:  "https://cireport.com",
				titleSubstr: "Skipped",
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
				text:        "Je sais pas ce qui se passe wesh",
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
			gcvs.Client = fakeclient
			gcvs.Logger = getLogger()
			gcvs.Run = params.New()

			mux.HandleFunc("/repos/check/run/statuses/sha", func(rw http.ResponseWriter, r *http.Request) {})
			mux.HandleFunc(fmt.Sprintf("/repos/check/run/check-runs/%d", checkrunid), func(rw http.ResponseWriter, r *http.Request) {
				bit, _ := io.ReadAll(r.Body)
				checkRun := &github.CheckRun{}
				err := json.Unmarshal(bit, checkRun)
				assert.NilError(t, err)

				if tt.args.nilCompletedAtDate {
					// I guess that's the way you check for an undefined year,
					// or maybe i don't understand fully how go works😅
					assert.Assert(t, checkRun.GetCompletedAt().Year() == 0o001)
				}
				assert.Equal(t, checkRun.GetStatus(), tt.args.status)
				assert.Equal(t, checkRun.GetConclusion(), tt.args.conclusion)
				assert.Equal(t, checkRun.Output.GetText(), tt.args.text)
				assert.Equal(t, checkRun.GetDetailsURL(), tt.args.detailsURL)
				assert.Assert(t, strings.Contains(checkRun.Output.GetTitle(), tt.args.titleSubstr))
				_, err = fmt.Fprintf(rw, `{"id": %d}`, resultid)
				assert.NilError(t, err)
			})
			if tt.addExistingCheckruns {
				tt.args.runevent.SHA = "sha"
				mux.HandleFunc(fmt.Sprintf("/repos/%v/%v/commits/%v/check-runs", tt.args.runevent.Organization, tt.args.runevent.Repository, tt.args.runevent.SHA), func(w http.ResponseWriter, r *http.Request) {
					_, _ = fmt.Fprintf(w, `{
						"total_count": 1,
						"check_runs": [
							{
								"id": %v,
								"external_id": "%v",
								"output": {
									"title": "Skipped",
									"summary": "My CI is skipping this commit"
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
			}
			if tt.pr != nil {
				status.PipelineRun = tt.pr
			}
			pacopts := &info.PacOpts{
				LogURL:   "https://log",
				Settings: &settings.Settings{},
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
					PipelineRuns: []*v1beta1.PipelineRun{tt.pr},
				}
			}
			stdata, _ := testclient.SeedTestData(t, ctx, testData)
			err := gcvs.CreateStatus(ctx, stdata.Pipeline, tt.args.runevent, pacopts, status)
			if (err != nil) != tt.wantErr {
				t.Errorf("GithubProvider.CreateStatus() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func TestGithubProvidercreateStatusCommit(t *testing.T) {
	issuenumber := 666
	anevent := &info.Event{
		Event:             &github.PullRequestEvent{PullRequest: &github.PullRequest{Number: github.Int(issuenumber)}},
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
			name:  "pull_request status skipped",
			event: anevent,
			status: provider.StatusOpts{
				Conclusion: "skipped",
			},
			expectedConclusion: "success",
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
				tt.event.Organization, tt.event.Repository, tt.event.SHA), func(rw http.ResponseWriter, r *http.Request) {
				body, _ := io.ReadAll(r.Body)
				assert.Check(t, strings.Contains(string(body), fmt.Sprintf(`"state":"%s"`, tt.expectedConclusion)))
			})
			if tt.status.Status == "completed" {
				mux.HandleFunc(fmt.Sprintf("/repos/%s/%s/issues/%d/comments",
					tt.event.Organization, tt.event.Repository, issuenumber), func(rw http.ResponseWriter, r *http.Request) {
					body, _ := io.ReadAll(r.Body)
					assert.Equal(t, fmt.Sprintf(`{"body":"%s<br>%s"}`, tt.status.Summary, tt.status.Text)+"\n", string(body))
				})
			}

			ctx, _ := rtesting.SetupFakeContext(t)
			provider := &Provider{
				Client: fakeclient,
			}

			if err := provider.createStatusCommit(ctx, tt.event, &info.PacOpts{Settings: &settings.Settings{}}, tt.status); (err != nil) != tt.wantErr {
				t.Errorf("GetCommitInfo() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGetCheckName(t *testing.T) {
	type args struct {
		status  provider.StatusOpts
		pacopts *info.PacOpts
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "no application name",
			args: args{
				status: provider.StatusOpts{
					OriginalPipelineRunName: "HELLO",
				},
				pacopts: &info.PacOpts{Settings: &settings.Settings{ApplicationName: ""}},
			},
			want: "HELLO",
		},
		{
			name: "application and pipelinerun name",
			args: args{
				status: provider.StatusOpts{
					OriginalPipelineRunName: "MOTO",
				},
				pacopts: &info.PacOpts{Settings: &settings.Settings{ApplicationName: "HELLO"}},
			},
			want: "HELLO / MOTO",
		},
		{
			name: "application no pipelinerun name",
			args: args{
				status: provider.StatusOpts{
					OriginalPipelineRunName: "",
				},
				pacopts: &info.PacOpts{Settings: &settings.Settings{ApplicationName: "PAC"}},
			},
			want: "PAC",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getCheckName(tt.args.status, tt.args.pacopts); got != tt.want {
				t.Errorf("getCheckName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestProviderGetExistingCheckRunID(t *testing.T) {
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
			expectedID: github.Int64(55555),
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
				Client: client,
			}
			mux.HandleFunc(fmt.Sprintf("/repos/%v/%v/commits/%v/check-runs", event.Organization, event.Repository, event.SHA), func(w http.ResponseWriter, r *http.Request) {
				_, _ = fmt.Fprintf(w, tt.jsonret)
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
