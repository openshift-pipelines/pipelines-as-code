package gitlab

import (
	"net/http"
	"testing"

	"github.com/google/go-github/v61/github"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/opscomments"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	thelp "github.com/openshift-pipelines/pipelines-as-code/pkg/provider/gitlab/test"
	"github.com/xanzy/go-gitlab"
	"gotest.tools/v3/assert"
	rtesting "knative.dev/pkg/reconciler/testing"
)

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
				payload: sample.MREventAsJSON("open", ""),
			},
			wantErr: true,
		},
		{
			name: "merge event",
			args: args{
				event:   gitlab.EventTypeMergeRequest,
				payload: sample.MREventAsJSON("open", ""),
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
			name: "tag event",
			args: args{
				event:   gitlab.EventTypeTagPush,
				payload: sample.PushEventAsJSON(true),
			},
			want: &info.Event{
				EventType:     "Tag Push",
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
				EventType:     opscomments.NoOpsCommentEventType.String(),
				TriggerTarget: "pull_request",
				Organization:  "hello-this-is-me-ze",
				Repository:    "project",
			},
		},
		{
			name: "note event test",
			args: args{
				event:   gitlab.EventTypeNote,
				payload: sample.NoteEventAsJSON("/test dummy"),
			},
			want: &info.Event{
				EventType:     opscomments.TestSingleCommentEventType.String(),
				TriggerTarget: "pull_request",
				Organization:  "hello-this-is-me-ze",
				Repository:    "project",
				State:         info.State{TargetTestPipelineRun: "dummy"},
			},
		},
		{
			name: "note event cancel all",
			args: args{
				event:   gitlab.EventTypeNote,
				payload: sample.NoteEventAsJSON("/cancel"),
			},
			want: &info.Event{
				EventType:     opscomments.CancelCommentAllEventType.String(),
				TriggerTarget: "pull_request",
				Organization:  "hello-this-is-me-ze",
				Repository:    "project",
			},
		},
		{
			name: "note event cancel a pr",
			args: args{
				event:   gitlab.EventTypeNote,
				payload: sample.NoteEventAsJSON("/cancel dummy"),
			},
			want: &info.Event{
				EventType:     opscomments.CancelCommentSingleEventType.String(),
				TriggerTarget: "pull_request",
				Organization:  "hello-this-is-me-ze",
				Repository:    "project",
				State:         info.State{TargetCancelPipelineRun: "dummy"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, _ := rtesting.SetupFakeContext(t)
			v := &Provider{
				Token:           github.String("tokeneuneu"),
				targetProjectID: tt.fields.targetProjectID,
				sourceProjectID: tt.fields.sourceProjectID,
				userID:          tt.fields.userID,
			}
			if tt.wantClient {
				client, _, tearDown := thelp.Setup(t)
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
				if tt.want.TargetTestPipelineRun != "" {
					assert.Equal(t, tt.want.TargetTestPipelineRun, got.TargetTestPipelineRun)
				}
				if tt.want.TargetCancelPipelineRun != "" {
					assert.Equal(t, tt.want.TargetCancelPipelineRun, got.TargetCancelPipelineRun)
				}
			}
		})
	}
}
