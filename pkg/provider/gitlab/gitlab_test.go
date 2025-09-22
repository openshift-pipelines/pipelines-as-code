package gitlab

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/events"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/opscomments"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/clients"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/settings"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/triggertype"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider"
	thelp "github.com/openshift-pipelines/pipelines-as-code/pkg/provider/gitlab/test"
	testclient "github.com/openshift-pipelines/pipelines-as-code/pkg/test/clients"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/test/logger"

	gitlab "gitlab.com/gitlab-org/api/client-go"
	"go.uber.org/zap"
	zapobserver "go.uber.org/zap/zaptest/observer"
	"gotest.tools/v3/assert"
	rtesting "knative.dev/pkg/reconciler/testing"
)

func TestCreateStatus(t *testing.T) {
	type fields struct {
		targetProjectID int
	}
	type args struct {
		event      *info.Event
		statusOpts provider.StatusOpts
		postStr    string
	}
	tests := []struct {
		name       string
		fields     fields
		args       args
		wantErr    bool
		wantClient bool
	}{
		{
			name:    "no client has been set",
			wantErr: true,
		},
		{
			name:       "skip in progress",
			wantClient: true,
			wantErr:    false,
			args: args{
				statusOpts: provider.StatusOpts{
					Status: "in_progress",
				},
			},
		},
		{
			name:       "skipped conclusion",
			wantClient: true,
			wantErr:    false,
			args: args{
				statusOpts: provider.StatusOpts{
					Conclusion: "skipped",
				},
				event: &info.Event{
					TriggerTarget: "pull_request",
				},
				postStr: "has skipped",
			},
		},
		{
			name:       "neutral conclusion",
			wantClient: true,
			wantErr:    false,
			args: args{
				statusOpts: provider.StatusOpts{
					Conclusion: "neutral",
				},
				event: &info.Event{
					TriggerTarget: "pull_request",
				},
				postStr: "has stopped",
			},
		},
		{
			name:       "failure conclusion",
			wantClient: true,
			wantErr:    false,
			args: args{
				statusOpts: provider.StatusOpts{
					Conclusion: "failure",
				},
				event: &info.Event{
					TriggerTarget: "pull_request",
				},
				postStr: "has failed",
			},
		},
		{
			name:       "success conclusion",
			wantClient: true,
			wantErr:    false,
			args: args{
				statusOpts: provider.StatusOpts{
					Conclusion: "success",
				},
				event: &info.Event{
					TriggerTarget: "pull_request",
				},
				postStr: "has successfully",
			},
		},
		{
			name:       "pending conclusion",
			wantClient: true,
			wantErr:    false,
			args: args{
				statusOpts: provider.StatusOpts{
					Conclusion: "pending",
				},
				event: &info.Event{
					TriggerTarget: "pull_request",
				},
				postStr: "",
			},
		},
		{
			name:       "completed conclusion",
			wantClient: true,
			wantErr:    false,
			args: args{
				statusOpts: provider.StatusOpts{
					Conclusion: "completed",
				},
				event: &info.Event{
					TriggerTarget: "pull_request",
				},
				postStr: "has completed",
			},
		},
		{
			name:       "gitops comments completed",
			wantClient: true,
			wantErr:    false,
			args: args{
				statusOpts: provider.StatusOpts{
					Conclusion: "completed",
				},
				event: &info.Event{
					TriggerTarget: "Note",
				},
				postStr: "has completed",
			},
		},
		{
			name:       "completed with a details url",
			wantClient: true,
			wantErr:    false,
			args: args{
				statusOpts: provider.StatusOpts{
					Conclusion: "skipped",
					DetailsURL: "https://url.com",
				},
				event: &info.Event{
					TriggerTarget: "pull_request",
				},
				postStr: "https://url.com",
			},
		},
		{
			name:       "pending conclusion for gitops command on pushed commit",
			wantClient: true,
			wantErr:    false,
			args: args{
				statusOpts: provider.StatusOpts{
					Conclusion: "pending",
				},
				event: &info.Event{
					TriggerTarget: "push",
					EventType:     opscomments.TestAllCommentEventType.String(),
				},
				postStr: "",
			},
		},
		{
			name:       "completed conclusion for gitops command on pushed commit",
			wantClient: true,
			wantErr:    false,
			args: args{
				statusOpts: provider.StatusOpts{
					Conclusion: "completed",
				},
				event: &info.Event{
					TriggerTarget: "push",
					EventType:     opscomments.RetestAllCommentEventType.String(),
				},
				postStr: "has completed",
			},
		},
		{
			name:       "commit status success on source project",
			wantClient: true,
			wantErr:    false,
			fields: fields{
				targetProjectID: 100,
			},
			args: args{
				statusOpts: provider.StatusOpts{
					Conclusion: "success",
				},
				event: &info.Event{
					TriggerTarget:   "pull_request",
					SourceProjectID: 200,
					TargetProjectID: 100,
					SHA:             "abc123",
				},
				postStr: "has successfully",
			},
		},
		{
			name:       "commit status falls back to target project",
			wantClient: true,
			wantErr:    false,
			fields: fields{
				targetProjectID: 100,
			},
			args: args{
				statusOpts: provider.StatusOpts{
					Conclusion: "success",
				},
				event: &info.Event{
					TriggerTarget:   "pull_request",
					SourceProjectID: 404, // Will fail to find this project
					TargetProjectID: 100,
					SHA:             "abc123",
				},
				postStr: "has successfully",
			},
		},
		{
			name:       "commit status fails on both projects but continues",
			wantClient: true,
			wantErr:    false,
			fields: fields{
				targetProjectID: 100,
			},
			args: args{
				statusOpts: provider.StatusOpts{
					Conclusion: "success",
				},
				event: &info.Event{
					TriggerTarget:   "pull_request",
					SourceProjectID: 404, // Will fail
					TargetProjectID: 405, // Will fail
					SHA:             "abc123",
				},
				postStr: "has successfully",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, _ := rtesting.SetupFakeContext(t)
			logger, _ := logger.GetLogger()
			stdata, _ := testclient.SeedTestData(t, ctx, testclient.Data{})
			run := &params.Run{
				Clients: clients.Clients{
					Kube: stdata.Kube,
					Log:  logger,
				},
			}
			v := &Provider{
				targetProjectID: tt.fields.targetProjectID,
				run:             params.New(),
				Logger:          logger,
				pacInfo: &info.PacOpts{
					Settings: settings.Settings{
						ApplicationName: settings.PACApplicationNameDefaultValue,
					},
				},
				eventEmitter: events.NewEventEmitter(run.Clients.Kube, logger),
			}
			if tt.args.event == nil {
				tt.args.event = info.NewEvent()
			}
			tt.args.event.PullRequestNumber = 666

			if tt.wantClient {
				client, mux, tearDown := thelp.Setup(t)
				v.SetGitLabClient(client)
				defer tearDown()

				// Mock commit status endpoints for both source and target projects
				if tt.args.event.SourceProjectID != 0 {
					// Mock source project commit status endpoint
					sourceStatusPath := fmt.Sprintf("/projects/%d/statuses/%s", tt.args.event.SourceProjectID, tt.args.event.SHA)
					mux.HandleFunc(sourceStatusPath, func(rw http.ResponseWriter, _ *http.Request) {
						if tt.args.event.SourceProjectID == 404 {
							// Simulate failure on source project
							rw.WriteHeader(http.StatusNotFound)
							fmt.Fprint(rw, `{"message": "404 Project Not Found"}`)
							return
						}
						// Success on source project
						rw.WriteHeader(http.StatusCreated)
						fmt.Fprint(rw, `{}`)
					})
				}

				if tt.args.event.TargetProjectID != 0 {
					// Mock target project commit status endpoint
					targetStatusPath := fmt.Sprintf("/projects/%d/statuses/%s", tt.args.event.TargetProjectID, tt.args.event.SHA)
					mux.HandleFunc(targetStatusPath, func(rw http.ResponseWriter, _ *http.Request) {
						if tt.args.event.TargetProjectID == 404 {
							// Simulate failure on target project
							rw.WriteHeader(http.StatusNotFound)
							fmt.Fprint(rw, `{"message": "404 Project Not Found"}`)
							return
						}
						// Success on target project
						rw.WriteHeader(http.StatusCreated)
						fmt.Fprint(rw, `{}`)
					})
				}

				thelp.MuxNotePost(t, mux, v.targetProjectID, tt.args.event.PullRequestNumber, tt.args.postStr)
			}

			if err := v.CreateStatus(ctx, tt.args.event, tt.args.statusOpts); (err != nil) != tt.wantErr {
				t.Errorf("CreateStatus() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGetCommitInfo(t *testing.T) {
	ctx, _ := rtesting.SetupFakeContext(t)
	client, _, tearDown := thelp.Setup(t)
	v := &Provider{gitlabClient: client}

	defer tearDown()
	assert.NilError(t, v.GetCommitInfo(ctx, info.NewEvent()))

	ncv := &Provider{}
	assert.Assert(t, ncv.GetCommitInfo(ctx, info.NewEvent()) != nil)
}

func TestGetConfig(t *testing.T) {
	v := &Provider{}
	assert.Assert(t, v.GetConfig().APIURL != "")
	assert.Assert(t, v.GetConfig().TaskStatusTMPL != "")
}

func TestSetClient(t *testing.T) {
	ctx, _ := rtesting.SetupFakeContext(t)
	core, observer := zapobserver.New(zap.InfoLevel)
	fakelogger := zap.New(core).Sugar()

	run := &params.Run{
		Clients: clients.Clients{
			Log: fakelogger,
		},
	}

	v := &Provider{}
	assert.Assert(t, v.SetClient(ctx, run, info.NewEvent(), nil, nil) != nil)

	client, _, tearDown := thelp.Setup(t)
	defer tearDown()

	vv := &Provider{gitlabClient: client}
	err := vv.SetClient(ctx, run, &info.Event{
		Provider: &info.Provider{
			Token: "hello",
		},
		Organization:    "my-org",
		Repository:      "my-repo",
		SourceProjectID: 1234,
		TargetProjectID: 1234,
	}, nil, nil)

	assert.NilError(t, err)
	assert.Assert(t, *vv.Token != "")

	logs := observer.TakeAll()
	assert.Assert(t, len(logs) > 0, "expected a log entry, got none")

	expected := fmt.Sprintf(
		"gitlab: initialized for client with token for apiURL=%s, org=%s, repo=%s",
		vv.apiURL, "my-org", "my-repo")

	assert.Equal(t, expected, logs[0].Message)
}

func TestSetClientRepositoryAccessCheck(t *testing.T) {
	ctx, _ := rtesting.SetupFakeContext(t)
	observer, _ := zapobserver.New(zap.InfoLevel)
	fakelogger := zap.New(observer).Sugar()
	run := &params.Run{
		Clients: clients.Clients{
			Log: fakelogger,
		},
	}

	tests := []struct {
		name              string
		triggerTarget     triggertype.Trigger
		sourceProjectID   int
		setupMockResponse func(*http.ServeMux, int)
		expectedError     string
	}{
		{
			name:            "Non-pull request trigger should skip access check",
			triggerTarget:   triggertype.Push,
			sourceProjectID: 123,
			setupMockResponse: func(_ *http.ServeMux, _ int) {
				// No mock needed - should not make the call
			},
			expectedError: "",
		},
		{
			name:            "Pull request with successful access",
			triggerTarget:   triggertype.PullRequest,
			sourceProjectID: 123,
			setupMockResponse: func(mux *http.ServeMux, projectID int) {
				path := fmt.Sprintf("/projects/%d", projectID)
				mux.HandleFunc(path, func(rw http.ResponseWriter, r *http.Request) {
					if r.Method == http.MethodGet {
						rw.WriteHeader(http.StatusOK)
						fmt.Fprint(rw, `{"id": 123, "name": "test-repo"}`)
					}
				})
			},
			expectedError: "",
		},
		{
			name:            "Pull request with not found should return specific error",
			triggerTarget:   triggertype.PullRequest,
			sourceProjectID: 456,
			setupMockResponse: func(mux *http.ServeMux, projectID int) {
				path := fmt.Sprintf("/projects/%d", projectID)
				mux.HandleFunc(path, func(rw http.ResponseWriter, r *http.Request) {
					if r.Method == http.MethodGet {
						// Return 404 without error to test the status code check
						rw.WriteHeader(http.StatusNotFound)
						fmt.Fprint(rw, `{"message": "404 Project Not Found"}`)
					}
				})
			},
			expectedError: "failed to access GitLab source repository ID 456: please ensure token has 'read_repository' scope on that repository",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient, mux, tearDown := thelp.Setup(t)
			defer tearDown()

			// Setup the mock for the repository access check API call
			if tt.setupMockResponse != nil {
				tt.setupMockResponse(mux, tt.sourceProjectID)
			}

			v := &Provider{gitlabClient: mockClient}
			event := &info.Event{
				Provider: &info.Provider{
					Token: "test-token",
				},
				Organization:    "test-org",
				Repository:      "test-repo",
				TriggerTarget:   tt.triggerTarget,
				SourceProjectID: tt.sourceProjectID,
				TargetProjectID: 123,
			}

			err := v.SetClient(ctx, run, event, nil, nil)

			if tt.expectedError != "" {
				assert.Assert(t, err != nil, "expected error but got none")
				assert.ErrorContains(t, err, tt.expectedError)
			} else {
				assert.NilError(t, err, "unexpected error: %v", err)
			}
		})
	}
}

func TestSetClientDetectAPIURL(t *testing.T) {
	ctx, _ := rtesting.SetupFakeContext(t)
	observer, _ := zapobserver.New(zap.InfoLevel)
	fakelogger := zap.New(observer).Sugar()
	run := &params.Run{
		Clients: clients.Clients{
			Log: fakelogger,
		},
	}
	mockClient, _, tearDown := thelp.Setup(t)
	defer tearDown()

	// Define test cases
	tests := []struct {
		name              string
		providerToken     string
		providerURL       string // input: event.Provider.URL
		repoURL           string // input: v.repoURL
		pathWithNamespace string // input: v.pathWithNamespace (needed if repoURL is used)
		eventURL          string // input: event.URL
		// Define expected outcomes
		expectedAPIURL string
		expectedError  string // Substring expected in the error message, "" for no error
	}{
		{
			name:          "Error: No token provided",
			providerToken: "",
			expectedError: "no git_provider.secret has been set",
		},
		{
			name:              "Success: API URL from event.Provider.URL (highest precedence)",
			providerToken:     "token",
			providerURL:       "https://provider.example.com",
			repoURL:           "https://repo.example.com/foo/bar", // Should be ignored
			pathWithNamespace: "foo/bar",
			eventURL:          "https://event.example.com/foo/bar", // Should be ignored
			expectedAPIURL:    "https://provider.example.com",
			expectedError:     "",
		},
		{
			name:              "Success: API URL from v.repoURL (non-public)",
			providerToken:     "token",
			providerURL:       "", // This must be empty to test the next case
			repoURL:           "https://private-gitlab.com/my/repo",
			pathWithNamespace: "my/repo",
			eventURL:          "https://event.example.com/my/repo", // Should be ignored
			expectedAPIURL:    "https://private-gitlab.com/",
			expectedError:     "",
		},
		{
			name:           "Success: API URL from event.URL",
			providerToken:  "token",
			providerURL:    "", // This must be empty
			repoURL:        "", // This must be empty
			eventURL:       "https://event-url.com/org/project",
			expectedAPIURL: "https://event-url.com",
			expectedError:  "",
		},
		{
			name:           "Success: Fallback to default public API URL",
			providerToken:  "token",
			providerURL:    "",
			repoURL:        "",
			eventURL:       "",
			expectedAPIURL: apiPublicURL, // Default case
			expectedError:  "",
		},
		{
			name:              "Success: Default URL when repoURL is public GitLab",
			providerToken:     "token",
			providerURL:       "",
			repoURL:           apiPublicURL + "/public/repo", // Starts with public URL, so skipped
			pathWithNamespace: "public/repo",
			eventURL:          "", // Falls through to default
			expectedAPIURL:    apiPublicURL,
			expectedError:     "",
		},
		{
			name:          "Error: Invalid URL from event.URL",
			providerToken: "token",
			providerURL:   "",
			repoURL:       "",
			eventURL:      "://bad-schema",
			expectedError: "parse \"://bad-schema\": missing protocol scheme", // Specific error from url.Parse
		},
		{
			name:          "Error: Invalid URL from event.Provider.URL (final parse)",
			providerToken: "token",
			providerURL:   "ht tp://invalid host", // Invalid URL format
			repoURL:       "",
			eventURL:      "",
			expectedError: "failed to parse api url", // Wrapper error message
		},
		{
			name:              "Error: Invalid URL from v.repoURL (final parse)",
			providerToken:     "token",
			providerURL:       "",
			repoURL:           "ht tp://invalid.repo.url/foo/bar", // Invalid format
			pathWithNamespace: "foo/bar",
			eventURL:          "",
			// Note: The calculated apiURL would be "ht tp://invalid.repo.url" before parsing
			expectedError: "failed to parse api url",
		},
	}

	// Run test cases
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup specific to this test case
			v := &Provider{
				gitlabClient:      mockClient, // Use the shared mock client
				repoURL:           tc.repoURL,
				pathWithNamespace: tc.pathWithNamespace,
			}
			event := info.NewEvent()
			event.Provider.Token = tc.providerToken
			event.Provider.URL = tc.providerURL
			event.URL = tc.eventURL
			// Set some default IDs to avoid potential nil pointer issues or side effects
			// if the GetProject part of SetClient is reached unexpectedly.
			event.TargetProjectID = 1
			event.SourceProjectID = 1

			// Execute the function under test
			// Using placeholder nil values for arguments not directly related to URL detection
			err := v.SetClient(ctx, run, event, nil, nil)

			// Assertions
			if tc.expectedError != "" {
				assert.ErrorContains(t, err, tc.expectedError)
				// If an error is expected, we usually don't check the apiURL state,
				// as it might be indeterminate or irrelevant.
			} else {
				assert.NilError(t, err)
				// Only check the resulting apiURL if no error was expected
				assert.Equal(t, tc.expectedAPIURL, v.apiURL)
				// Optionally, check if the client was actually set (if no error)
				assert.Assert(t, v.Client() != nil)
				assert.Assert(t, v.Token != nil && *v.Token == tc.providerToken)
			}
		})
	}
}

func TestGetTektonDir(t *testing.T) {
	samplePR, err := os.ReadFile("../../resolve/testdata/pipeline-finally.yaml")
	assert.NilError(t, err)
	type fields struct {
		targetProjectID int
		sourceProjectID int
		userID          int
	}
	type args struct {
		event      *info.Event
		path       string
		provenance string
	}
	tests := []struct {
		name                 string
		fields               fields
		args                 args
		wantStr              string
		wantErr              string
		wantTreeAPIErr       bool
		wantFilesAPIErr      bool
		wantClient           bool
		prcontent            string
		filterMessageSnippet string
	}{
		{
			name:    "no client set",
			wantErr: noClientErrStr,
		},
		{
			name:       "not found, no err",
			wantClient: true,
			args:       args{event: &info.Event{}},
		},
		{
			name:       "bad yaml",
			wantClient: true,
			args: args{
				event: &info.Event{SHA: "abcd", HeadBranch: "main"},
				path:  ".tekton",
			},
			fields: fields{
				sourceProjectID: 10,
			},
			prcontent: "bad:\n- yaml\nfoo",
			wantErr:   "error unmarshalling yaml file pr.yaml: yaml: line 4: could not find expected ':'",
		},
		{
			name:      "list tekton dir on pull request",
			prcontent: string(samplePR),
			args: args{
				path: ".tekton",
				event: &info.Event{
					HeadBranch:    "main",
					TriggerTarget: triggertype.PullRequest,
				},
			},
			fields: fields{
				sourceProjectID: 100,
			},
			wantClient:           true,
			wantStr:              "kind: PipelineRun",
			filterMessageSnippet: `Using PipelineRun definition from source merge request on commit SHA`,
		},
		{
			name:      "list tekton dir on push",
			prcontent: string(samplePR),
			args: args{
				path: ".tekton",
				event: &info.Event{
					HeadBranch:    "main",
					TriggerTarget: triggertype.Push,
				},
			},
			fields: fields{
				sourceProjectID: 100,
			},
			wantClient:           true,
			wantStr:              "kind: PipelineRun",
			filterMessageSnippet: `Using PipelineRun definition from source push on commit SHA`,
		},
		{
			name:      "list tekton dir on default_branch",
			prcontent: string(samplePR),
			args: args{
				provenance: "default_branch",
				path:       ".tekton",
				event: &info.Event{
					DefaultBranch: "main",
				},
			},
			fields: fields{
				sourceProjectID: 100,
			},
			wantClient: true,
			wantStr:    "kind: PipelineRun",
		},
		{
			name:      "list tekton dir no --- prefix",
			prcontent: strings.TrimPrefix(string(samplePR), "---"),
			args: args{
				path: ".tekton",
				event: &info.Event{
					HeadBranch: "main",
				},
			},
			fields: fields{
				sourceProjectID: 100,
			},
			wantClient: true,
			wantStr:    "kind: PipelineRun",
		},
		{
			name:      "list tekton dir tree api call error",
			prcontent: strings.TrimPrefix(string(samplePR), "---"),
			args: args{
				path: ".tekton",
				event: &info.Event{
					HeadBranch: "main",
				},
			},
			fields: fields{
				sourceProjectID: 100,
			},
			wantClient:     true,
			wantTreeAPIErr: true,
			wantErr:        "failed to list .tekton dir",
		},
		{
			name:      "get file raw api call error",
			prcontent: strings.TrimPrefix(string(samplePR), "---"),
			args: args{
				path: ".tekton",
				event: &info.Event{
					HeadBranch: "main",
				},
			},
			fields: fields{
				sourceProjectID: 100,
			},
			wantClient:      true,
			wantFilesAPIErr: true,
			wantErr:         "failed to get filename from api pr.yaml dir",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, _ := rtesting.SetupFakeContext(t)

			observer, exporter := zapobserver.New(zap.InfoLevel)
			fakelogger := zap.New(observer).Sugar()
			v := &Provider{
				targetProjectID: tt.fields.targetProjectID,
				sourceProjectID: tt.fields.sourceProjectID,
				userID:          tt.fields.userID,
				Logger:          fakelogger,
			}
			if tt.wantClient {
				client, mux, tearDown := thelp.Setup(t)
				v.SetGitLabClient(client)
				muxbranch := tt.args.event.HeadBranch
				if tt.args.provenance == "default_branch" {
					muxbranch = tt.args.event.DefaultBranch
				}
				if tt.args.path != "" && tt.prcontent != "" {
					thelp.MuxListTektonDir(t, mux, tt.fields.sourceProjectID, muxbranch, tt.prcontent, tt.wantTreeAPIErr, tt.wantFilesAPIErr)
				}
				defer tearDown()
			}

			got, err := v.GetTektonDir(ctx, tt.args.event, tt.args.path, tt.args.provenance)
			if tt.wantErr != "" {
				assert.Assert(t, err != nil, "expected error %s, got %v", tt.wantErr, err)
				assert.ErrorContains(t, err, tt.wantErr)
				return
			}
			if tt.wantStr != "" {
				assert.Assert(t, strings.Contains(got, tt.wantStr), "%s is not in %s", tt.wantStr, got)
			}
			if tt.filterMessageSnippet != "" {
				gotcha := exporter.FilterMessageSnippet(tt.filterMessageSnippet)
				assert.Assert(t, gotcha.Len() > 0, "expected to find %s in logs, found %v", tt.filterMessageSnippet, exporter.All())
			}
		})
	}
}

func TestGetFileInsideRepo(t *testing.T) {
	content := "hello moto"
	ctx, _ := rtesting.SetupFakeContext(t)
	client, mux, tearDown := thelp.Setup(t)
	defer tearDown()

	event := &info.Event{
		HeadBranch: "branch",
	}
	v := Provider{
		sourceProjectID: 10,
		gitlabClient:    client,
	}
	thelp.MuxListTektonDir(t, mux, v.sourceProjectID, event.HeadBranch, content, false, false)
	got, err := v.GetFileInsideRepo(ctx, event, "pr.yaml", "")
	assert.NilError(t, err)
	assert.Equal(t, content, got)

	_, err = v.GetFileInsideRepo(ctx, event, "notfound", "")
	assert.Assert(t, err != nil)
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name        string
		wantErr     bool
		secretToken string
		eventToken  string
	}{
		{
			name:        "valid event",
			wantErr:     false,
			secretToken: "test",
			eventToken:  "test",
		},
		{
			name:        "fail validation, no secret defined",
			wantErr:     true,
			secretToken: "",
			eventToken:  "test",
		},
		{
			name:        "fail validation",
			wantErr:     true,
			secretToken: "secret",
			eventToken:  "test",
		},
		{
			name:        "fail validation, missing event token",
			wantErr:     true,
			secretToken: "secret",
			eventToken:  "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := &Provider{}

			httpHeader := http.Header{}
			httpHeader.Set("X-Gitlab-Token", tt.eventToken)

			event := info.NewEvent()
			event.Request = &info.Request{
				Header: httpHeader,
			}
			event.Provider = &info.Provider{
				WebhookSecret: tt.secretToken,
			}

			if err := v.Validate(context.TODO(), nil, event); (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGetFiles(t *testing.T) {
	tests := []struct {
		name                             string
		event                            *info.Event
		mrchanges                        []*gitlab.MergeRequestDiff
		pushChanges                      []*gitlab.Diff
		wantAddedFilesCount              int
		wantDeletedFilesCount            int
		wantModifiedFilesCount           int
		wantRenamedFilesCount            int
		sourceProjectID, targetProjectID int
		wantError                        bool
	}{
		{
			name: "pull-request",
			event: &info.Event{
				TriggerTarget:     "pull_request",
				Organization:      "pullrequestowner",
				Repository:        "pullrequestrepository",
				PullRequestNumber: 10,
			},
			mrchanges: []*gitlab.MergeRequestDiff{
				{
					NewPath: "modified.yaml",
				},
				{
					NewPath: "added.doc",
					NewFile: true,
				},
				{
					NewPath:     "removed.yaml",
					DeletedFile: true,
				},
				{
					NewPath:     "renamed.doc",
					RenamedFile: true,
				},
			},
			wantAddedFilesCount:    1,
			wantDeletedFilesCount:  1,
			wantModifiedFilesCount: 1,
			wantRenamedFilesCount:  1,
			targetProjectID:        10,
		},
		{
			name: "pull-request with wrong project ID",
			event: &info.Event{
				TriggerTarget:     "pull_request",
				Organization:      "pullrequestowner",
				Repository:        "pullrequestrepository",
				PullRequestNumber: 10,
			},
			mrchanges: []*gitlab.MergeRequestDiff{
				{
					NewPath: "modified.yaml",
				},
				{
					NewPath: "added.doc",
					NewFile: true,
				},
				{
					NewPath:     "removed.yaml",
					DeletedFile: true,
				},
				{
					NewPath:     "renamed.doc",
					RenamedFile: true,
				},
			},
			wantAddedFilesCount:    0,
			wantDeletedFilesCount:  0,
			wantModifiedFilesCount: 0,
			wantRenamedFilesCount:  0,
			targetProjectID:        12,
			wantError:              true,
		},
		{
			name: "push",
			event: &info.Event{
				TriggerTarget: "push",
				Organization:  "pushrequestowner",
				Repository:    "pushrequestrepository",
				SHA:           "shacommitinfo",
			},
			pushChanges: []*gitlab.Diff{
				{
					NewPath: "modified.yaml",
				},
				{
					NewPath: "added.doc",
					NewFile: true,
				},
				{
					NewPath:     "removed.yaml",
					DeletedFile: true,
				},
				{
					NewPath:     "renamed.doc",
					RenamedFile: true,
				},
			},
			wantAddedFilesCount:    1,
			wantDeletedFilesCount:  1,
			wantModifiedFilesCount: 1,
			wantRenamedFilesCount:  1,
			sourceProjectID:        0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, _ := rtesting.SetupFakeContext(t)
			fakeclient, mux, teardown := thelp.Setup(t)
			defer teardown()
			mergeFileChanges := []*gitlab.MergeRequestDiff{
				{
					NewPath: "modified.yaml",
				},
				{
					NewPath: "added.doc",
					NewFile: true,
				},
				{
					NewPath:     "removed.yaml",
					DeletedFile: true,
				},
				{
					NewPath:     "renamed.doc",
					RenamedFile: true,
				},
			}
			if tt.event.TriggerTarget == "pull_request" {
				mux.HandleFunc(fmt.Sprintf("/projects/10/merge_requests/%d/diffs",
					tt.event.PullRequestNumber), func(rw http.ResponseWriter, _ *http.Request) {
					jeez, err := json.Marshal(mergeFileChanges)
					assert.NilError(t, err)
					_, _ = rw.Write(jeez)
				})
			}
			pushFileChanges := []*gitlab.Diff{
				{
					NewPath: "modified.yaml",
				},
				{
					NewPath: "added.doc",
					NewFile: true,
				},
				{
					NewPath:     "removed.yaml",
					DeletedFile: true,
				},
				{
					NewPath:     "renamed.doc",
					RenamedFile: true,
				},
			}
			if tt.event.TriggerTarget == "push" {
				mux.HandleFunc(fmt.Sprintf("/projects/0/repository/commits/%s/diff", tt.event.SHA),
					func(rw http.ResponseWriter, _ *http.Request) {
						jeez, err := json.Marshal(pushFileChanges)
						assert.NilError(t, err)
						_, _ = rw.Write(jeez)
					})
			}

			providerInfo := &Provider{gitlabClient: fakeclient, sourceProjectID: tt.sourceProjectID, targetProjectID: tt.targetProjectID}
			changedFiles, err := providerInfo.GetFiles(ctx, tt.event)
			if tt.wantError != true {
				assert.NilError(t, err, nil)
			}
			assert.Equal(t, tt.wantAddedFilesCount, len(changedFiles.Added))
			assert.Equal(t, tt.wantDeletedFilesCount, len(changedFiles.Deleted))
			assert.Equal(t, tt.wantModifiedFilesCount, len(changedFiles.Modified))
			assert.Equal(t, tt.wantRenamedFilesCount, len(changedFiles.Renamed))

			if tt.event.TriggerTarget == "pull_request" {
				for i := range changedFiles.All {
					assert.Equal(t, tt.mrchanges[i].NewPath, changedFiles.All[i])
				}
			}
			if tt.event.TriggerTarget == "push" {
				for i := range changedFiles.All {
					assert.Equal(t, tt.pushChanges[i].NewPath, changedFiles.All[i])
				}
			}
		})
	}
}

func TestIsHeadCommitOfBranch(t *testing.T) {
	tests := []struct {
		name          string
		event         *info.Event
		branchName    string
		wantClient    bool
		errStatusCode int
		ErrMsg        string
	}{
		{
			name:       "bad/client is not initialized",
			wantClient: false,
			ErrMsg:     "no gitlab client has been initialized",
		},
		{
			name:          "bad/user is not authorized",
			wantClient:    true,
			branchName:    "cool-branch",
			errStatusCode: http.StatusUnauthorized,
			ErrMsg:        "401",
		},
		{
			name:          "bad/branch doesn't exist",
			wantClient:    true,
			branchName:    "wrong-branch",
			errStatusCode: http.StatusNotFound,
			ErrMsg:        "404",
		},
		{
			name:       "bad/SHA is not HEAD of the branch",
			wantClient: true,
			event:      &info.Event{SHA: "IAmNotHEAD"},
			branchName: "cool-branch",
			ErrMsg:     "provided SHA IAmNotHEAD is not the HEAD commit of the branch cool-branch",
		},
		{
			name:       "good/SHA is HEAD commit",
			wantClient: true,
			event:      &info.Event{SHA: "IAmHEAD321"},
			branchName: "cool-branch",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _ = rtesting.SetupFakeContext(t)
			fakeclient, mux, teardown := thelp.Setup(t)
			defer teardown()
			glProvider := &Provider{sourceProjectID: 1}
			if tt.wantClient {
				glProvider.SetGitLabClient(fakeclient)
				mux.HandleFunc("/projects/1/repository/branches/cool-branch",
					func(rw http.ResponseWriter, _ *http.Request) {
						if tt.errStatusCode != 0 {
							rw.WriteHeader(tt.errStatusCode)
							return
						}
						branch := &gitlab.Branch{Name: "cool-branch", Commit: &gitlab.Commit{ID: "IAmHEAD321"}}
						bytes, _ := json.Marshal(branch)
						_, _ = rw.Write(bytes)
					})
			}
			err := glProvider.isHeadCommitOfBranch(tt.event, tt.branchName)
			if tt.ErrMsg != "" {
				assert.ErrorContains(t, err, tt.ErrMsg)
				return
			}
			assert.NilError(t, err)
		})
	}
}

func TestGitLabCreateComment(t *testing.T) {
	tests := []struct {
		name          string
		event         *info.Event
		commit        string
		updateMarker  string
		mockResponses map[string]func(rw http.ResponseWriter, _ *http.Request)
		wantErr       string
		clientNil     bool
	}{
		{
			name:      "nil client error",
			clientNil: true,
			event:     &info.Event{PullRequestNumber: 123},
			wantErr:   "no gitlab client has been initialized",
		},
		{
			name:    "not a merge request error",
			event:   &info.Event{PullRequestNumber: 0},
			wantErr: "create comment only works on merge requests",
		},
		{
			name:         "create new comment",
			event:        &info.Event{PullRequestNumber: 123},
			commit:       "New Comment",
			updateMarker: "",
			mockResponses: map[string]func(rw http.ResponseWriter, _ *http.Request){
				"/projects/666/merge_requests/123/notes": func(rw http.ResponseWriter, r *http.Request) {
					assert.Equal(t, r.Method, http.MethodPost)
					rw.WriteHeader(http.StatusCreated)
					fmt.Fprint(rw, `{}`)
				},
			},
		},
		{
			name:         "update existing comment",
			event:        &info.Event{PullRequestNumber: 123},
			commit:       "Updated Comment",
			updateMarker: "MARKER",
			mockResponses: map[string]func(rw http.ResponseWriter, _ *http.Request){
				"/projects/666/merge_requests/123/notes": func(rw http.ResponseWriter, r *http.Request) {
					if r.Method == http.MethodGet {
						fmt.Fprint(rw, `[{"id": 555, "body": "MARKER"}]`)
						return
					}
				},
				"/projects/666/merge_requests/123/notes/555": func(rw http.ResponseWriter, r *http.Request) {
					assert.Equal(t, r.Method, "PUT")
					rw.WriteHeader(http.StatusOK)
					fmt.Fprint(rw, `{}`)
				},
			},
		},
		{
			name:         "no matching comment creates new",
			event:        &info.Event{PullRequestNumber: 123},
			commit:       "New Comment",
			updateMarker: "MARKER",
			mockResponses: map[string]func(rw http.ResponseWriter, _ *http.Request){
				"/projects/666/merge_requests/123/notes": func(rw http.ResponseWriter, r *http.Request) {
					if r.Method == http.MethodGet {
						fmt.Fprint(rw, `[{"id": 555, "body": "NO_MATCH"}]`)
						return
					}
					assert.Equal(t, r.Method, http.MethodPost)
					rw.WriteHeader(http.StatusCreated)
					fmt.Fprint(rw, `{}`)
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeclient, mux, teardown := thelp.Setup(t)
			defer teardown()
			observer, _ := zapobserver.New(zap.InfoLevel)
			logger := zap.New(observer).Sugar()

			if tt.clientNil {
				p := &Provider{
					Logger:          logger,
					sourceProjectID: 666,
				}
				err := p.CreateComment(context.Background(), tt.event, tt.commit, tt.updateMarker)
				assert.ErrorContains(t, err, tt.wantErr)
				return
			}

			for endpoint, handler := range tt.mockResponses {
				mux.HandleFunc(endpoint, handler)
			}

			p := &Provider{
				sourceProjectID: 666,
				gitlabClient:    fakeclient,
			}
			err := p.CreateComment(context.Background(), tt.event, tt.commit, tt.updateMarker)
			if tt.wantErr != "" {
				assert.ErrorContains(t, err, tt.wantErr)
			} else {
				assert.NilError(t, err)
			}
		})
	}
}
