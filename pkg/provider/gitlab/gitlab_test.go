package gitlab

import (
	"testing"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider"
	thelp "github.com/openshift-pipelines/pipelines-as-code/pkg/provider/gitlab/test"
	"github.com/xanzy/go-gitlab"
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
	sampleMR := thelp.MMEvent{
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
	sampleMRjson := thelp.MakeMergeEvent(sampleMR)
	type fields struct {
		targetProjectID int
		sourceProjectID int
		mergeRequestID  int
		userID          int
	}
	type args struct {
		event   *info.Event
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
				event:   &info.Event{EventType: "none"},
			},
			wantErr: true,
		},
		{
			name: "merge event",
			args: args{
				event: &info.Event{
					EventType: string(gitlab.EventTypeMergeRequest),
				},
				payload: sampleMRjson,
			},
			want: &info.Event{
				EventType:     "Merge Request",
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
				Info: info.Info{
					Event: tt.args.event,
				},
			}
			got, err := v.ParsePayload(ctx, run, tt.args.payload)
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
