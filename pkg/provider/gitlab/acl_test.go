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

func TestIsAllowedOwnersFile(t *testing.T) {
	tests := []struct {
		name                    string
		targetProjectID         int
		sender                  string
		defaultBranch           string
		ownersFile              string
		ownersAliasesFile       string
		ownersFileError         bool
		ownersAliasesError      bool
		ownersAliasesStatusCode int
		wantAllowed             bool
		wantErr                 bool
	}{
		{
			name:            "no owners file",
			targetProjectID: 5000,
			sender:          "testuser",
			defaultBranch:   "main",
			ownersFile:      "",
			wantAllowed:     false,
			wantErr:         false,
		},
		{
			name:            "owners file allows user",
			targetProjectID: 5000,
			sender:          "testuser",
			defaultBranch:   "main",
			ownersFile:      "---\napprovers:\n  - testuser\n",
			wantAllowed:     true,
			wantErr:         false,
		},
		{
			name:            "owners file denies user",
			targetProjectID: 5000,
			sender:          "testuser",
			defaultBranch:   "main",
			ownersFile:      "---\napprovers:\n  - someoneelse\n",
			wantAllowed:     false,
			wantErr:         false,
		},
		{
			name:              "owners file with aliases not found",
			targetProjectID:   5000,
			sender:            "testuser",
			defaultBranch:     "main",
			ownersFile:        "---\napprovers:\n  - testuser\n",
			ownersAliasesFile: "",
			wantAllowed:       true,
			wantErr:           false,
		},
		{
			name:              "owners file with aliases file exists",
			targetProjectID:   5000,
			sender:            "testuser",
			defaultBranch:     "main",
			ownersFile:        "---\napprovers:\n  - team-lead\n",
			ownersAliasesFile: "---\naliases:\n  team-lead:\n    - testuser\n",
			wantAllowed:       true,
			wantErr:           false,
		},
		{
			name:                    "owners aliases returns error status",
			targetProjectID:         5000,
			sender:                  "testuser",
			defaultBranch:           "main",
			ownersFile:              "---\napprovers:\n  - testuser\n",
			ownersAliasesError:      true,
			ownersAliasesStatusCode: http.StatusUnauthorized,
			wantAllowed:             false,
			wantErr:                 true,
		},
		{
			name:                    "owners aliases returns internal server error",
			targetProjectID:         5000,
			sender:                  "testuser",
			defaultBranch:           "main",
			ownersFile:              "---\napprovers:\n  - testuser\n",
			ownersAliasesError:      true,
			ownersAliasesStatusCode: http.StatusInternalServerError,
			wantAllowed:             false,
			wantErr:                 true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, _ := rtesting.SetupFakeContext(t)

			v := &Provider{
				targetProjectID: tt.targetProjectID,
			}

			client, mux, tearDown := thelp.Setup(t)
			defer tearDown()
			v.gitlabClient = client

			// Setup OWNERS file
			if tt.ownersFile != "" {
				thelp.MuxGetFile(mux, tt.targetProjectID, "OWNERS", tt.ownersFile, tt.ownersFileError)
			} else {
				// Return empty for missing OWNERS file
				thelp.MuxGetFile(mux, tt.targetProjectID, "OWNERS", "", false)
			}

			// Setup OWNERS_ALIASES file
			switch {
			case tt.ownersAliasesError:
				// Setup error response for OWNERS_ALIASES
				path := fmt.Sprintf("/projects/%d/repository/files/OWNERS_ALIASES/raw", tt.targetProjectID)
				mux.HandleFunc(path, func(rw http.ResponseWriter, _ *http.Request) {
					rw.WriteHeader(tt.ownersAliasesStatusCode)
					_, _ = rw.Write([]byte(`{"error": "test error"}`))
				})
			case tt.ownersAliasesFile != "":
				thelp.MuxGetFile(mux, tt.targetProjectID, "OWNERS_ALIASES", tt.ownersAliasesFile, false)
			default:
				// Return 404 for missing OWNERS_ALIASES file
				path := fmt.Sprintf("/projects/%d/repository/files/OWNERS_ALIASES/raw", tt.targetProjectID)
				mux.HandleFunc(path, func(rw http.ResponseWriter, _ *http.Request) {
					rw.WriteHeader(http.StatusNotFound)
					_, _ = rw.Write([]byte(`{"error": "not found"}`))
				})
			}

			ev := &info.Event{
				Sender:        tt.sender,
				DefaultBranch: tt.defaultBranch,
			}

			// Execute IsAllowedOwnersFile
			allowed, err := v.IsAllowedOwnersFile(ctx, ev)

			// Verify error
			if (err != nil) != tt.wantErr {
				t.Errorf("IsAllowedOwnersFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// Verify result
			if allowed != tt.wantAllowed {
				t.Errorf("IsAllowedOwnersFile() = %v, want %v", allowed, tt.wantAllowed)
			}
		})
	}
}

func TestCheckMembership(t *testing.T) {
	tests := []struct {
		name              string
		targetProjectID   int
		userID            int
		apiAllowMember    bool
		apiFailure        bool
		ownersFile        string
		sender            string
		wantResult        bool
		wantCached        bool
		wantCachedValue   bool
		verifyCacheNotSet bool
		verifyRetry       bool
	}{
		{
			name:            "gitlab member + owners allowed",
			targetProjectID: 5000,
			userID:          1000,
			apiAllowMember:  true,
			ownersFile:      "---\napprovers:\n  - testuser\n",
			sender:          "testuser",
			wantResult:      true,
			wantCached:      true,
			wantCachedValue: true,
		},
		{
			name:            "gitlab member + owners denied",
			targetProjectID: 5000,
			userID:          1000,
			apiAllowMember:  true,
			ownersFile:      "---\napprovers:\n  - someoneelse\n",
			sender:          "testuser",
			wantResult:      true,
			wantCached:      true,
			wantCachedValue: true,
		},
		{
			name:            "gitlab not member + owners allowed",
			targetProjectID: 5000,
			userID:          1000,
			apiAllowMember:  false,
			ownersFile:      "---\napprovers:\n  - testuser\n",
			sender:          "testuser",
			wantResult:      true,
			wantCached:      true,
			wantCachedValue: true,
		},
		{
			name:            "gitlab not member + owners denied",
			targetProjectID: 5000,
			userID:          1000,
			apiAllowMember:  false,
			ownersFile:      "---\napprovers:\n  - someoneelse\n",
			sender:          "testuser",
			wantResult:      false,
			wantCached:      true,
			wantCachedValue: false,
		},
		{
			name:              "api failure + owners allowed",
			targetProjectID:   5000,
			userID:            1000,
			apiFailure:        true,
			ownersFile:        "---\napprovers:\n  - testuser\n",
			sender:            "testuser",
			wantResult:        true,
			verifyCacheNotSet: true,
		},
		{
			name:              "api failure + owners denied",
			targetProjectID:   5000,
			userID:            1000,
			apiFailure:        true,
			ownersFile:        "---\napprovers:\n  - someoneelse\n",
			sender:            "testuser",
			wantResult:        false,
			verifyCacheNotSet: true,
			verifyRetry:       true,
		},
		{
			name:            "cache initialization",
			targetProjectID: 5000,
			userID:          1000,
			apiAllowMember:  true,
			sender:          "testuser",
			wantResult:      true,
			wantCached:      true,
			wantCachedValue: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, _ := rtesting.SetupFakeContext(t)

			v := &Provider{
				targetProjectID: tt.targetProjectID,
				userID:          tt.userID,
				memberCache:     nil, // Start with nil cache to test lazy initialization
			}

			client, mux, tearDown := thelp.Setup(t)
			defer tearDown()
			v.gitlabClient = client

			var callCount int
			// Setup API response
			switch {
			case tt.apiFailure:
				path := fmt.Sprintf("/projects/%d/members/all/%d", tt.targetProjectID, tt.userID)
				mux.HandleFunc(path, func(rw http.ResponseWriter, _ *http.Request) {
					callCount++
					rw.WriteHeader(http.StatusInternalServerError)
					_, _ = rw.Write([]byte(`{"error": "internal server error"}`))
				})
			case tt.apiAllowMember:
				thelp.MuxAllowUserID(mux, tt.targetProjectID, tt.userID)
			default:
				thelp.MuxDisallowUserID(mux, tt.targetProjectID, tt.userID)
			}

			// Setup OWNERS file
			if tt.ownersFile != "" {
				thelp.MuxGetFile(mux, tt.targetProjectID, "OWNERS", tt.ownersFile, false)
			}

			ev := &info.Event{
				Sender:        tt.sender,
				DefaultBranch: "main",
			}

			// Execute checkMembership
			result := v.checkMembership(ctx, ev, tt.userID)

			// Verify result
			if result != tt.wantResult {
				t.Errorf("checkMembership() = %v, want %v", result, tt.wantResult)
			}

			// Verify cache behavior
			if tt.verifyCacheNotSet {
				if _, ok := v.memberCache[tt.userID]; ok {
					t.Errorf("expected result NOT to be cached when API fails")
				}
			} else if tt.wantCached {
				cached, ok := v.memberCache[tt.userID]
				if !ok {
					t.Errorf("expected result to be cached")
				} else if cached != tt.wantCachedValue {
					t.Errorf("cached value = %v, want %v", cached, tt.wantCachedValue)
				}
			}

			// Verify cache was initialized
			if v.memberCache == nil {
				t.Errorf("expected memberCache to be initialized")
			}

			// Verify retry behavior for API failures
			if tt.verifyRetry {
				initialCallCount := callCount
				result = v.checkMembership(ctx, ev, tt.userID)
				if result != tt.wantResult {
					t.Errorf("checkMembership() on retry = %v, want %v", result, tt.wantResult)
				}
				if callCount <= initialCallCount {
					t.Errorf("expected API to be called again (not cached), but call count did not increase")
				}
			}
		})
	}
}
