package pipelineascode

import (
	"strings"
	"testing"

	"github.com/google/go-github/v56/github"
	apipac "github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/keys"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/consoleui"
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
			name:                 "old secrets",
			tmpl:                 `secretName: "pac-git-basic-auth-{{repo_owner}}-{{repo_name}}"`,
			upgradeMessageSubstr: "old basic auth secret name",
			needupdate:           true,
		},
		{
			name:       "no need",
			tmpl:       ` secretName: "foo-bar-foo"`,
			needupdate: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewPacs(nil, nil, &params.Run{Clients: clients.Clients{}}, nil, nil)
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
	assert.Equal(t, prs[0].GetName(), ret[0].GetName())
	ret = filterRunningPipelineRunOnTargetTest(testPipeline, prs)
	assert.Equal(t, prs[0].GetName(), ret[0].GetName())
	prs = []*tektonv1.PipelineRun{}
	ret = filterRunningPipelineRunOnTargetTest(testPipeline, prs)
	assert.Assert(t, ret == nil)
}

func TestGetPipelineRunsFromRepo(t *testing.T) {
	tests := []struct {
		name                  string
		repositories          *v1alpha1.Repository
		tektondir             string
		expectedNumberOfPruns int
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
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			observer, _ := zapobserver.New(zap.InfoLevel)
			logger := zap.New(observer).Sugar()
			ctx, _ := rtesting.SetupFakeContext(t)
			fakeclient, mux, _, teardown := ghtesthelper.SetupGH()
			defer teardown()

			runevent := &info.Event{
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
			if tt.tektondir != "" {
				ghtesthelper.SetupGitTree(t, mux, tt.tektondir, runevent, false)
			}

			stdata, _ := testclient.SeedTestData(t, ctx, testclient.Data{})
			cs := &params.Run{
				Clients: clients.Clients{
					PipelineAsCode: stdata.PipelineAsCode,
					Log:            logger,
					Kube:           stdata.Kube,
					Tekton:         stdata.Pipeline,
					ConsoleUI:      consoleui.FallBackConsole{},
				},
				Info: info.Info{
					Pac: &info.PacOpts{
						Settings: &settings.Settings{
							SecretAutoCreation: true,
							RemoteTasks:        true,
						},
					},
				},
			}
			k8int := &kitesthelper.KinterfaceTest{
				ConsoleURL: "https://console.url",
			}
			vcx := &ghprovider.Provider{
				Client: fakeclient,
				Token:  github.String("None"),
				Logger: logger,
			}
			p := NewPacs(runevent, vcx, cs, k8int, logger)
			matchedPRs, err := p.getPipelineRunsFromRepo(ctx, tt.repositories)
			assert.NilError(t, err)
			matchedPRNames := []string{}
			for i := range matchedPRs {
				matchedPRNames = append(matchedPRNames, matchedPRs[i].PipelineRun.GetGenerateName())
			}
			assert.Equal(t, len(matchedPRNames), tt.expectedNumberOfPruns)
		})
	}
}
