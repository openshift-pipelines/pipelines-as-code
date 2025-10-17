package matcher

import (
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/clients"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	testclient "github.com/openshift-pipelines/pipelines-as-code/pkg/test/clients"
	testnewrepo "github.com/openshift-pipelines/pipelines-as-code/pkg/test/repository"
	"go.uber.org/zap"
	zapobserver "go.uber.org/zap/zaptest/observer"
	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	rtesting "knative.dev/pkg/reconciler/testing"
)

const (
	mainBranch            = "mainBranch"
	targetNamespace       = "targetNamespace"
	targetOldestNamespace = "targetOldestNamespace"
	targetURL             = "https//nowhere.togo"
)

func Test_getRepoByCR(t *testing.T) {
	cw := clockwork.NewFakeClock()
	type args struct {
		data     testclient.Data
		runevent info.Event
	}
	tests := []struct {
		name         string
		args         args
		wantTargetNS string
		wantErr      bool
	}{
		{
			name: "test-match",
			args: args{
				data: testclient.Data{
					Repositories: []*v1alpha1.Repository{
						testnewrepo.NewRepo(
							testnewrepo.RepoTestcreationOpts{
								Name:             "test-good",
								URL:              targetURL,
								InstallNamespace: targetNamespace,
								CreateTime:       metav1.Time{Time: cw.Now().Add(-1 * time.Minute)},
							},
						),
					},
				},
				runevent: info.Event{URL: targetURL, BaseBranch: mainBranch, EventType: "pull_request"},
			},
			wantTargetNS: targetNamespace,
			wantErr:      false,
		},
		{
			name: "test-match-url-slash-at-the-end",
			args: args{
				data: testclient.Data{
					Repositories: []*v1alpha1.Repository{
						testnewrepo.NewRepo(
							testnewrepo.RepoTestcreationOpts{
								Name:             "test-good",
								URL:              "https//nowhere.togo/",
								InstallNamespace: targetNamespace,
							},
						),
					},
				},
				runevent: info.Event{URL: targetURL, BaseBranch: mainBranch, EventType: "pull_request"},
			},
			wantTargetNS: targetNamespace,
			wantErr:      false,
		},
		{
			name: "test-nomatch-url",
			args: args{
				data: testclient.Data{
					Repositories: []*v1alpha1.Repository{
						testnewrepo.NewRepo(
							testnewrepo.RepoTestcreationOpts{
								Name:             "test-good",
								URL:              "http://nottarget.url",
								InstallNamespace: targetNamespace,
							},
						),
					},
				},
				runevent: info.Event{URL: targetURL, BaseBranch: mainBranch, EventType: "pull_request"},
			},
			wantTargetNS: "",
			wantErr:      false,
		},
		{
			name: "straightforward-branch",
			args: args{
				data: testclient.Data{
					Repositories: []*v1alpha1.Repository{
						testnewrepo.NewRepo(
							testnewrepo.RepoTestcreationOpts{
								Name:             "test-good",
								URL:              targetURL,
								InstallNamespace: targetNamespace,
							},
						),
					},
				},
				runevent: info.Event{
					URL: targetURL, BaseBranch: "refs/heads/mainBranch",
					EventType: "pull_request",
				},
			},
			wantTargetNS: targetNamespace,
			wantErr:      false,
		},
		{
			name: "test-multiple-match-get-oldest",
			args: args{
				data: testclient.Data{
					Repositories: []*v1alpha1.Repository{
						testnewrepo.NewRepo(
							testnewrepo.RepoTestcreationOpts{
								Name:             "test-new",
								URL:              targetURL,
								InstallNamespace: targetNamespace,
								CreateTime:       metav1.Time{Time: cw.Now().Add(-1 * time.Minute)},
							},
						),
						testnewrepo.NewRepo(
							testnewrepo.RepoTestcreationOpts{
								Name:             "test-old",
								URL:              targetURL,
								InstallNamespace: targetOldestNamespace,
								CreateTime:       metav1.Time{Time: cw.Now().Add(-5 * time.Minute)},
							},
						),
					},
				},
				runevent: info.Event{URL: targetURL, BaseBranch: mainBranch, EventType: "pull_request"},
			},
			wantTargetNS: targetOldestNamespace,
			wantErr:      false,
		},
		{
			name: "glob-branch",
			args: args{
				data: testclient.Data{
					Repositories: []*v1alpha1.Repository{
						testnewrepo.NewRepo(
							testnewrepo.RepoTestcreationOpts{
								Name:             "test-good",
								URL:              targetURL,
								InstallNamespace: targetNamespace,
							},
						),
					},
				},
				runevent: info.Event{
					URL:        targetURL,
					BaseBranch: "refs/tags/1.0",
					EventType:  "pull_request",
				},
			},
			wantTargetNS: targetNamespace,
			wantErr:      false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, _ := rtesting.SetupFakeContext(t)
			cs, _ := testclient.SeedTestData(t, ctx, tt.args.data)
			observer, _ := zapobserver.New(zap.InfoLevel)
			logger := zap.New(observer).Sugar()
			client := &params.Run{
				Clients: clients.Clients{PipelineAsCode: cs.PipelineAsCode, Log: logger},
				Info:    info.Info{},
			}
			got, err := MatchEventURLRepo(ctx, client, &tt.args.runevent, "")

			if err == nil && tt.wantErr {
				assert.NilError(t, err, "GetRepoByCR() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantTargetNS == "" && got != nil {
				t.Errorf("GetRepoByCR() got = '%v', want '%v'", got.GetNamespace(), tt.wantTargetNS)
			}
			if tt.wantTargetNS != "" && got == nil {
				t.Errorf("GetRepoByCR() want nil got '%v'", tt.wantTargetNS)
			}

			if tt.wantTargetNS != "" && tt.wantTargetNS != got.GetNamespace() {
				t.Errorf("GetRepoByCR() got = '%v', want '%v'", got.GetNamespace(), tt.wantTargetNS)
			}
		})
	}
}

func TestIncomingWebhookRule(t *testing.T) {
	tests := []struct {
		name             string
		branch           string
		incomingWebhooks []v1alpha1.Incoming
		wantMatch        bool
		wantSecretName   string
	}{
		{
			name:   "exact match - backward compatibility",
			branch: "main",
			incomingWebhooks: []v1alpha1.Incoming{
				{
					Type:    "webhook-url",
					Secret:  v1alpha1.Secret{Name: "main-secret", Key: "token"},
					Targets: []string{"main", "develop"},
				},
			},
			wantMatch:      true,
			wantSecretName: "main-secret",
		},
		{
			name:   "exact match - second target",
			branch: "develop",
			incomingWebhooks: []v1alpha1.Incoming{
				{
					Type:    "webhook-url",
					Secret:  v1alpha1.Secret{Name: "dev-secret", Key: "token"},
					Targets: []string{"main", "develop"},
				},
			},
			wantMatch:      true,
			wantSecretName: "dev-secret",
		},
		{
			name:   "regex match - feature branch",
			branch: "feature/new-ui",
			incomingWebhooks: []v1alpha1.Incoming{
				{
					Type:    "webhook-url",
					Secret:  v1alpha1.Secret{Name: "feature-secret", Key: "token"},
					Targets: []string{"^feature/.*"},
				},
			},
			wantMatch:      true,
			wantSecretName: "feature-secret",
		},
		{
			name:   "regex match - release branch with semver",
			branch: "release/v1.2.3",
			incomingWebhooks: []v1alpha1.Incoming{
				{
					Type:    "webhook-url",
					Secret:  v1alpha1.Secret{Name: "release-secret", Key: "token"},
					Targets: []string{"^release/v[0-9]+\\.[0-9]+\\.[0-9]+$"},
				},
			},
			wantMatch:      true,
			wantSecretName: "release-secret",
		},
		{
			name:   "first match wins - exact before regex",
			branch: "main",
			incomingWebhooks: []v1alpha1.Incoming{
				{
					Type:    "webhook-url",
					Secret:  v1alpha1.Secret{Name: "main-secret", Key: "token"},
					Targets: []string{"main"},
				},
				{
					Type:    "webhook-url",
					Secret:  v1alpha1.Secret{Name: "catch-all-secret", Key: "token"},
					Targets: []string{"^.*$"},
				},
			},
			wantMatch:      true,
			wantSecretName: "main-secret",
		},
		{
			name:   "first match wins - first regex wins",
			branch: "feature/testing",
			incomingWebhooks: []v1alpha1.Incoming{
				{
					Type:    "webhook-url",
					Secret:  v1alpha1.Secret{Name: "feature-secret", Key: "token"},
					Targets: []string{"^feature/.*"},
				},
				{
					Type:    "webhook-url",
					Secret:  v1alpha1.Secret{Name: "catch-all-secret", Key: "token"},
					Targets: []string{"^.*$"},
				},
			},
			wantMatch:      true,
			wantSecretName: "feature-secret",
		},
		{
			name:   "first match wins - webhook order matters",
			branch: "hotfix/critical",
			incomingWebhooks: []v1alpha1.Incoming{
				{
					Type:    "webhook-url",
					Secret:  v1alpha1.Secret{Name: "catch-all-secret", Key: "token"},
					Targets: []string{"^.*$"},
				},
				{
					Type:    "webhook-url",
					Secret:  v1alpha1.Secret{Name: "hotfix-secret", Key: "token"},
					Targets: []string{"^hotfix/.*"},
				},
			},
			wantMatch:      true,
			wantSecretName: "catch-all-secret", // First webhook matches, even though second is more specific
		},
		{
			name:   "no match - branch not in targets",
			branch: "unknown-branch",
			incomingWebhooks: []v1alpha1.Incoming{
				{
					Type:    "webhook-url",
					Secret:  v1alpha1.Secret{Name: "main-secret", Key: "token"},
					Targets: []string{"main", "develop"},
				},
			},
			wantMatch: false,
		},
		{
			name:   "no match - regex doesn't match",
			branch: "main",
			incomingWebhooks: []v1alpha1.Incoming{
				{
					Type:    "webhook-url",
					Secret:  v1alpha1.Secret{Name: "feature-secret", Key: "token"},
					Targets: []string{"^feature/.*"},
				},
			},
			wantMatch: false,
		},
		{
			name:   "invalid regex - skip and continue",
			branch: "main",
			incomingWebhooks: []v1alpha1.Incoming{
				{
					Type:    "webhook-url",
					Secret:  v1alpha1.Secret{Name: "bad-secret", Key: "token"},
					Targets: []string{"^(invalid[regex$"}, // Invalid regex
				},
				{
					Type:    "webhook-url",
					Secret:  v1alpha1.Secret{Name: "good-secret", Key: "token"},
					Targets: []string{"main"},
				},
			},
			wantMatch:      true,
			wantSecretName: "good-secret", // Should skip invalid regex and match next webhook
		},
		{
			name:   "mixed exact and regex in same webhook",
			branch: "staging",
			incomingWebhooks: []v1alpha1.Incoming{
				{
					Type:    "webhook-url",
					Secret:  v1alpha1.Secret{Name: "mixed-secret", Key: "token"},
					Targets: []string{"main", "^feature/.*", "staging"},
				},
			},
			wantMatch:      true,
			wantSecretName: "mixed-secret",
		},
		{
			name:   "regex with character class",
			branch: "JIRA-12345/bugfix",
			incomingWebhooks: []v1alpha1.Incoming{
				{
					Type:    "webhook-url",
					Secret:  v1alpha1.Secret{Name: "jira-secret", Key: "token"},
					Targets: []string{"^[A-Z]+-[0-9]+/.*$"},
				},
			},
			wantMatch:      true,
			wantSecretName: "jira-secret",
		},
		{
			name:   "regex with alternation",
			branch: "dev/testing",
			incomingWebhooks: []v1alpha1.Incoming{
				{
					Type:    "webhook-url",
					Secret:  v1alpha1.Secret{Name: "env-secret", Key: "token"},
					Targets: []string{"^(dev|staging|prod)/.*$"},
				},
			},
			wantMatch:      true,
			wantSecretName: "env-secret",
		},
		{
			name:             "empty webhooks",
			branch:           "main",
			incomingWebhooks: []v1alpha1.Incoming{},
			wantMatch:        false,
		},
		{
			name:   "empty targets",
			branch: "main",
			incomingWebhooks: []v1alpha1.Incoming{
				{
					Type:    "webhook-url",
					Secret:  v1alpha1.Secret{Name: "empty-secret", Key: "token"},
					Targets: []string{},
				},
			},
			wantMatch: false,
		},
		{
			name:   "regex pattern with dot-plus",
			branch: "hotfix/urgent-fix",
			incomingWebhooks: []v1alpha1.Incoming{
				{
					Type:    "webhook-url",
					Secret:  v1alpha1.Secret{Name: "hotfix-secret", Key: "token"},
					Targets: []string{"^hotfix/.+$"},
				},
			},
			wantMatch:      true,
			wantSecretName: "hotfix-secret",
		},
		{
			name:   "multiple webhooks - production first wins",
			branch: "main",
			incomingWebhooks: []v1alpha1.Incoming{
				{
					Type:    "webhook-url",
					Secret:  v1alpha1.Secret{Name: "prod-secret", Key: "token"},
					Targets: []string{"main"},
				},
				{
					Type:    "webhook-url",
					Secret:  v1alpha1.Secret{Name: "dev-secret", Key: "token"},
					Targets: []string{"develop"},
				},
				{
					Type:    "webhook-url",
					Secret:  v1alpha1.Secret{Name: "feature-secret", Key: "token"},
					Targets: []string{"^feature/.*"},
				},
			},
			wantMatch:      true,
			wantSecretName: "prod-secret",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IncomingWebhookRule(tt.branch, tt.incomingWebhooks)

			if tt.wantMatch {
				if got == nil {
					t.Errorf("IncomingWebhookRule() = nil, want match for branch %s", tt.branch)
					return
				}
				if got.Secret.Name != tt.wantSecretName {
					t.Errorf("IncomingWebhookRule() got secret name = %s, want %s", got.Secret.Name, tt.wantSecretName)
				}
			} else if got != nil {
				t.Errorf("IncomingWebhookRule() = %+v, want nil for branch %s", got, tt.branch)
			}
		})
	}
}

func TestMatchTarget(t *testing.T) {
	tests := []struct {
		name       string
		branch     string
		target     string
		wantMatch  bool
		wantError  bool
		errorCheck string
	}{
		{
			name:      "exact match",
			branch:    "main",
			target:    "main",
			wantMatch: true,
			wantError: false,
		},
		{
			name:      "exact match - no match",
			branch:    "main",
			target:    "develop",
			wantMatch: false,
			wantError: false,
		},
		{
			name:      "regex match - simple pattern",
			branch:    "feature/new-ui",
			target:    "^feature/.*",
			wantMatch: true,
			wantError: false,
		},
		{
			name:      "regex match - anchored pattern",
			branch:    "release/v1.2.3",
			target:    "^release/v[0-9]+\\.[0-9]+\\.[0-9]+$",
			wantMatch: true,
			wantError: false,
		},
		{
			name:      "regex no match - strict pattern",
			branch:    "release/v1.2",
			target:    "^release/v[0-9]+\\.[0-9]+\\.[0-9]+$",
			wantMatch: false,
			wantError: false,
		},
		{
			name:       "invalid regex",
			branch:     "main",
			target:     "^(invalid[regex$",
			wantMatch:  false,
			wantError:  true,
			errorCheck: "invalid regex pattern",
		},
		{
			name:      "regex with character class",
			branch:    "JIRA-123/feature",
			target:    "^[A-Z]+-[0-9]+/.*$",
			wantMatch: true,
			wantError: false,
		},
		{
			name:      "regex with alternation",
			branch:    "dev/testing",
			target:    "^(dev|staging|prod)/.*$",
			wantMatch: true,
			wantError: false,
		},
		{
			name:      "regex with dot-star",
			branch:    "anything-goes",
			target:    "^.*$",
			wantMatch: true,
			wantError: false,
		},
		{
			name:      "regex with dot-plus",
			branch:    "hotfix/urgent",
			target:    "^hotfix/.+$",
			wantMatch: true,
			wantError: false,
		},
		{
			name:      "regex with backslash-d",
			branch:    "release-2024",
			target:    "^release-\\d+$",
			wantMatch: true,
			wantError: false,
		},
		{
			name:      "regex with backslash-w",
			branch:    "feature_branch",
			target:    "^feature\\w+$",
			wantMatch: true,
			wantError: false,
		},
		{
			name:      "plain string with slash - not treated as regex",
			branch:    "feature/test",
			target:    "feature/test",
			wantMatch: true,
			wantError: false,
		},
		{
			name:      "plain string - case sensitive",
			branch:    "Main",
			target:    "main",
			wantMatch: false,
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotMatch, err := matchTarget(tt.branch, tt.target)

			if tt.wantError {
				if err == nil {
					t.Errorf("matchTarget() expected error but got nil")
					return
				}
				if tt.errorCheck != "" {
					assert.ErrorContains(t, err, tt.errorCheck)
				}
			} else {
				if err != nil {
					t.Errorf("matchTarget() unexpected error = %v", err)
					return
				}
				if gotMatch != tt.wantMatch {
					t.Errorf("matchTarget() gotMatch = %v, want %v for branch=%s, target=%s", gotMatch, tt.wantMatch, tt.branch, tt.target)
				}
			}
		})
	}
}

func TestMatchTarget_UnanchoredPatterns(t *testing.T) {
	tests := []struct {
		name      string
		branch    string
		target    string
		wantMatch bool
		wantError bool
	}{
		{
			name:      "unanchored - matches substring",
			branch:    "test/release/v1.0",
			target:    "release",
			wantMatch: false, // "release" is exact match, not regex
			wantError: false,
		},
		{
			name:      "unanchored regex - matches substring",
			branch:    "test/release/v1.0",
			target:    "release.*",
			wantMatch: true, // Matches "release/v1.0" substring
			wantError: false,
		},
		{
			name:      "unanchored - dot star prefix",
			branch:    "feature/my-branch",
			target:    ".*feature",
			wantMatch: true,
			wantError: false,
		},
		{
			name:      "unanchored - dot star suffix",
			branch:    "my-feature-branch",
			target:    "feature.*",
			wantMatch: true,
			wantError: false,
		},
		{
			name:      "unanchored - dot star both sides",
			branch:    "test/production/deploy",
			target:    ".*production.*",
			wantMatch: true,
			wantError: false,
		},
		{
			name:      "overly permissive - matches almost everything",
			branch:    "any-branch-name",
			target:    ".*",
			wantMatch: true,
			wantError: false,
		},
		{
			name:      "overly permissive - dot plus",
			branch:    "any-branch",
			target:    ".+",
			wantMatch: true,
			wantError: false,
		},
		{
			name:      "unanchored character class",
			branch:    "test-123-branch",
			target:    "[0-9]+",
			wantMatch: true, // Matches "123"
			wantError: false,
		},
		{
			name:      "partial word match - dangerous for security",
			branch:    "reproduce-bug",
			target:    "prod.*", // Might match unintended branches
			wantMatch: true,     // Matches "produce-bug"
			wantError: false,
		},
		{
			name:      "partial match - main substring",
			branch:    "mainly-this-branch",
			target:    "main.*",
			wantMatch: true, // Matches "mainly-this-branch"
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotMatch, err := matchTarget(tt.branch, tt.target)

			if tt.wantError {
				if err == nil {
					t.Errorf("matchTarget() expected error but got nil")
				}
			} else {
				if err != nil {
					t.Errorf("matchTarget() unexpected error = %v", err)
					return
				}
				if gotMatch != tt.wantMatch {
					t.Errorf("matchTarget() gotMatch = %v, want %v for branch=%s, target=%s", gotMatch, tt.wantMatch, tt.branch, tt.target)
				}
			}
		})
	}
}

func TestMatchTarget_ReDoSPatterns(t *testing.T) {
	tests := []struct {
		name      string
		branch    string
		target    string
		wantError bool
		skipTest  bool
		reason    string
	}{
		{
			name:      "catastrophic backtracking - nested quantifiers",
			branch:    "aaaaaaaaaaaaaaaaaaaaaaaX",
			target:    "^(a+)+$",
			wantError: false, // Go's RE2 engine is safe from ReDoS
		},
		{
			name:      "catastrophic backtracking - alternation with quantifiers",
			branch:    "aaaaaaaaaaaaaaaaaaaaaaaX",
			target:    "^(a|a)*$",
			wantError: false, // RE2 handles this safely
		},
		{
			name:      "catastrophic backtracking - optional groups",
			branch:    "aaaaaaaaaaaaaaaaaaaaaaaX",
			target:    "^(a*)*$",
			wantError: false, // RE2 safe
		},
		{
			name:      "complex nested quantifiers",
			branch:    "aaaaabbbbbX",
			target:    "^(a+|b+)*$",
			wantError: false, // RE2 safe
		},
		{
			name:      "exponential backtracking pattern",
			branch:    "aaaaaaaaaaaaaaaaaaaaX",
			target:    "^([a-zA-Z]+)*$",
			wantError: false, // RE2 safe
		},
		{
			name:      "overlapping alternation",
			branch:    "aaaaaaaaaaaaaaaa",
			target:    "^(a|ab|abc)*$",
			wantError: false, // RE2 safe
		},
		{
			name:      "very long repetition",
			branch:    "test-branch",
			target:    "^(.*a){100}.*$",
			wantError: false, // RE2 handles this
		},
		{
			name:      "nested groups with quantifiers",
			branch:    "aaaaaaaaaaaaaaX",
			target:    "^((a+)+)+$",
			wantError: false, // RE2 safe
		},
		{
			name:      "optional nested groups",
			branch:    "test-branch-name",
			target:    "^((test)?)+$",
			wantError: false, // RE2 safe
		},
		{
			name:      "greedy quantifiers with alternation",
			branch:    "aaaaaaaaaaaaX",
			target:    "^(a+|a*)+$",
			wantError: false, // RE2 safe
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.skipTest {
				t.Skip(tt.reason)
			}

			// Use a timeout to catch patterns that might hang
			done := make(chan struct{})
			var gotMatch bool
			var err error

			go func() {
				gotMatch, err = matchTarget(tt.branch, tt.target)
				close(done)
			}()

			select {
			case <-done:
				// Test completed
				if tt.wantError {
					if err == nil {
						t.Errorf("matchTarget() expected error but got nil")
					}
				} else {
					if err != nil {
						t.Logf("matchTarget() error (expected for some patterns): %v", err)
					}
					// Just verify it didn't hang - we don't care about the match result for ReDoS tests
					t.Logf("Pattern '%s' completed successfully (match=%v)", tt.target, gotMatch)
				}
			case <-time.After(5 * time.Second):
				t.Errorf("matchTarget() timed out after 5 seconds - potential ReDoS vulnerability with pattern '%s'", tt.target)
			}
		})
	}
}
