package gitlab

import (
	"io/ioutil"
	"net/http"
	"strings"
	"testing"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider"
	thelp "github.com/openshift-pipelines/pipelines-as-code/pkg/provider/gitlab/test"
	"github.com/xanzy/go-gitlab"
	"go.uber.org/zap"
	zapobserver "go.uber.org/zap/zaptest/observer"
	"gotest.tools/v3/assert"
	rtesting "knative.dev/pkg/reconciler/testing"
)

func TestCreateStatus(t *testing.T) {
	type fields struct {
		targetProjectID int
		mergeRequestID  int
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
				mergeRequestID:  tt.fields.mergeRequestID,
			}
			if tt.wantClient {
				client, mux, tearDown := thelp.Setup(ctx, t)
				v.Client = client
				defer tearDown()
				thelp.MuxNotePost(t, mux, v.targetProjectID, v.mergeRequestID, tt.args.postStr)
			}

			if tt.args.event == nil {
				tt.args.event = &info.Event{}
			}

			pacOpts := &info.PacOpts{ApplicationName: "Test me"}
			if err := v.CreateStatus(ctx, tt.args.event, pacOpts, tt.args.statusOpts); (err != nil) != tt.wantErr {
				t.Errorf("CreateStatus() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestParsePayload(t *testing.T) {
	sample := thelp.TEvent{
		Username:          "foo",
		DefaultBranch:     "main",
		URL:               "https://foo.com",
		SHA:               "sha",
		SHAurl:            "https://url",
		SHAtitle:          "commit it",
		Headbranch:        "branch",
		Basebranch:        "main",
		UserID:            10,
		MRID:              1,
		TargetProjectID:   100,
		SourceProjectID:   200,
		PathWithNameSpace: "hello/this/is/me/ze/project",
	}
	type fields struct {
		targetProjectID int
		sourceProjectID int
		mergeRequestID  int
		userID          int
	}
	type args struct {
		event   gitlab.EventType
		payload string
	}
	tests := []struct {
		name       string
		fields     fields
		args       args
		want       *info.Event
		wantErr    bool
		wantClient bool
	}{
		{
			name: "bad payload",
			args: args{
				payload: "nono",
				event:   "none",
			},
			wantErr: true,
		},
		{
			name: "event not supported",
			args: args{
				event:   gitlab.EventTypePipeline,
				payload: sample.MREventAsJSON(),
			},
			wantErr: true,
		},
		{
			name: "merge event",
			args: args{
				event:   gitlab.EventTypeMergeRequest,
				payload: sample.MREventAsJSON(),
			},
			want: &info.Event{
				EventType:     "Merge Request",
				TriggerTarget: "pull_request",
				Organization:  "hello-this-is-me-ze",
				Repository:    "project",
			},
		},
		{
			name: "push event no commits",
			args: args{
				event:   gitlab.EventTypePush,
				payload: sample.PushEventAsJSON(false),
			},
			wantErr: true,
		},
		{
			name: "push event",
			args: args{
				event:   gitlab.EventTypePush,
				payload: sample.PushEventAsJSON(true),
			},
			want: &info.Event{
				EventType:     "Push",
				TriggerTarget: "push",
				Organization:  "hello-this-is-me-ze",
				Repository:    "project",
			},
		},

		{
			name: "note event",
			args: args{
				event:   gitlab.EventTypeNote,
				payload: sample.NoteEventAsJSON(""),
			},
			want: &info.Event{
				EventType:     "Note",
				TriggerTarget: "pull_request",
				Organization:  "hello-this-is-me-ze",
				Repository:    "project",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, _ := rtesting.SetupFakeContext(t)
			v := &Provider{
				Token:           gitlab.String("tokeneuneu"),
				targetProjectID: tt.fields.targetProjectID,
				sourceProjectID: tt.fields.sourceProjectID,
				mergeRequestID:  tt.fields.mergeRequestID,
				userID:          tt.fields.userID,
			}
			if tt.wantClient {
				client, _, tearDown := thelp.Setup(ctx, t)
				v.Client = client
				defer tearDown()
			}
			run := &params.Run{
				Info: info.Info{},
			}

			request := &http.Request{Header: map[string][]string{}}
			request.Header.Set("X-Gitlab-Event", string(tt.args.event))

			got, err := v.ParsePayload(ctx, run, request, tt.args.payload)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParsePayload() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.want != nil {
				assert.Equal(t, tt.want.TriggerTarget, got.TriggerTarget)
				assert.Equal(t, tt.want.EventType, got.EventType)
				assert.Equal(t, tt.want.Organization, got.Organization)
				assert.Equal(t, tt.want.Repository, got.Repository)
			}
		})
	}
}

func TestGetCommitInfo(t *testing.T) {
	ctx, _ := rtesting.SetupFakeContext(t)
	client, _, tearDown := thelp.Setup(ctx, t)
	v := &Provider{Client: client}

	defer tearDown()
	assert.NilError(t, v.GetCommitInfo(ctx, nil))

	ncv := &Provider{}
	assert.Assert(t, ncv.GetCommitInfo(ctx, nil) != nil)
}

func TestGetConfig(t *testing.T) {
	v := &Provider{}
	assert.Assert(t, v.GetConfig().APIURL != "")
	assert.Assert(t, v.GetConfig().TaskStatusTMPL != "")
}

func TestSetClient(t *testing.T) {
	ctx, _ := rtesting.SetupFakeContext(t)
	v := &Provider{}
	assert.Assert(t, v.SetClient(ctx, info.NewEvent()) != nil)

	client, _, tearDown := thelp.Setup(ctx, t)
	defer tearDown()
	vv := &Provider{Client: client}
	err := vv.SetClient(ctx, &info.Event{
		Provider: &info.Provider{
			Token: "hello",
		},
	})
	assert.NilError(t, err)
	assert.Assert(t, *vv.Token != "")
}

func TestGetTektonDir(t *testing.T) {
	samplePR, err := ioutil.ReadFile("../../resolve/testdata/pipeline-finally.yaml")
	assert.NilError(t, err)
	type fields struct {
		targetProjectID int
		sourceProjectID int
		mergeRequestID  int
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
				mergeRequestID:  tt.fields.mergeRequestID,
				userID:          tt.fields.userID,
			}
			if tt.wantClient {
				client, mux, tearDown := thelp.Setup(ctx, t)
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
	client, mux, tearDown := thelp.Setup(ctx, t)
	defer tearDown()

	event := &info.Event{
		HeadBranch: "branch",
	}
	v := Provider{
		sourceProjectID: 10,
		Client:          client,
	}
	thelp.MuxListTektonDir(t, mux, v.sourceProjectID, event.HeadBranch, content)
	got, err := v.GetFileInsideRepo(ctx, event, ".tekton/pr.yaml", "")
	assert.NilError(t, err)
	assert.Equal(t, content, got)

	_, err = v.GetFileInsideRepo(ctx, event, "notfound", "")
	assert.Assert(t, err != nil)
}

func TestProvider_Detect(t *testing.T) {
	sample := thelp.TEvent{
		Username:          "foo",
		DefaultBranch:     "main",
		URL:               "https://foo.com",
		SHA:               "sha",
		SHAurl:            "https://url",
		SHAtitle:          "commit it",
		Headbranch:        "branch",
		Basebranch:        "main",
		UserID:            10,
		MRID:              1,
		TargetProjectID:   100,
		SourceProjectID:   200,
		PathWithNameSpace: "hello/this/is/me/ze/project",
	}
	tests := []struct {
		name          string
		wantErrString string
		isGL          bool
		processReq    bool
		event         string
		eventType     gitlab.EventType
	}{
		{
			name:       "not a gitlab Event",
			eventType:  "",
			isGL:       false,
			processReq: false,
		},
		{
			name:          "invalid gitlab Event",
			eventType:     "invalid",
			wantErrString: "unexpected event type: invalid",
			isGL:          false,
			processReq:    false,
		},
		{
			name:       "valid merge Event",
			event:      sample.MREventAsJSON(),
			eventType:  gitlab.EventTypeMergeRequest,
			isGL:       true,
			processReq: true,
		},
		{
			name:       "issue note event with no valid comment",
			event:      sample.NoteEventAsJSON("abc"),
			eventType:  gitlab.EventTypeNote,
			isGL:       true,
			processReq: false,
		},
		{
			name:       "issue note Event with ok-to-test comment",
			event:      sample.NoteEventAsJSON("/ok-to-test"),
			eventType:  gitlab.EventTypeNote,
			isGL:       true,
			processReq: true,
		},
		{
			name:       "issue comment Event with ok-to-test and some string",
			event:      sample.NoteEventAsJSON("abc /ok-to-test"),
			eventType:  gitlab.EventTypeNote,
			isGL:       true,
			processReq: false,
		},
		{
			name:       "issue comment Event with retest",
			event:      sample.NoteEventAsJSON("/retest"),
			eventType:  gitlab.EventTypeNote,
			isGL:       true,
			processReq: true,
		},
		{
			name:       "push event",
			event:      sample.PushEventAsJSON(true),
			eventType:  gitlab.EventTypePush,
			isGL:       true,
			processReq: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gprovider := Provider{}
			logger := getLogger()

			header := &http.Header{}
			header.Set("X-Gitlab-Event", string(tt.eventType))

			isGL, processReq, _, err := gprovider.Detect(header, tt.event, logger)
			if tt.wantErrString != "" {
				assert.ErrorContains(t, err, tt.wantErrString)
				return
			}
			assert.NilError(t, err)
			assert.Equal(t, tt.isGL, isGL)
			assert.Equal(t, tt.processReq, processReq)
		})
	}
}

func getLogger() *zap.SugaredLogger {
	observer, _ := zapobserver.New(zap.InfoLevel)
	logger := zap.New(observer).Sugar()
	return logger
}
