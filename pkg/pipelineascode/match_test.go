package pipelineascode

import (
	"strings"
	"testing"

	"github.com/google/go-github/v69/github"
	apipac "github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/keys"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/consoleui"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/events"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/clients"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/settings"
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

func TestChangeSecret(t *testing.T) {
	prs := []*tektonv1.PipelineRun{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "{{git_auth_secret}}",
			},
		},
	}
	err := changeSecret(prs)
	assert.NilError(t, err)
	assert.Assert(t, strings.HasPrefix(prs[0].GetName(), "pac-gitauth"), prs[0].GetName(), "has no pac-gitauth prefix")
	assert.Assert(t, prs[0].GetAnnotations()[apipac.GitAuthSecret] != "")
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
			logSnippet: `prun: bad-tekton-yaml tekton validation error: json: cannot unmarshal object into Go struct field PipelineSpec.spec.pipelineSpec.tasks of type []v1beta1.PipelineTask`,
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
				Client: fakeclient,
				Token:  github.Ptr("None"),
				Logger: logger,
			}
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
