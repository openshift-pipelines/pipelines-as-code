package pipelineascode

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/google/go-github/v71/github"
	apipac "github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/keys"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/consoleui"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/events"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/opscomments"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/clients"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/settings"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/triggertype"
	ghprovider "github.com/openshift-pipelines/pipelines-as-code/pkg/provider/github"
	testclient "github.com/openshift-pipelines/pipelines-as-code/pkg/test/clients"
	ghtesthelper "github.com/openshift-pipelines/pipelines-as-code/pkg/test/github"
	kitesthelper "github.com/openshift-pipelines/pipelines-as-code/pkg/test/kubernetestint"
	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"go.uber.org/zap"
	zapobserver "go.uber.org/zap/zaptest/observer"
	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	rtesting "knative.dev/pkg/reconciler/testing"
)

func TestPacRun_checkNeedUpdate(t *testing.T) {
	tests := []struct {
		name                 string
		tmpl                 string
		upgradeMessageSubstr string
		needupdate           bool
	}{
		{
			name:       "no need",
			tmpl:       ` secretName: "foo-bar-foo"`,
			needupdate: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewPacs(nil, nil, &params.Run{Clients: clients.Clients{}}, &info.PacOpts{}, nil, nil, nil)
			got, needupdate := p.checkNeedUpdate(tt.tmpl)
			if tt.upgradeMessageSubstr != "" {
				assert.Assert(t, strings.Contains(got, tt.upgradeMessageSubstr))
			}
			assert.Assert(t, needupdate == tt.needupdate)
		})
	}
}

func TestChangePipelineRun(t *testing.T) {
	repo := &v1alpha1.Repository{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "testrepo",
			Namespace: "test",
		},
	}

	prs := []*tektonv1.PipelineRun{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "{{git_auth_secret}}",
				Namespace: "{{ repo_name }}",
			},
		},
	}

	jsonErrorPRs := []*tektonv1.PipelineRun{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "json-error-pr",
				Namespace: "test",
			},
			Spec: tektonv1.PipelineRunSpec{
				Params: []tektonv1.Param{
					{
						Name: "my-param",
						Value: tektonv1.ParamValue{
							// We are intentionally leaving this empty:
							// Type: tektonv1.ParamTypeString,
							StringVal: "some-value",
						},
					},
				},
			},
		},
	}

	tests := []struct {
		name          string
		prs           []*tektonv1.PipelineRun
		expectedError string
	}{
		{
			name: "test with params",
			prs:  prs,
		},
		{
			name:          "test with json error",
			prs:           jsonErrorPRs,
			expectedError: "failed to marshal PipelineRun json-error-pr: json: error calling MarshalJSON for type v1.ParamValue: impossible ParamValues.Type: \"\"",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := info.NewEvent()
			event.Repository = "testrepo"
			p := &PacRun{event: event}
			ctx, _ := rtesting.SetupFakeContext(t)
			err := p.changePipelineRun(ctx, repo, tt.prs)
			if tt.expectedError != "" {
				assert.Error(t, err, tt.expectedError)
				return
			}
			assert.Assert(t, strings.HasPrefix(tt.prs[0].GetName(), "pac-gitauth"), tt.prs[0].GetName(), "has no pac-gitauth prefix")
			assert.Assert(t, tt.prs[0].GetAnnotations()[apipac.GitAuthSecret] != "")
			assert.Assert(t, tt.prs[0].GetNamespace() == "testrepo", "namespace should be testrepo: %v", tt.prs[0].GetNamespace())
		})
	}
}

func TestFilterRunningPipelineRunOnTargetTest(t *testing.T) {
	testPipeline := "test"
	prs := []*tektonv1.PipelineRun{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "pipelinerun-" + testPipeline,
				Annotations: map[string]string{
					apipac.OriginalPRName: testPipeline,
				},
			},
		},
	}
	ret := filterRunningPipelineRunOnTargetTest("", prs)
	assert.Assert(t, ret == nil)
	ret = filterRunningPipelineRunOnTargetTest(testPipeline, prs)
	assert.Equal(t, prs[0].GetName(), ret.GetName())
	prs = []*tektonv1.PipelineRun{}
	ret = filterRunningPipelineRunOnTargetTest(testPipeline, prs)
	assert.Assert(t, ret == nil)
}

func TestGetPipelineRunsFromRepo(t *testing.T) {
	pullRequestEvent := &info.Event{
		SHA:           "principale",
		Organization:  "organizationes",
		Repository:    "lagaffe",
		URL:           "https://service/documentation",
		HeadBranch:    "main",
		BaseBranch:    "main",
		Sender:        "fantasio",
		EventType:     "pull_request",
		TriggerTarget: "pull_request",
	}
	okToTestEvent := &info.Event{
		SHA:           "principale",
		Organization:  "organizationes",
		Repository:    "lagaffe",
		URL:           "https://service/documentation",
		HeadBranch:    "main",
		BaseBranch:    "main",
		Sender:        "fantasio",
		EventType:     "ok-to-test-comment",
		TriggerTarget: "pull_request",
	}
	testExplicitNoMatchPREvent := &info.Event{
		SHA:           "principale",
		Organization:  "organizationes",
		Repository:    "lagaffe",
		URL:           "https://service/documentation",
		HeadBranch:    "main",
		BaseBranch:    "main",
		Sender:        "fantasio",
		TriggerTarget: "pull_request",
		State: info.State{
			TargetTestPipelineRun: "no-match",
		},
	}

	tests := []struct {
		name                  string
		repositories          *v1alpha1.Repository
		tektondir             string
		expectedNumberOfPruns int
		event                 *info.Event
		logSnippet            string
	}{
		{
			name: "more than one pipelinerun in .tekton dir",
			repositories: &v1alpha1.Repository{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "testrepo",
					Namespace: "test",
				},
				Spec: v1alpha1.RepositorySpec{},
			},
			tektondir:             "testdata/pull_request_multiplepipelineruns",
			expectedNumberOfPruns: 2,
			event:                 pullRequestEvent,
		},
		{
			name: "single pipelinerun in .tekton dir",
			repositories: &v1alpha1.Repository{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "testrepo",
					Namespace: "test",
				},
				Spec: v1alpha1.RepositorySpec{},
			},
			tektondir:             "testdata/pull_request",
			expectedNumberOfPruns: 1,
			event:                 pullRequestEvent,
		},
		{
			name: "invalid tekton pipelineruns in directory",
			repositories: &v1alpha1.Repository{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "testrepo",
					Namespace: "test",
				},
				Spec: v1alpha1.RepositorySpec{},
			},
			tektondir:  "testdata/invalid_tekton_yaml",
			event:      pullRequestEvent,
			logSnippet: `json: cannot unmarshal object into Go struct field PipelineSpec.spec.pipelineSpec.tasks of type []v1beta1.PipelineTask`,
		},
		{
			name: "no-match pipelineruns in .tekton dir, only matched should be returned",
			repositories: &v1alpha1.Repository{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "testrepo",
					Namespace: "test",
				},
				Spec: v1alpha1.RepositorySpec{},
			},
			// we have 3 PR in there 2 that has a match on pull request and 1 that is a no-matching
			// matching those two that is matching here
			tektondir:             "testdata/no-match",
			expectedNumberOfPruns: 2,
			event:                 pullRequestEvent,
		},
		{
			name: "no-match pipelineruns in .tekton dir, only match the no-match",
			repositories: &v1alpha1.Repository{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "testrepo",
					Namespace: "test",
				},
				Spec: v1alpha1.RepositorySpec{},
			},
			// we have 3 PR in there 2 that has a match on pull request and 1 that is a no-matching
			// matching that only one here
			tektondir:             "testdata/no-match",
			expectedNumberOfPruns: 1,
			event:                 testExplicitNoMatchPREvent,
		},
		{
			name: "no-match pipelineruns in .tekton dir, on ok-to-test command for an external user",
			repositories: &v1alpha1.Repository{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "testrepo",
					Namespace: "test",
				},
				Spec: v1alpha1.RepositorySpec{},
			},
			// if `testdata/no_yaml` dir is supplied here p.getPipelineRunsFromRepo func will return after
			// GetTektonDir so providing `testdat/push_branch` so that it should call MatchPipelineRunsByAnnotation
			// first and then create a neutral check-run.
			tektondir:             "testdata/push_branch",
			expectedNumberOfPruns: 0,
			event:                 okToTestEvent,
		},
		{
			name: "no .tekton dir in repository",
			repositories: &v1alpha1.Repository{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "testrepo",
					Namespace: "test",
				},
				Spec: v1alpha1.RepositorySpec{},
			},
			tektondir:             "testdata/no_tekton_dir",
			expectedNumberOfPruns: 0,
			event:                 okToTestEvent,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			observerCore, logCatcher := zapobserver.New(zap.InfoLevel)
			logger := zap.New(observerCore).Sugar()
			ctx, _ := rtesting.SetupFakeContext(t)
			fakeclient, mux, _, teardown := ghtesthelper.SetupGH()
			defer teardown()

			if tt.tektondir != "" {
				ghtesthelper.SetupGitTree(t, mux, tt.tektondir, tt.event, false)
			}

			stdata, _ := testclient.SeedTestData(t, ctx, testclient.Data{})
			cs := &params.Run{
				Clients: clients.Clients{
					PipelineAsCode: stdata.PipelineAsCode,
					Log:            logger,
					Kube:           stdata.Kube,
					Tekton:         stdata.Pipeline,
				},
			}
			cs.Clients.SetConsoleUI(consoleui.FallBackConsole{})
			k8int := &kitesthelper.KinterfaceTest{
				ConsoleURL: "https://console.url",
			}
			vcx := &ghprovider.Provider{
				Token:  github.Ptr("None"),
				Logger: logger,
			}
			vcx.SetGithubClient(fakeclient)
			pacInfo := &info.PacOpts{
				Settings: settings.Settings{
					ApplicationName:    "Pipelines as Code CI",
					SecretAutoCreation: true,
					RemoteTasks:        true,
				},
			}
			vcx.SetPacInfo(pacInfo)
			p := NewPacs(tt.event, vcx, cs, pacInfo, k8int, logger, nil)
			p.eventEmitter = events.NewEventEmitter(stdata.Kube, logger)
			matchedPRs, err := p.getPipelineRunsFromRepo(ctx, tt.repositories)
			assert.NilError(t, err)
			matchedPRNames := []string{}
			for i := range matchedPRs {
				matchedPRNames = append(matchedPRNames, matchedPRs[i].PipelineRun.GetGenerateName())
			}
			if tt.logSnippet != "" {
				assert.Assert(t, logCatcher.FilterMessageSnippet(tt.logSnippet).Len() > 0, logCatcher.All())
			}
			assert.Equal(t, len(matchedPRNames), tt.expectedNumberOfPruns)
		})
	}
}

func TestVerifyRepoAndUser(t *testing.T) {
	observerCore, _ := zapobserver.New(zap.InfoLevel)
	logger := zap.New(observerCore).Sugar()

	payload := []byte(`{"key": "value"}`)
	mac := hmac.New(sha256.New, []byte("secret"))
	mac.Write(payload)
	sha256secret := hex.EncodeToString(mac.Sum(nil))

	header := make(http.Header)
	header.Set(github.SHA256SignatureHeader, fmt.Sprintf("sha256=%s", sha256secret))

	request := &info.Request{
		Header:  header,
		Payload: payload,
	}

	tests := []struct {
		name          string
		runevent      info.Event
		repositories  []*v1alpha1.Repository
		webhookSecret string
		wantRepoNil   bool
		wantErr       bool
		wantErrMsg    string
	}{
		{
			name: "no repository match",
			runevent: info.Event{
				Organization:  "owner",
				Repository:    "repo",
				URL:           "https://example.com/owner/repo",
				SHA:           "123abc",
				EventType:     triggertype.PullRequest.String(),
				TriggerTarget: triggertype.PullRequest,
			},
			wantRepoNil: true,
			wantErr:     false,
		},
		{
			name: "missing git_provider section",
			runevent: info.Event{
				Organization:  "owner",
				Repository:    "repo",
				URL:           "https://example.com/owner/repo",
				SHA:           "123abc",
				EventType:     triggertype.PullRequest.String(),
				TriggerTarget: triggertype.PullRequest,
			},
			repositories: []*v1alpha1.Repository{{
				ObjectMeta: metav1.ObjectMeta{Name: "repo", Namespace: "ns"},
				Spec:       v1alpha1.RepositorySpec{URL: "https://example.com/owner/repo"},
			}},
			wantRepoNil: false,
			wantErr:     true,
			wantErrMsg:  "cannot get secret from repository: failed to find git_provider details in repository spec: ns/repo",
		},
		{
			name: "webhook validation failure",
			runevent: info.Event{
				Organization:   "owner",
				Repository:     "repo",
				URL:            "https://example.com/owner/repo",
				SHA:            "123abc",
				EventType:      triggertype.PullRequest.String(),
				TriggerTarget:  triggertype.PullRequest,
				InstallationID: 1,
				Request:        &info.Request{Header: http.Header{}},
			},
			repositories: []*v1alpha1.Repository{{
				ObjectMeta: metav1.ObjectMeta{Name: "repo", Namespace: "ns"},
				Spec:       v1alpha1.RepositorySpec{URL: "https://example.com/owner/repo"},
			}},
			wantRepoNil: false,
			wantErr:     true,
			wantErrMsg:  "could not validate payload, check your webhook secret?: no signature has been detected, for security reason we are not allowing webhooks that has no secret",
		},
		{
			name: "webhook secret is not set",
			runevent: info.Event{
				Organization:   "owner",
				Repository:     "repo",
				URL:            "https://example.com/owner/repo",
				SHA:            "123abc",
				EventType:      triggertype.PullRequest.String(),
				TriggerTarget:  triggertype.PullRequest,
				InstallationID: 1,
				Request:        request,
			},
			repositories: []*v1alpha1.Repository{{
				ObjectMeta: metav1.ObjectMeta{Name: "repo", Namespace: "ns"},
				Spec:       v1alpha1.RepositorySpec{URL: "https://example.com/owner/repo"},
			}},
			wantRepoNil: false,
			wantErr:     true,
			wantErrMsg:  "could not validate payload, check your webhook secret?: no webhook secret has been set, in repository CR or secret",
		},
		{
			name: "permission denied push comment",
			runevent: info.Event{
				Organization:   "owner",
				Repository:     "repo",
				URL:            "https://example.com/owner/repo",
				SHA:            "123abc",
				EventType:      opscomments.TestAllCommentEventType.String(),
				TriggerTarget:  triggertype.Push,
				Sender:         "intruder",
				InstallationID: 1,
				Request:        request,
			},
			repositories: []*v1alpha1.Repository{{
				ObjectMeta: metav1.ObjectMeta{Name: "repo", Namespace: "ns"},
				Spec:       v1alpha1.RepositorySpec{URL: "https://example.com/owner/repo"},
			}},
			webhookSecret: "secret",
			wantRepoNil:   true,
			wantErr:       true,
			wantErrMsg:    "failed to run create status, user is not allowed to run the CI",
		},
		{
			name: "permission denied pull_request comment pending approval",
			runevent: info.Event{
				Organization:   "owner",
				Repository:     "repo",
				URL:            "https://example.com/owner/repo",
				SHA:            "123abc",
				EventType:      triggertype.PullRequest.String(),
				TriggerTarget:  triggertype.PullRequest,
				Sender:         "outsider",
				InstallationID: 1,
				Request:        request,
			},
			repositories: []*v1alpha1.Repository{{
				ObjectMeta: metav1.ObjectMeta{Name: "repo", Namespace: "ns"},
				Spec:       v1alpha1.RepositorySpec{URL: "https://example.com/owner/repo"},
			}},
			webhookSecret: "secret",
			wantRepoNil:   true,
			wantErr:       true,
			wantErrMsg:    "failed to run create status, user is not allowed to run the CI",
		},
		{
			name: "commit not found",
			runevent: info.Event{
				Organization:   "owner",
				Repository:     "repo",
				URL:            "https://example.com/owner/repo",
				SHA:            "",
				EventType:      triggertype.PullRequest.String(),
				TriggerTarget:  triggertype.PullRequest,
				InstallationID: 1,
				Sender:         "owner",
				Request:        request,
			},
			repositories: []*v1alpha1.Repository{{
				ObjectMeta: metav1.ObjectMeta{Name: "repo", Namespace: "ns"},
				Spec:       v1alpha1.RepositorySpec{URL: "https://example.com/owner/repo"},
			}},
			webhookSecret: "secret",
			wantRepoNil:   false,
			wantErr:       true,
			wantErrMsg:    "could not find commit info",
		},
		{
			name: "happy path",
			runevent: info.Event{
				Organization:   "owner",
				Repository:     "repo",
				URL:            "https://example.com/owner/repo",
				SHA:            "123abc",
				EventType:      triggertype.PullRequest.String(),
				TriggerTarget:  triggertype.PullRequest,
				InstallationID: 1,
				Sender:         "owner",
				Request:        request,
			},
			repositories: []*v1alpha1.Repository{{
				ObjectMeta: metav1.ObjectMeta{Name: "repo", Namespace: "ns"},
				Spec:       v1alpha1.RepositorySpec{URL: "https://example.com/owner/repo"},
			}},
			webhookSecret: "secret",
			wantRepoNil:   false,
			wantErr:       false,
		},
	}

	pacInfo := &info.PacOpts{Settings: settings.DefaultSettings()}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			baseCtx, _ := rtesting.SetupFakeContext(t)
			ctx := info.StoreNS(baseCtx, "pac")

			ghClient, mux, _, teardown := ghtesthelper.SetupGH()
			defer teardown()

			// commit endpoint
			commitPath := fmt.Sprintf("/repos/%s/%s/git/commits/%s", tt.runevent.Organization, tt.runevent.Repository, tt.runevent.SHA)
			mux.HandleFunc(commitPath, func(rw http.ResponseWriter, _ *http.Request) {
				if tt.runevent.SHA == "" {
					rw.WriteHeader(http.StatusNotFound)
					return
				}
				fmt.Fprint(rw, `{"sha":"123abc","html_url":"https://example.com/commit/123abc","message":"msg"}`)
			})

			// org members empty
			mux.HandleFunc(fmt.Sprintf("/orgs/%s/members", tt.runevent.Organization), func(rw http.ResponseWriter, _ *http.Request) { fmt.Fprint(rw, `[]`) })
			// collaborator check – return 404 for non‐collaborator sender when defined
			if tt.runevent.Sender != "" && tt.runevent.Sender != tt.runevent.Organization {
				mux.HandleFunc(
					fmt.Sprintf("/repos/%s/%s/collaborators/%s", tt.runevent.Organization, tt.runevent.Repository, tt.runevent.Sender),
					func(rw http.ResponseWriter, _ *http.Request) { rw.WriteHeader(http.StatusNotFound) },
				)
			}
			// status endpoint stub (used when CreateStatus is called)
			mux.HandleFunc(
				fmt.Sprintf("/repos/%s/%s/statuses/%s", tt.runevent.Organization, tt.runevent.Repository, tt.runevent.SHA),
				func(rw http.ResponseWriter, _ *http.Request) { fmt.Fprint(rw, `{}`) },
			)

			vcx := &ghprovider.Provider{Token: github.Ptr("token"), Logger: logger}
			vcx.SetGithubClient(ghClient)
			vcx.SetPacInfo(pacInfo)

			k8int := &kitesthelper.KinterfaceTest{GetSecretResult: map[string]string{"pipelines-as-code-secret": tt.webhookSecret}}

			stdata, _ := testclient.SeedTestData(t, ctx, testclient.Data{Repositories: tt.repositories /*Secret: []*corev1.Secret{secret}*/})
			in := info.NewInfo()
			cs := &params.Run{
				Info: in,
				Clients: clients.Clients{
					PipelineAsCode: stdata.PipelineAsCode,
					Kube:           stdata.Kube,
					Tekton:         stdata.Pipeline,
					Log:            logger,
				},
			}
			cs.Clients.SetConsoleUI(consoleui.FallBackConsole{})

			ev := tt.runevent
			ev.Provider = &info.Provider{Token: "token", WebhookSecret: tt.webhookSecret}

			p := NewPacs(&ev, vcx, cs, pacInfo, k8int, logger, nil)
			repo, err := p.verifyRepoAndUser(ctx)
			assert.Assert(t, (err != nil) == tt.wantErr)

			if tt.wantErr {
				assert.ErrorContains(t, err, tt.wantErrMsg)
			}

			if tt.wantRepoNil {
				assert.Assert(t, repo == nil)
			} else {
				assert.Assert(t, repo != nil)
			}
		})
	}
}
