package github

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"testing"

	"github.com/google/go-github/v39/github"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/clients"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	ghtesthelper "github.com/openshift-pipelines/pipelines-as-code/pkg/test/github"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/env"
	"gotest.tools/v3/fs"
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

	gvcs := VCS{
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
	_, err = gvcs.ParsePayload(ctx, run, string(b))
	assert.NilError(t, err)

	// would bomb out on "assertion failed: error is not nil: invalid character
	// '\n' in string literal" if we don't fix the payload
	assert.NilError(t, err)
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
			triggerTarget: "issue-recheck",
			wantErrString: "404",
			payloadEventStruct: github.CheckRunEvent{
				Repo: sampleRepo,
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
				Repo: sampleRepo,
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
				Repo: sampleRepo,
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
			gvcs := VCS{
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
			ret, err := gvcs.ParsePayload(ctx, run, jeez)
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
				"noapplication_id": "Foo",
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
				"application_id": "BAD",
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
				"application_id": "2222",
			},
			wantErr: true,
		},
		{
			name: "Bad/Private Key chelou",
			envs: map[string]string{
				"PAC_INSTALLATION_ID":  "11111",
				"PAC_WORKSPACE_SECRET": "here",
				"PAC_WEBVCS_APIURL":    "foo.bar.com",
			},
			wsSecretFiles: map[string]string{
				"application_id": "2222",
				"private.key":    "hello",
			},
			wantErr: true,
		},
		{
			name: "Good/ghe base domain",
			envs: map[string]string{
				"PAC_INSTALLATION_ID":  "11111",
				"PAC_WORKSPACE_SECRET": "here",
				"PAC_WEBVCS_APIURL":    "foo.bar.com",
			},
			wsSecretFiles: map[string]string{
				"application_id": "2222",
				"private.key":    fakePrivateKey,
			},
			resultBaseURL: "https://foo.bar.com/api/v3/",
		},
		{
			name: "Good/full ghe url",
			envs: map[string]string{
				"PAC_INSTALLATION_ID":  "11111",
				"PAC_WORKSPACE_SECRET": "here",
				"PAC_WEBVCS_APIURL":    "https://alpha.beta.com",
			},
			wsSecretFiles: map[string]string{
				"application_id": "2222",
				"private.key":    fakePrivateKey,
			},
			resultBaseURL: "https://alpha.beta.com/api/v3/",
		},
		{
			name: "Good/default github url",
			envs: map[string]string{
				"PAC_INSTALLATION_ID":  "11111",
				"PAC_WORKSPACE_SECRET": "here",
			},
			wsSecretFiles: map[string]string{
				"application_id": "2222",
				"private.key":    fakePrivateKey,
			},
			resultBaseURL: "https://api.github.com/",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if s, ok := tt.envs["PAC_WORKSPACE_SECRET"]; ok && s == "here" {
				d := fs.NewDir(t, "workspace-secret", fs.WithFiles(tt.wsSecretFiles))
				defer d.Remove()
				tt.envs["PAC_WORKSPACE_SECRET"] = d.Path()
			}
			envRemove := env.PatchAll(t, tt.envs)
			defer envRemove()

			ctx, _ := rtesting.SetupFakeContext(t)
			jeez, _ := json.Marshal(samplePRevent)
			gvcs := VCS{}
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

			_, err := gvcs.ParsePayload(ctx, run, string(jeez))
			if tt.wantErr {
				assert.Assert(t, err != nil)
				return
			}
			assert.NilError(t, err)
			if tt.nilClient {
				assert.Assert(t, gvcs.Client == nil)
				return
			}
			assert.Assert(t, gvcs.Client != nil)
			assert.Equal(t, gvcs.Client.BaseURL.String(), tt.resultBaseURL)
		})
	}
}
