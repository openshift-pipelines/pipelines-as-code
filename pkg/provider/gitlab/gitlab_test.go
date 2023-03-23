package gitlab

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/settings"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider"
	thelp "github.com/openshift-pipelines/pipelines-as-code/pkg/provider/gitlab/test"
	"github.com/xanzy/go-gitlab"
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
			v := &Provider{
				targetProjectID: tt.fields.targetProjectID,
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

			pacOpts := &info.PacOpts{Settings: &settings.Settings{ApplicationName: "Test me"}}
			if err := v.CreateStatus(ctx, nil, tt.args.event, pacOpts, tt.args.statusOpts); (err != nil) != tt.wantErr {
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
	assert.Assert(t, v.SetClient(ctx, nil, info.NewEvent()) != nil)

	client, _, tearDown := thelp.Setup(t)
	defer tearDown()
	vv := &Provider{Client: client}
	err := vv.SetClient(ctx, nil, &info.Event{
		Provider: &info.Provider{
			Token: "hello",
		},
	})
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
	err := v.SetClient(ctx, nil, event)
	assert.ErrorContains(t, err, "no git_provider.secret has been set")

	event.Provider.Token = "hello"

	v.repoURL, event.URL, event.Provider.URL = "", "", ""
	event.URL = fmt.Sprintf("%s/hello-this-is-me-ze/project", fakehost)
	err = v.SetClient(ctx, nil, event)
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
		event *info.Event
		path  string
	}
	tests := []struct {
		name       string
		fields     fields
		args       args
		wantStr    string
		wantErr    bool
		wantClient bool
		prcontent  string
	}{
		{
			name:    "no client set",
			wantErr: true,
		},
		{
			name:       "not found, no err",
			wantClient: true,
			args:       args{event: &info.Event{}},
		},
		{
			name:       "bad yaml",
			wantClient: true,
			args:       args{event: &info.Event{SHA: "abcd", HeadBranch: "main"}},
			fields: fields{
				sourceProjectID: 10,
			},
			prcontent: "bad yaml",
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

			v := &Provider{
				targetProjectID: tt.fields.targetProjectID,
				sourceProjectID: tt.fields.sourceProjectID,
				userID:          tt.fields.userID,
			}
			if tt.wantClient {
				client, mux, tearDown := thelp.Setup(t)
				v.Client = client
				if tt.args.path != "" && tt.prcontent != "" {
					thelp.MuxListTektonDir(t, mux, tt.fields.sourceProjectID, tt.args.event.HeadBranch, tt.prcontent)
				}
				defer tearDown()
			}

			got, err := v.GetTektonDir(ctx, tt.args.event, tt.args.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetTektonDir() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantStr != "" {
				assert.Assert(t, strings.Contains(got, tt.wantStr), "%s is not in %s", tt.wantStr, got)
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
	got, err := v.GetFileInsideRepo(ctx, event, ".tekton/subtree/pr.yaml", "")
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
	commitFiles := &gitlab.MergeRequest{}
	tests := []struct {
		name        string
		event       *info.Event
		mrchanges   *gitlab.MergeRequest
		pushChanges []*gitlab.Diff
	}{
		{
			name: "pull-request",
			event: &info.Event{
				TriggerTarget:     "pull_request",
				Organization:      "pullrequestowner",
				Repository:        "pullrequestrepository",
				PullRequestNumber: 10,
			},
			mrchanges: &gitlab.MergeRequest{
				Changes: append(commitFiles.Changes,
					struct {
						OldPath     string `json:"old_path"`
						NewPath     string `json:"new_path"`
						AMode       string `json:"a_mode"`
						BMode       string `json:"b_mode"`
						Diff        string `json:"diff"`
						NewFile     bool   `json:"new_file"`
						RenamedFile bool   `json:"renamed_file"`
						DeletedFile bool   `json:"deleted_file"`
					}{NewPath: "test.txt"}),
			},
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
					NewPath: "first.txt",
				}, {
					NewPath: "second.yaml",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, _ := rtesting.SetupFakeContext(t)
			fakeclient, mux, teardown := thelp.Setup(t)
			defer teardown()
			mergeFileChanges := &gitlab.MergeRequest{
				Changes: append(commitFiles.Changes,
					struct {
						OldPath     string `json:"old_path"`
						NewPath     string `json:"new_path"`
						AMode       string `json:"a_mode"`
						BMode       string `json:"b_mode"`
						Diff        string `json:"diff"`
						NewFile     bool   `json:"new_file"`
						RenamedFile bool   `json:"renamed_file"`
						DeletedFile bool   `json:"deleted_file"`
					}{NewPath: "test.txt"}),
			}
			if tt.event.TriggerTarget == "pull_request" {
				mux.HandleFunc(fmt.Sprintf("/projects/0/merge_requests/%d/changes",
					tt.event.PullRequestNumber), func(rw http.ResponseWriter, r *http.Request) {
					jeez, err := json.Marshal(mergeFileChanges)
					assert.NilError(t, err)
					_, _ = rw.Write(jeez)
				})
			}
			pushFileChanges := []*gitlab.Diff{
				{
					NewPath: "first.txt",
				}, {
					NewPath: "second.yaml",
				},
			}
			if tt.event.TriggerTarget == "push" {
				mux.HandleFunc(fmt.Sprintf("/projects/0/repository/commits/%s/diff",
					tt.event.SHA), func(rw http.ResponseWriter, r *http.Request) {
					jeez, err := json.Marshal(pushFileChanges)
					assert.NilError(t, err)
					_, _ = rw.Write(jeez)
				})
			}

			providerInfo := &Provider{Client: fakeclient}
			fileData, err := providerInfo.GetFiles(ctx, tt.event)
			assert.NilError(t, err, nil)
			if tt.event.TriggerTarget == "pull_request" {
				for i := range fileData {
					assert.Equal(t, tt.mrchanges.Changes[i].NewPath, fileData[i])
				}
			}
			if tt.event.TriggerTarget == "push" {
				for i := range fileData {
					assert.Equal(t, tt.pushChanges[i].NewPath, fileData[i])
				}
			}
		})
	}
}
