package gitlab

import (
	"testing"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	thelp "github.com/openshift-pipelines/pipelines-as-code/pkg/provider/gitlab/test"
	rtesting "knative.dev/pkg/reconciler/testing"
)

func TestIsAllowed(t *testing.T) {
	type fields struct {
		targetProjectID int
		sourceProjectID int
		userID          int
	}
	type args struct {
		event *info.Event
	}
	tests := []struct {
		name            string
		fields          fields
		args            args
		allowed         bool
		wantErr         bool
		wantClient      bool
		allowMemberID   int
		ownerFile       string
		commentContent  string
		commentAuthor   string
		commentAuthorID int
	}{
		{
			name:    "check client has been set",
			wantErr: true,
		},
		{
			name:          "allowed as member of project",
			allowed:       true,
			wantClient:    true,
			allowMemberID: 123,
			fields: fields{
				userID:          123,
				targetProjectID: 2525,
			},
			args: args{
				event: &info.Event{},
			},
		},
		{
			name:       "allowed from ownerfile",
			allowed:    true,
			wantClient: true,
			fields: fields{
				userID:          123,
				targetProjectID: 2525,
			},
			args: args{
				event: &info.Event{Sender: "allowmeplease"},
			},
			ownerFile: "---\n approvers:\n  - allowmeplease\n",
		},
		{
			name:       "allowed from ok-to-test",
			allowed:    true,
			wantClient: true,
			fields: fields{
				userID:          6666,
				targetProjectID: 2525,
			},
			args: args{
				event: &info.Event{Sender: "noowner", PullRequestNumber: 1},
			},
			allowMemberID:   1111,
			commentContent:  "/ok-to-test",
			commentAuthor:   "admin",
			commentAuthorID: 1111,
		},
		{
			name:       "disallowed from non authorized note",
			wantClient: true,
			fields: fields{
				userID:          6666,
				targetProjectID: 2525,
			},
			args: args{
				event: &info.Event{Sender: "noowner", PullRequestNumber: 1},
			},
			commentContent: "/ok-to-test",
			commentAuthor:  "notallowed",
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
				if tt.allowMemberID != 0 {
					thelp.MuxAllowUserID(mux, tt.fields.targetProjectID, tt.allowMemberID)
				}
				if tt.ownerFile != "" {
					thelp.MuxGetFile(mux, tt.fields.targetProjectID, "OWNERS", tt.ownerFile)
				}
				if tt.commentContent != "" {
					thelp.MuxDiscussionsNote(mux, tt.fields.targetProjectID,
						tt.args.event.PullRequestNumber, tt.commentAuthor, tt.commentAuthorID, tt.commentContent)
				}

				defer tearDown()
			}
			got, err := v.IsAllowed(ctx, tt.args.event)
			if (err != nil) != tt.wantErr {
				t.Errorf("IsAllowed() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.allowed {
				t.Errorf("IsAllowed() got = %v, want %v", got, tt.allowed)
			}
		})
	}
}
