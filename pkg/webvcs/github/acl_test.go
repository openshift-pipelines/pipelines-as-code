package github

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"testing"

	"github.com/google/go-github/v35/github"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	ghtesthelper "github.com/openshift-pipelines/pipelines-as-code/pkg/test/github"
	rtesting "knative.dev/pkg/reconciler/testing"
)

func TestOkToTestComment(t *testing.T) {
	tests := []struct {
		name          string
		commentsReply string
		runevent      info.Event
		allowed       bool
		wantErr       bool
	}{
		{
			name:          "good",
			commentsReply: `[{"body": "/ok-to-test", "user": {"login": "owner"}}]`,
			runevent: info.Event{
				Owner:     "owner",
				Sender:    "nonowner",
				EventType: "issue_comment",
			},
			allowed: true,
			wantErr: false,
		},
		{
			name:          "no-ok-to-test",
			commentsReply: `[{"body": "Foo Bar", "user": {"login": "owner"}}]`,
			runevent: info.Event{
				Owner:     "owner",
				Sender:    "nonowner",
				EventType: "issue_comment",
			},
			allowed: false,
			wantErr: false,
		},
		{
			name:          "ok-to-test-not-from-owner",
			commentsReply: `[{"body": "/ok-to-test", "user": {"login": "notowner"}}]`,
			runevent: info.Event{
				Owner:     "owner",
				Sender:    "nonowner",
				EventType: "issue_comment",
			},
			allowed: false,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repoOwnerURL := "http://url.com/owner/repo/1"
			tt.runevent.Event = &github.IssueCommentEvent{
				Issue: &github.Issue{
					PullRequestLinks: &github.PullRequestLinks{
						HTMLURL: &repoOwnerURL,
					},
				},
			}
			tt.runevent.TriggerTarget = "ok-to-test-comment"
			fakeclient, mux, _, teardown := ghtesthelper.SetupGH()
			defer teardown()
			mux.HandleFunc("/repos/owner/issues/1/comments", func(rw http.ResponseWriter, r *http.Request) {
				fmt.Fprint(rw, tt.commentsReply)
			})
			ctx, _ := rtesting.SetupFakeContext(t)
			gvcs := VCS{
				Client: fakeclient,
			}

			got, err := gvcs.IsAllowed(ctx, &tt.runevent)
			if (err != nil) != tt.wantErr {
				t.Errorf("aclCheck() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.allowed {
				t.Errorf("aclCheck() = %v, want %v", got, tt.allowed)
			}
		})
	}
}

func TestAclCheckAll(t *testing.T) {
	fakeclient, mux, _, teardown := ghtesthelper.SetupGH()
	defer teardown()

	orgallowed := "allowed"
	orgdenied := "denied"
	repoOwnerFileAllowed := "repoOwnerAllowed"

	errit := "err"

	mux.HandleFunc("/orgs/"+orgallowed+"/public_members", func(rw http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(rw, `[{"login": "login_%s"}]`, orgallowed)
	})

	mux.HandleFunc("/orgs/"+orgdenied+"/public_members", func(rw http.ResponseWriter, r *http.Request) {
		fmt.Fprint(rw, `[]`)
	})

	mux.HandleFunc("/orgs/"+errit+"/public_members", func(rw http.ResponseWriter, r *http.Request) {
		fmt.Fprint(rw, `x1x`)
	})

	mux.HandleFunc("/orgs/"+repoOwnerFileAllowed+"/public_members", func(rw http.ResponseWriter, r *http.Request) {
		fmt.Fprint(rw, `[]`)
	})

	mux.HandleFunc("/repos/"+repoOwnerFileAllowed+"/contents/OWNERS", func(rw http.ResponseWriter, r *http.Request) {
		fmt.Fprint(rw, `{"name": "OWNERS", "path": "OWNERS", "sha": "ownerssha"}`)
	})

	mux.HandleFunc("/repos/"+repoOwnerFileAllowed+"/git/blobs/ownerssha", func(rw http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(rw, `{"content": "%s"}`, base64.RawStdEncoding.EncodeToString([]byte("approvers:\n  - approved\n")))
	})

	ctx, _ := rtesting.SetupFakeContext(t)
	gvcs := VCS{
		Client: fakeclient,
	}

	tests := []struct {
		name     string
		runevent info.Event
		allowed  bool
		wantErr  bool
	}{
		{
			name: "sender allowed in org",
			runevent: info.Event{
				Owner:  orgallowed,
				Sender: "login_allowed",
			},
			allowed: true,
			wantErr: false,
		},
		{
			name: "sender allowed from owner file",
			runevent: info.Event{
				Owner:  repoOwnerFileAllowed,
				Sender: "approved",
			},
			allowed: true,
			wantErr: false,
		},
		{
			name: "owner is sender is allowed",
			runevent: info.Event{
				Owner:  orgallowed,
				Sender: "allowed",
			},
			allowed: true,
			wantErr: false,
		},
		{
			name: "sender not allowed in org",
			runevent: info.Event{
				Owner:  orgdenied,
				Sender: "notallowed",
			},
			allowed: false,
			wantErr: false,
		},
		{
			name: "err it",
			runevent: info.Event{
				Owner:  errit,
				Sender: "error",
			},
			allowed: false,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := gvcs.IsAllowed(ctx, &tt.runevent)
			if (err != nil) != tt.wantErr {
				t.Errorf("aclCheckAll() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.allowed {
				t.Errorf("aclCheckAll() = %v, want %v", got, tt.allowed)
			}
		})
	}
}
