package gitlab

import (
	"fmt"
	"net/http"
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
		threadFirstNote string
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
			name:       "allowed when /ok-to-test is in a reply note",
			allowed:    true,
			wantClient: true,
			fields: fields{
				userID:          6666,
				targetProjectID: 2525,
			},
			args: args{
				event: &info.Event{Sender: "noowner", PullRequestNumber: 2},
			},
			allowMemberID:   1111,
			threadFirstNote: "random comment",
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
				v.gitlabClient = client
				if tt.allowMemberID != 0 {
					thelp.MuxAllowUserID(mux, tt.fields.targetProjectID, tt.allowMemberID)
				} else {
					thelp.MuxDisallowUserID(mux, tt.fields.targetProjectID, tt.allowMemberID)
				}
				if tt.ownerFile != "" {
					thelp.MuxGetFile(mux, tt.fields.targetProjectID, "OWNERS", tt.ownerFile, false)
				}
				if tt.commentContent != "" {
					if tt.threadFirstNote != "" {
						thelp.MuxDiscussionsNoteWithReply(mux, tt.fields.targetProjectID,
							tt.args.event.PullRequestNumber,
							"someuser", 2222, tt.threadFirstNote,
							tt.commentAuthor, tt.commentAuthorID, tt.commentContent)
					} else {
						thelp.MuxDiscussionsNote(mux, tt.fields.targetProjectID,
							tt.args.event.PullRequestNumber, tt.commentAuthor, tt.commentAuthorID, tt.commentContent)
					}
				} else {
					thelp.MuxDiscussionsNoteEmpty(mux, tt.fields.targetProjectID, tt.args.event.PullRequestNumber)
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

func TestMembershipCaching(t *testing.T) {
	ctx, _ := rtesting.SetupFakeContext(t)

	v := &Provider{
		targetProjectID: 3030,
		userID:          4242,
	}

	client, mux, tearDown := thelp.Setup(t)
	defer tearDown()
	v.gitlabClient = client

	// Count how many times the membership API is hit.
	var calls int
	thelp.MuxAllowUserIDCounting(mux, v.targetProjectID, v.userID, &calls)

	ev := &info.Event{Sender: "someone", PullRequestNumber: 1}

	// First call should hit the API once and cache the result.
	allowed, err := v.IsAllowed(ctx, ev)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !allowed {
		t.Fatalf("expected allowed on first membership check")
	}
	if calls < 1 {
		t.Fatalf("expected at least 1 membership API call, got %d", calls)
	}

	// Second call should use the cache and not hit the API again.
	allowed, err = v.IsAllowed(ctx, ev)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !allowed {
		t.Fatalf("expected allowed on cached membership check")
	}
	if calls != 1 {
		t.Fatalf("expected cached result with no extra API call, got %d calls", calls)
	}
}

func TestMembershipAPIFailureDoesNotCacheApiError(t *testing.T) {
	ctx, _ := rtesting.SetupFakeContext(t)

	v := &Provider{
		targetProjectID: 3030,
		userID:          4242,
	}

	client, mux, tearDown := thelp.Setup(t)
	defer tearDown()
	v.gitlabClient = client

	ev := &info.Event{Sender: "someone"}

	var (
		calls   int
		success bool
	)
	path := fmt.Sprintf("/projects/%d/members/all/%d", v.targetProjectID, v.userID)
	mux.HandleFunc(path, func(rw http.ResponseWriter, _ *http.Request) {
		calls++
		if !success {
			rw.WriteHeader(http.StatusInternalServerError)
			_, _ = rw.Write([]byte(`{}`))
			return
		}
		_, err := fmt.Fprintf(rw, `{"id": %d}`, v.userID)
		if err != nil {
			t.Fatalf("failed to write response: %v", err)
		}
	})

	thelp.MuxDiscussionsNoteEmpty(mux, v.targetProjectID, ev.PullRequestNumber)

	allowed, err := v.IsAllowed(ctx, ev)
	if err != nil {
		t.Fatalf("unexpected error on failure path: %v", err)
	}
	if allowed {
		t.Fatalf("expected not allowed when membership API fails and no fallback grants access")
	}
	if calls < 1 {
		t.Fatalf("expected at least 1 membership API call, got %d", calls)
	}
	initialCallCount := calls

	// Make the next API call succeed; the provider should retry because the previous failure wasn't cached.
	success = true

	allowed, err = v.IsAllowed(ctx, ev)
	if err != nil {
		t.Fatalf("unexpected error on retry path: %v", err)
	}
	if !allowed {
		t.Fatalf("expected allowed when membership API succeeds on retry")
	}
	if calls <= initialCallCount {
		t.Fatalf("expected membership API to be called again after retry, got %d total calls (initial %d)", calls, initialCallCount)
	}
}
