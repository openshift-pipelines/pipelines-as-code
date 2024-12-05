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
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/clients"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/settings"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider"
	thelp "github.com/openshift-pipelines/pipelines-as-code/pkg/provider/gitlab/test"
	testclient "github.com/openshift-pipelines/pipelines-as-code/pkg/test/clients"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/test/logger"
	"github.com/xanzy/go-gitlab"
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, _ := rtesting.SetupFakeContext(t)
			logger, _ := logger.GetLogger()
			stdata, _ := testclient.SeedTestData(t, ctx, testclient.Data{})
			run := &params.Run{
				Clients: clients.Clients{
					Kube: stdata.Kube,
				},
			}
			v := &Provider{
				targetProjectID: tt.fields.targetProjectID,
				run:             params.New(),
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
				v.Client = client
				defer tearDown()
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
	v := &Provider{Client: client}

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
	v := &Provider{}
	assert.Assert(t, v.SetClient(ctx, nil, info.NewEvent(), nil, nil) != nil)

	client, _, tearDown := thelp.Setup(t)
	defer tearDown()
	vv := &Provider{Client: client}
	err := vv.SetClient(ctx, nil, &info.Event{
		Provider: &info.Provider{
			Token: "hello",
		},
	}, nil, nil)
	assert.NilError(t, err)
	assert.Assert(t, *vv.Token != "")
}

func TestSetClientDetectAPIURL(t *testing.T) {
	fakehost := "https://fakehost.com"
	ctx, _ := rtesting.SetupFakeContext(t)
	client, _, tearDown := thelp.Setup(t)
	defer tearDown()
	v := &Provider{Client: client}
	event := info.NewEvent()
	err := v.SetClient(ctx, nil, event, nil, nil)
	assert.ErrorContains(t, err, "no git_provider.secret has been set")

	event.Provider.Token = "hello"

	v.repoURL, event.URL, event.Provider.URL = "", "", ""
	event.URL = fmt.Sprintf("%s/hello-this-is-me-ze/project", fakehost)
	err = v.SetClient(ctx, nil, event, nil, nil)
	assert.NilError(t, err)
	assert.Equal(t, fakehost, v.apiURL)

	v.repoURL, event.URL, event.Provider.URL = "", "", ""
	event.Provider.URL = fmt.Sprintf("%s/hello-this-is-me-ze/anotherproject", fakehost)
	assert.Equal(t, fakehost, v.apiURL)

	v.repoURL = fmt.Sprintf("%s/hello-this-is-me-ze/anotherproject", fakehost)
	v.pathWithNamespace = "hello-this-is-me-ze/anotherproject"
	event.URL, event.Provider.URL = "", ""
	assert.Equal(t, fakehost, v.apiURL)
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
			name:      "list tekton dir",
			prcontent: string(samplePR),
			args: args{
				path: ".tekton",
				event: &info.Event{
					HeadBranch: "main",
				},
			},
			fields: fields{
				sourceProjectID: 100,
			},
			wantClient:           true,
			wantStr:              "kind: PipelineRun",
			filterMessageSnippet: `Using PipelineRun definition from source merge request SHA`,
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
				v.Client = client
				muxbranch := tt.args.event.HeadBranch
				if tt.args.provenance == "default_branch" {
					muxbranch = tt.args.event.DefaultBranch
				}
				if tt.args.path != "" && tt.prcontent != "" {
					thelp.MuxListTektonDir(t, mux, tt.fields.sourceProjectID, muxbranch, tt.prcontent)
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
		Client:          client,
	}
	thelp.MuxListTektonDir(t, mux, v.sourceProjectID, event.HeadBranch, content)
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

			providerInfo := &Provider{Client: fakeclient, sourceProjectID: tt.sourceProjectID, targetProjectID: tt.targetProjectID}
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
