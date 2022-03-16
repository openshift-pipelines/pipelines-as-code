package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"testing"

	"github.com/google/go-github/v43/github"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/clients"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	testclient "github.com/openshift-pipelines/pipelines-as-code/pkg/test/clients"
	ghtesthelper "github.com/openshift-pipelines/pipelines-as-code/pkg/test/github"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/env"
	"gotest.tools/v3/fs"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	rtesting "knative.dev/pkg/reconciler/testing"
)

// script kiddies, don't get too excited, this has been randomly generated with random words
const fakePrivateKey = `-----BEGIN RSA PRIVATE KEY-----
MIICXAIBAAKBgQC6GorZBeri0eVERMZQDFh5E1RMPjFk9AevaWr27yJse6eiUlos
gY2L2vcZKLOrdvVR+TLLapIMFfg1E1qVr1iTHP3IiSCs1uW6NKDmxEQc9Uf/fG9c
i56tGmTVxLkC94AvlVFmgxtWfHdP3lF2O0EcfRyIi6EIbGkWDqWQVEQG2wIDAQAB
AoGAaKOd6FK0dB5Si6Uj4ERgxosAvfHGMh4n6BAc7YUd1ONeKR2myBl77eQLRaEm
DMXRP+sfDVL5lUQRED62ky1JXlDc0TmdLiO+2YVyXI5Tbej0Q6wGVC25/HedguUX
fw+MdKe8jsOOXVRLrJ2GfpKZ2CmOKGTm/hyrFa10TmeoTxkCQQDa4fvqZYD4vOwZ
CplONnVk+PyQETj+mAyUiBnHEeLpztMImNLVwZbrmMHnBtCNx5We10oCLW+Qndfw
Xi4LgliVAkEA2amSV+TZiUVQmm5j9yzon0rt1FK+cmVWfRS/JAUXyvl+Xh/J+7Gu
QzoEGJNAnzkUIZuwhTfNRWlzURWYA8BVrwJAZFQhfJd6PomaTwAktU0REm9ulTrP
vSNE4PBhoHX6ZOGAqfgi7AgIfYVPm+3rupE5a82TBtx8vvUa/fqtcGkW4QJAaL9t
WPUeJyx/XMJxQzuOe1JA4CQt2LmiBLHeRoRY7ephgQSFXKYmed3KqNT8jWOXp5DY
Q1QWaigUQdpFfNCrqwJBANLgWaJV722PhQXOCmR+INvZ7ksIhJVcq/x1l2BYOLw2
QsncVExbMiPa9Oclo5qLuTosS8qwHm1MJEytp3/SkB8=
-----END RSA PRIVATE KEY-----`

var sampleRepo = &github.Repository{
	Owner: &github.User{
		Login: github.String("owner"),
	},
	Name:          github.String("reponame"),
	DefaultBranch: github.String("defaultbranch"),
	HTMLURL:       github.String("https://github.com/owner/repo"),
}

var testInstallationID = int64(1)

var samplePRevent = github.PullRequestEvent{
	PullRequest: &github.PullRequest{
		Head: &github.PullRequestBranch{
			SHA: github.String("sampleHeadsha"),
			Ref: github.String("headred"),
		},
		Base: &github.PullRequestBranch{
			SHA: github.String("basesha"),
			Ref: github.String("baseref"),
		},
		User: &github.User{
			Login: github.String("user"),
		},
	},
	Repo: sampleRepo,
}

var samplePR = github.PullRequest{
	Number: github.Int(54321),
	Head: &github.PullRequestBranch{
		SHA:  github.String("samplePRsha"),
		Repo: sampleRepo,
	},
}

// TODO: better testing matrix against only public function like we do for bitbucket-cloud
func TestPayLoadFix(t *testing.T) {
	b, err := ioutil.ReadFile("testdata/pull_request_with_newlines.json")
	assert.NilError(t, err)
	ctx, _ := rtesting.SetupFakeContext(t)
	fakeclient, mux, _, teardown := ghtesthelper.SetupGH()
	defer teardown()
	mux.HandleFunc("/repos/repo/owner/commits/SHA", func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `{"commit": {"message": "HELLO"}}`)
	})
	mux.HandleFunc("/repos/repo/owner/git/commits/SHA", func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `{"commit": {"message": "HELLO"}}`)
	})

	gprovider := Provider{
		Client: fakeclient,
	}

	logger := getLogger()

	event := &info.Event{
		EventType:     "pull_request",
		TriggerTarget: "pull_request",
	}
	run := &params.Run{
		Clients: clients.Clients{
			Log: logger,
		},
		Info: info.Info{
			Event: event,
		},
	}
	_, err = gprovider.ParsePayload(ctx, run, string(b))
	assert.NilError(t, err)

	// would bomb out on "assertion failed: error is not nil: invalid character
	// '\n' in string literal" if we don't fix the payload
	assert.NilError(t, err)
}

func TestEventParsePayLoad(t *testing.T) {
	fakeclient, mux, _, teardown := ghtesthelper.SetupGH()
	defer teardown()

	tests := []struct {
		name               string
		wantErrString      string
		eventType          string
		payloadEventStruct interface{}
		jeez               string
		triggerTarget      string
		githubClient       *github.Client
		muxReplies         map[string]interface{}
		shaRet             string
	}{
		{
			name:          "bad/unknow event",
			wantErrString: "unknown X-Github-Event",
			eventType:     "unknown",
			triggerTarget: "unknown",
		},
		{
			name:          "bad/invalid json",
			wantErrString: "invalid character",
			eventType:     "pull_request",
			triggerTarget: "unknown",
			jeez:          "xxxx",
		},
		{
			name:               "bad/not supported",
			wantErrString:      "this event is not supported",
			eventType:          "pull_request_review_comment",
			triggerTarget:      "pull_request",
			payloadEventStruct: github.PullRequestReviewCommentEvent{Action: github.String("created")},
		},
		{
			name:               "bad/check run only issue recheck supported",
			wantErrString:      "only issue recheck is supported",
			eventType:          "check_run",
			triggerTarget:      "nonopetitrobot",
			payloadEventStruct: github.CheckRunEvent{Action: github.String("created")},
			githubClient:       fakeclient,
		},
		{
			name:               "bad/check run only with github apps",
			wantErrString:      "only supported with github apps",
			eventType:          "check_run",
			triggerTarget:      "pull_request",
			payloadEventStruct: github.CheckRunEvent{Action: github.String("created")},
		},
		{
			name:               "bad/issue comment retest only with github apps",
			wantErrString:      "only supported with github apps",
			eventType:          "issue_comment",
			triggerTarget:      "pull_request",
			payloadEventStruct: github.IssueCommentEvent{Action: github.String("created")},
		},
		{
			name:               "bad/issue comment not coming from pull request",
			eventType:          "issue_comment",
			triggerTarget:      "pull_request",
			githubClient:       fakeclient,
			payloadEventStruct: github.IssueCommentEvent{Issue: &github.Issue{}},
			wantErrString:      "issue comment is not coming from a pull_request",
		},
		{
			name:          "bad/issue comment invalid pullrequest",
			eventType:     "issue_comment",
			triggerTarget: "pull_request",
			githubClient:  fakeclient,
			payloadEventStruct: github.IssueCommentEvent{Issue: &github.Issue{
				PullRequestLinks: &github.PullRequestLinks{
					HTMLURL: github.String("/bad"),
				},
			}},
			wantErrString: "bad pull request number",
		},
		{
			name:          "bad/rerequest error fetching PR",
			githubClient:  fakeclient,
			eventType:     "check_run",
			triggerTarget: "pull_request",
			wantErrString: "404",
			payloadEventStruct: github.CheckRunEvent{
				Action: github.String("rerequested"),
				Repo:   sampleRepo,
				CheckRun: &github.CheckRun{
					CheckSuite: &github.CheckSuite{
						PullRequests: []*github.PullRequest{&samplePR},
					},
				},
			},
			shaRet: "samplePRsha",
		},
		{
			name:          "good/rerequest on pull request",
			eventType:     "check_run",
			githubClient:  fakeclient,
			triggerTarget: "issue-recheck",
			payloadEventStruct: github.CheckRunEvent{
				Action: github.String("rerequested"),
				Repo:   sampleRepo,
				CheckRun: &github.CheckRun{
					CheckSuite: &github.CheckSuite{
						PullRequests: []*github.PullRequest{&samplePR},
					},
				},
			},
			muxReplies: map[string]interface{}{"/repos/owner/reponame/pulls/54321": samplePR},
			shaRet:     "samplePRsha",
		},
		{
			name:          "good/rerequest on push",
			eventType:     "check_run",
			githubClient:  fakeclient,
			triggerTarget: "issue-recheck",
			payloadEventStruct: github.CheckRunEvent{
				Action: github.String("rerequested"),
				Repo:   sampleRepo,
				CheckRun: &github.CheckRun{
					CheckSuite: &github.CheckSuite{
						HeadSHA: github.String("headSHACheckSuite"),
					},
				},
			},
			shaRet: "headSHACheckSuite",
		},
		{
			name:          "good/issue comment",
			eventType:     "issue_comment",
			triggerTarget: "pull_request",
			githubClient:  fakeclient,
			payloadEventStruct: github.IssueCommentEvent{
				Issue: &github.Issue{
					PullRequestLinks: &github.PullRequestLinks{
						HTMLURL: github.String("/666"),
					},
				},
				Repo: sampleRepo,
			},
			muxReplies: map[string]interface{}{"/repos/owner/reponame/pulls/666": samplePR},
			shaRet:     "samplePRsha",
		},
		{
			name:               "good/pull request",
			eventType:          "pull_request",
			triggerTarget:      "pull_request",
			payloadEventStruct: samplePRevent,
			shaRet:             "sampleHeadsha",
		},
		{
			name:          "good/push",
			eventType:     "push",
			triggerTarget: "push",
			payloadEventStruct: github.PushEvent{
				Repo: &github.PushEventRepository{
					Owner: &github.User{Login: github.String("owner")},
					Name:  github.String("pushRepo"),
				},
				HeadCommit: &github.HeadCommit{ID: github.String("SHAPush")},
			},
			shaRet: "SHAPush",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, _ := rtesting.SetupFakeContext(t)

			for key, value := range tt.muxReplies {
				mux.HandleFunc(key, func(rw http.ResponseWriter, r *http.Request) {
					bjeez, _ := json.Marshal(value)
					fmt.Fprint(rw, string(bjeez))
				})
			}
			gprovider := Provider{
				Client: tt.githubClient,
			}
			logger := getLogger()
			event := &info.Event{
				EventType:     tt.eventType,
				TriggerTarget: tt.triggerTarget,
			}

			run := &params.Run{
				Clients: clients.Clients{
					Log: logger,
				},
				Info: info.Info{
					Event: event,
				},
			}
			bjeez, _ := json.Marshal(tt.payloadEventStruct)
			jeez := string(bjeez)
			if tt.jeez != "" {
				jeez = tt.jeez
			}
			ret, err := gprovider.ParseEventPayload(ctx, run, jeez)
			if tt.wantErrString != "" {
				assert.ErrorContains(t, err, tt.wantErrString)
				return
			}
			assert.NilError(t, err)
			assert.Assert(t, ret != nil)
			assert.Equal(t, tt.shaRet, ret.SHA)
		})
	}
}

func TestParsePayLoad(t *testing.T) {
	fakeclient, mux, _, teardown := ghtesthelper.SetupGH()
	defer teardown()

	tests := []struct {
		name               string
		wantErrString      string
		eventType          string
		payloadEventStruct interface{}
		jeez               string
		triggerTarget      string
		githubClient       *github.Client
		muxReplies         map[string]interface{}
		shaRet             string
	}{
		{
			name:          "bad/unknow event",
			wantErrString: "unknown X-Github-Event",
			eventType:     "unknown",
		},
		{
			name:          "bad/invalid json",
			wantErrString: "invalid character",
			eventType:     "pull_request",
			jeez:          "xxxx",
		},
		{
			name:               "bad/not supported",
			wantErrString:      "this event is not supported",
			eventType:          "pull_request_review_comment",
			payloadEventStruct: github.PullRequestReviewCommentEvent{Action: github.String("created")},
		},
		{
			name:               "bad/check run only issue recheck supported",
			wantErrString:      "only issue recheck is supported",
			eventType:          "check_run",
			triggerTarget:      "nonopetitrobot",
			payloadEventStruct: github.CheckRunEvent{Action: github.String("created")},
			githubClient:       fakeclient,
		},
		{
			name:               "bad/check run only with github apps",
			wantErrString:      "only supported with github apps",
			eventType:          "check_run",
			payloadEventStruct: github.CheckRunEvent{Action: github.String("created")},
		},
		{
			name:               "bad/issue comment retest only with github apps",
			wantErrString:      "only supported with github apps",
			eventType:          "issue_comment",
			payloadEventStruct: github.IssueCommentEvent{Action: github.String("created")},
		},
		{
			name:               "bad/issue comment not coming from pull request",
			eventType:          "issue_comment",
			githubClient:       fakeclient,
			payloadEventStruct: github.IssueCommentEvent{Issue: &github.Issue{}},
			wantErrString:      "issue comment is not coming from a pull_request",
		},
		{
			name:         "bad/issue comment invalid pullrequest",
			eventType:    "issue_comment",
			githubClient: fakeclient,
			payloadEventStruct: github.IssueCommentEvent{Issue: &github.Issue{
				PullRequestLinks: &github.PullRequestLinks{
					HTMLURL: github.String("/bad"),
				},
			}},
			wantErrString: "bad pull request number",
		},
		{
			name:          "bad/rerequest error fetching PR",
			githubClient:  fakeclient,
			eventType:     "check_run",
			triggerTarget: "pull_request",
			wantErrString: "404",
			payloadEventStruct: github.CheckRunEvent{
				Action: github.String("rerequested"),
				Repo:   sampleRepo,
				CheckRun: &github.CheckRun{
					CheckSuite: &github.CheckSuite{
						PullRequests: []*github.PullRequest{&samplePR},
					},
				},
			},
			shaRet: "samplePRsha",
		},

		{
			name:          "good/rerequest on pull request",
			eventType:     "check_run",
			githubClient:  fakeclient,
			triggerTarget: "issue-recheck",
			payloadEventStruct: github.CheckRunEvent{
				Action: github.String("rerequested"),
				Repo:   sampleRepo,
				CheckRun: &github.CheckRun{
					CheckSuite: &github.CheckSuite{
						PullRequests: []*github.PullRequest{&samplePR},
					},
				},
			},
			muxReplies: map[string]interface{}{"/repos/owner/reponame/pulls/54321": samplePR},
			shaRet:     "samplePRsha",
		},
		{
			name:          "good/rerequest on push",
			eventType:     "check_run",
			githubClient:  fakeclient,
			triggerTarget: "issue-recheck",
			payloadEventStruct: github.CheckRunEvent{
				Action: github.String("rerequested"),
				Repo:   sampleRepo,
				CheckRun: &github.CheckRun{
					CheckSuite: &github.CheckSuite{
						HeadSHA: github.String("headSHACheckSuite"),
					},
				},
			},
			shaRet: "headSHACheckSuite",
		},
		{
			name:         "good/issue comment",
			eventType:    "issue_comment",
			githubClient: fakeclient,
			payloadEventStruct: github.IssueCommentEvent{
				Issue: &github.Issue{
					PullRequestLinks: &github.PullRequestLinks{
						HTMLURL: github.String("/666"),
					},
				},
				Repo: sampleRepo,
			},
			muxReplies: map[string]interface{}{"/repos/owner/reponame/pulls/666": samplePR},
			shaRet:     "samplePRsha",
		},
		{
			name:               "good/pull request",
			eventType:          "pull_request",
			triggerTarget:      "pull_request",
			payloadEventStruct: samplePRevent,
			shaRet:             "sampleHeadsha",
		},
		{
			name:          "good/push",
			eventType:     "push",
			triggerTarget: "push",
			payloadEventStruct: github.PushEvent{
				Repo: &github.PushEventRepository{
					Owner: &github.User{Login: github.String("owner")},
					Name:  github.String("pushRepo"),
				},
				HeadCommit: &github.HeadCommit{ID: github.String("SHAPush")},
			},
			shaRet: "SHAPush",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, _ := rtesting.SetupFakeContext(t)

			for key, value := range tt.muxReplies {
				mux.HandleFunc(key, func(rw http.ResponseWriter, r *http.Request) {
					bjeez, _ := json.Marshal(value)
					fmt.Fprint(rw, string(bjeez))
				})
			}
			gprovider := Provider{
				Client: tt.githubClient,
			}
			logger := getLogger()
			event := &info.Event{
				EventType:     tt.eventType,
				TriggerTarget: tt.triggerTarget,
			}

			run := &params.Run{
				Clients: clients.Clients{
					Log: logger,
				},
				Info: info.Info{
					Event: event,
				},
			}
			bjeez, _ := json.Marshal(tt.payloadEventStruct)
			jeez := string(bjeez)
			if tt.jeez != "" {
				jeez = tt.jeez
			}
			ret, err := gprovider.ParsePayload(ctx, run, jeez)
			if tt.wantErrString != "" {
				assert.ErrorContains(t, err, tt.wantErrString)
				return
			}
			assert.NilError(t, err)
			assert.Assert(t, ret != nil)
			assert.Equal(t, tt.shaRet, ret.SHA)
		})
	}
}

func TestAppTokenGeneration(t *testing.T) {
	fakeGithubAuthURL := "https://fake.gitub.auth/api/v3/"
	tests := []struct {
		name          string
		wantErr       bool
		nilClient     bool
		envs          map[string]string
		wsSecretFiles map[string]string
		resultBaseURL string
	}{
		{
			name: "Env not set",
			envs: map[string]string{
				"FOO": "bar",
			},
			nilClient: true,
		},
		{
			name: "Bad Account ID",
			envs: map[string]string{
				"PAC_INSTALLATION_ID":  "bad",
				"PAC_WORKSPACE_SECRET": "xxx",
			},
			wantErr: true,
		},
		{
			name: "bad/Workspace path doesn't exist",
			envs: map[string]string{
				"PAC_INSTALLATION_ID":  "11111",
				"PAC_WORKSPACE_SECRET": "not/here",
			},
			wantErr: true,
		},
		{
			name: "bad/Application ID in workspace doesn't exist",
			envs: map[string]string{
				"PAC_INSTALLATION_ID":  "11111",
				"PAC_WORKSPACE_SECRET": "here",
			},
			wsSecretFiles: map[string]string{
				"nogithub-application-id": "Foo",
			},
			wantErr: true,
		},
		{
			name: "bad/wrong ApplicationID",
			envs: map[string]string{
				"PAC_INSTALLATION_ID":  "11111",
				"PAC_WORKSPACE_SECRET": "here",
			},
			wsSecretFiles: map[string]string{
				"github-application-id": "BAD",
			},
			wantErr: true,
		},
		{
			name: "bad/Private Key not present",
			envs: map[string]string{
				"PAC_INSTALLATION_ID":  "11111",
				"PAC_WORKSPACE_SECRET": "here",
			},
			wsSecretFiles: map[string]string{
				"github-application-id": "2222",
			},
			wantErr: true,
		},
		{
			name: "Bad/Private Key chelou",
			envs: map[string]string{
				"PAC_INSTALLATION_ID":     "11111",
				"PAC_WORKSPACE_SECRET":    "here",
				"PAC_GIT_PROVIDER_APIURL": "foo.bar.com",
			},
			wsSecretFiles: map[string]string{
				"github-application-id": "2222",
				"github-private-key":    "hello",
			},
			wantErr: true,
		},
		{
			name: "Good/ghe base domain",
			envs: map[string]string{
				"PAC_INSTALLATION_ID":     "11111",
				"PAC_WORKSPACE_SECRET":    "here",
				"PAC_GIT_PROVIDER_APIURL": "foo.bar.com",
			},
			wsSecretFiles: map[string]string{
				"github-application-id": "2222",
				"github-private-key":    fakePrivateKey,
			},
			resultBaseURL: fakeGithubAuthURL,
		},
		{
			name: "Good/full ghe url",
			envs: map[string]string{
				"PAC_INSTALLATION_ID":     "11111",
				"PAC_WORKSPACE_SECRET":    "here",
				"PAC_GIT_PROVIDER_APIURL": "https://alpha.beta.com",
			},
			wsSecretFiles: map[string]string{
				"github-application-id": "2222",
				"github-private-key":    fakePrivateKey,
			},
			resultBaseURL: fakeGithubAuthURL,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if s, ok := tt.envs["PAC_WORKSPACE_SECRET"]; ok && s == "here" {
				d := fs.NewDir(t, "workspace-secret", fs.WithFiles(tt.wsSecretFiles))
				defer d.Remove()
				tt.envs["PAC_WORKSPACE_SECRET"] = d.Path()
			}

			_, mux, url, teardown := ghtesthelper.SetupGH()
			defer teardown()
			mux.HandleFunc(fmt.Sprintf("/app/installations/%s/access_tokens", tt.envs["PAC_INSTALLATION_ID"]), func(w http.ResponseWriter, r *http.Request) {
				_, _ = fmt.Fprint(w, `{"commit": {"message": "HELLO"}}`)
			})
			tt.envs["PAC_GIT_PROVIDER_APIURL"] = fakeGithubAuthURL
			tt.envs["PAC_GIT_PROVIDER_TOKEN_APIURL"] = url + "/api/v3"

			envRemove := env.PatchAll(t, tt.envs)
			defer envRemove()

			ctx, _ := rtesting.SetupFakeContext(t)
			jeez, _ := json.Marshal(samplePRevent)
			gprovider := Provider{}
			logger := getLogger()
			event := &info.Event{
				EventType:     "pull_request",
				TriggerTarget: "pull_request",
			}
			run := &params.Run{
				Clients: clients.Clients{
					Log: logger,
				},
				Info: info.Info{
					Event: event,
					Pac:   &info.PacOpts{},
				},
			}

			_, err := gprovider.ParsePayload(ctx, run, string(jeez))
			if tt.wantErr {
				assert.Assert(t, err != nil)
				return
			}
			assert.NilError(t, err)
			if tt.nilClient {
				assert.Assert(t, gprovider.Client == nil)
				return
			}
			assert.Assert(t, gprovider.Client != nil)
			assert.Equal(t, gprovider.Client.BaseURL.String(), tt.resultBaseURL)
		})
	}
}

func TestFetchTokenGeneration(t *testing.T) {

	testNamespace := "pipelinesascode"

	ctx_nosecret, _ := rtesting.SetupFakeContext(t)
	noSecret, _ := testclient.SeedTestData(t, ctx_nosecret, testclient.Data{})

	ctx, _ := rtesting.SetupFakeContext(t)
	vaildSecret, _ := testclient.SeedTestData(t, ctx, testclient.Data{
		Secret: []*corev1.Secret{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pipelines-as-code-secret",
					Namespace: testNamespace,
				},
				Data: map[string][]byte{
					"github-application-id": []byte("12345"),
					"github-private-key":    []byte(fakePrivateKey),
				},
			},
		},
	})

	ctx_invalidAppID, _ := rtesting.SetupFakeContext(t)
	invalidAppID, _ := testclient.SeedTestData(t, ctx_invalidAppID, testclient.Data{
		Secret: []*corev1.Secret{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pipelines-as-code-secret",
					Namespace: testNamespace,
				},
				Data: map[string][]byte{
					"github-application-id": []byte("abcd"),
					"github-private-key":    []byte(fakePrivateKey),
				},
			},
		},
	})

	ctx_invalidPrivateKey, _ := rtesting.SetupFakeContext(t)
	invalidPrivateKey, _ := testclient.SeedTestData(t, ctx_invalidPrivateKey, testclient.Data{
		Secret: []*corev1.Secret{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pipelines-as-code-secret",
					Namespace: testNamespace,
				},
				Data: map[string][]byte{
					"github-application-id": []byte("12345"),
					"github-private-key":    []byte("invalid-key"),
				},
			},
		},
	})

	fakeGithubAuthURL := "https://fake.gitub.auth/api/v3/"
	tests := []struct {
		ctx           context.Context
		name          string
		wantErr       bool
		nilClient     bool
		seedData      testclient.Clients
		envs          map[string]string
		resultBaseURL string
	}{
		{
			name: "secret not found",
			ctx:  ctx_nosecret,
			envs: map[string]string{
				"SYSTEM_NAMESPACE": "foo",
			},
			seedData: noSecret,
			wantErr:  true,
		},
		{
			ctx:  ctx,
			name: "secret found",
			envs: map[string]string{
				"SYSTEM_NAMESPACE": testNamespace,
			},
			seedData:      vaildSecret,
			resultBaseURL: "https://fake.gitub.auth/api/v3/",
			nilClient:     false,
		},
		{
			ctx:  ctx_invalidAppID,
			name: "invalid app id in secret",
			envs: map[string]string{
				"SYSTEM_NAMESPACE": testNamespace,
			},
			wantErr:  true,
			seedData: invalidAppID,
		},
		{
			ctx:  ctx_invalidPrivateKey,
			name: "invalid app id in secret",
			envs: map[string]string{
				"SYSTEM_NAMESPACE": testNamespace,
			},
			wantErr:  true,
			seedData: invalidPrivateKey,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			_, mux, url, teardown := ghtesthelper.SetupGH()
			defer teardown()
			testInstallID := strconv.FormatInt(testInstallationID, 10)
			mux.HandleFunc(fmt.Sprintf("/app/installations/%s/access_tokens", string(testInstallID)), func(w http.ResponseWriter, r *http.Request) {
				_, _ = fmt.Fprint(w, `{"commit": {"message": "HELLO"}}`)
			})
			tt.envs["PAC_GIT_PROVIDER_APIURL"] = fakeGithubAuthURL
			tt.envs["PAC_GIT_PROVIDER_TOKEN_APIURL"] = url + "/api/v3"

			envRemove := env.PatchAll(t, tt.envs)
			defer envRemove()

			// adding installation id to event to enforce client creation
			samplePRevent.Installation = &github.Installation{
				ID: &testInstallationID,
			}

			jeez, _ := json.Marshal(samplePRevent)
			gprovider := Provider{}
			logger := getLogger()
			event := &info.Event{
				EventType:     "pull_request",
				TriggerTarget: "pull_request",
			}
			run := &params.Run{
				Clients: clients.Clients{
					Log:  logger,
					Kube: tt.seedData.Kube,
				},
				Info: info.Info{
					Event: event,
					Pac:   &info.PacOpts{},
				},
			}

			_, err := gprovider.ParseEventPayload(tt.ctx, run, string(jeez))
			if tt.wantErr {
				assert.Assert(t, err != nil)
				return
			}
			assert.NilError(t, err)
			if tt.nilClient {
				assert.Assert(t, gprovider.Client == nil)
				return
			}
			assert.Assert(t, gprovider.Client != nil)
			assert.Equal(t, gprovider.Client.BaseURL.String(), tt.resultBaseURL)
		})
	}
}
