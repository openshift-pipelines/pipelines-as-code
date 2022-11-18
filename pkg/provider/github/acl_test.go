package github

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"testing"

	"github.com/google/go-github/v48/github"
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
			name:          "good issue comment event",
			commentsReply: `[{"body": "/ok-to-test", "user": {"login": "owner"}}]`,
			runevent: info.Event{
				Organization: "owner",
				Sender:       "nonowner",
				EventType:    "issue_comment",
				Event: &github.IssueCommentEvent{
					Issue: &github.Issue{
						PullRequestLinks: &github.PullRequestLinks{
							HTMLURL: github.String("http://url.com/owner/repo/1"),
						},
					},
				},
			},
			allowed: true,
			wantErr: false,
		},
		{
			name:          "good issue pull request event",
			commentsReply: `[{"body": "/ok-to-test", "user": {"login": "owner"}}]`,
			runevent: info.Event{
				Organization: "owner",
				Sender:       "nonowner",
				EventType:    "issue_comment",
				Event: &github.PullRequestEvent{
					PullRequest: &github.PullRequest{
						HTMLURL: github.String("http://url.com/owner/repo/1"),
					},
				},
			},
			allowed: true,
			wantErr: false,
		},
		{
			name:          "bad event origin",
			commentsReply: `[{"body": "/ok-to-test", "user": {"login": "owner"}}]`,
			runevent: info.Event{
				Organization: "owner",
				Sender:       "nonowner",
				EventType:    "issue_comment",
				Event: &github.CheckRunEvent{
					CheckRun: &github.CheckRun{
						HTMLURL: github.String("http://url.com/owner/repo/1"),
					},
				},
			},
			allowed: false,
		},
		{
			name:          "no-ok-to-test",
			commentsReply: `[{"body": "Foo Bar", "user": {"login": "owner"}}]`,
			runevent: info.Event{
				Organization: "owner",
				Sender:       "nonowner",
				EventType:    "issue_comment",
				Event: &github.IssueCommentEvent{
					Issue: &github.Issue{
						PullRequestLinks: &github.PullRequestLinks{
							HTMLURL: github.String("http://url.com/owner/repo/1"),
						},
					},
				},
			},
			allowed: false,
			wantErr: false,
		},
		{
			name:          "ok-to-test-not-from-owner",
			commentsReply: `[{"body": "/ok-to-test", "user": {"login": "notowner"}}]`,
			runevent: info.Event{
				Organization: "owner",
				Sender:       "nonowner",
				EventType:    "issue_comment",
				Event: &github.IssueCommentEvent{
					Issue: &github.Issue{
						PullRequestLinks: &github.PullRequestLinks{
							HTMLURL: github.String("http://url.com/owner/repo/1"),
						},
					},
				},
			},
			allowed: false,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.runevent.TriggerTarget = "ok-to-test-comment"
			fakeclient, mux, _, teardown := ghtesthelper.SetupGH()
			defer teardown()
			mux.HandleFunc("/repos/owner/issues/1/comments", func(rw http.ResponseWriter, r *http.Request) {
				fmt.Fprint(rw, tt.commentsReply)
			})
			mux.HandleFunc("/repos/owner/collaborators", func(rw http.ResponseWriter, r *http.Request) {
				fmt.Fprint(rw, "[]")
			})
			ctx, _ := rtesting.SetupFakeContext(t)
			gprovider := Provider{
				Client: fakeclient,
			}

			got, err := gprovider.IsAllowed(ctx, &tt.runevent)
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
	collabOwner := "collab"
	collabRepo := "collabRepo"
	collaborator := "collaborator"
	repoOwnerFileAllowed := "repoOwnerAllowed"

	errit := "err"

	mux.HandleFunc("/orgs/"+orgallowed+"/public_members", func(rw http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(rw, `[{"login": "login_%s"}]`, orgallowed)
	})

	mux.HandleFunc("/orgs/"+orgdenied+"/public_members", func(rw http.ResponseWriter, r *http.Request) {
		fmt.Fprint(rw, `[]`)
	})

	mux.HandleFunc("/repos/"+orgdenied+"/collaborators", func(rw http.ResponseWriter, r *http.Request) {
		fmt.Fprint(rw, `[]`)
	})

	mux.HandleFunc("/orgs/"+errit+"/public_members", func(rw http.ResponseWriter, r *http.Request) {
		fmt.Fprint(rw, `x1x`)
	})

	mux.HandleFunc("/orgs/"+repoOwnerFileAllowed+"/public_members", func(rw http.ResponseWriter, r *http.Request) {
		fmt.Fprint(rw, `[]`)
	})
	mux.HandleFunc("/repos/"+repoOwnerFileAllowed+"/collaborators", func(rw http.ResponseWriter, r *http.Request) {
		fmt.Fprint(rw, `[]`)
	})
	mux.HandleFunc("/repos/"+repoOwnerFileAllowed+"/contents/OWNERS", func(rw http.ResponseWriter, r *http.Request) {
		fmt.Fprint(rw, `{"name": "OWNERS", "path": "OWNERS", "sha": "ownerssha"}`)
	})

	mux.HandleFunc("/repos/"+repoOwnerFileAllowed+"/git/blobs/ownerssha", func(rw http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(rw, `{"content": "%s"}`, base64.RawStdEncoding.EncodeToString([]byte("approvers:\n  - approved\n")))
	})

	mux.HandleFunc(fmt.Sprintf("/repos/%v/%v/collaborators", collabOwner, collabRepo), func(rw http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(rw, `[{"login": "%s"}]`, collaborator)
	})

	ctx, _ := rtesting.SetupFakeContext(t)
	gprovider := Provider{
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
				Organization: orgallowed,
				Sender:       "login_allowed",
			},
			allowed: true,
			wantErr: false,
		},
		{
			name: "sender allowed from owner file",
			runevent: info.Event{
				Organization: repoOwnerFileAllowed,
				Sender:       "approved",
			},
			allowed: true,
			wantErr: false,
		},
		{
			name: "owner is sender is allowed",
			runevent: info.Event{
				Organization: orgallowed,
				Sender:       "allowed",
			},
			allowed: true,
			wantErr: false,
		},
		{
			name: "sender allowed since collaborator on repo",
			runevent: info.Event{
				Organization: collabOwner,
				Repository:   collabRepo,
				Sender:       collaborator,
			},
			allowed: true,
			wantErr: false,
		},
		{
			name: "sender not allowed in org",
			runevent: info.Event{
				Organization: orgdenied,
				Sender:       "notallowed",
			},
			allowed: false,
			wantErr: false,
		},
		{
			name: "err it",
			runevent: info.Event{
				Organization: errit,
				Sender:       "error",
			},
			allowed: false,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := gprovider.IsAllowed(ctx, &tt.runevent)
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
