package pipelineascode

import (
	"strings"
	"testing"

	apipac "github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/keys"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/clients"
	tektonv1beta1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	prs := []*tektonv1beta1.PipelineRun{
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
	prs := []*tektonv1beta1.PipelineRun{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "pipelinerun-" + testPipeline,
				Labels: map[string]string{
					apipac.OriginalPRName: testPipeline,
				},
			},
		},
	}
	ret := filterRunningPipelineRunOnTargetTest("", prs)
	assert.Equal(t, prs[0].GetName(), ret[0].GetName())
	ret = filterRunningPipelineRunOnTargetTest(testPipeline, prs)
	assert.Equal(t, prs[0].GetName(), ret[0].GetName())
	prs = []*tektonv1beta1.PipelineRun{}
	ret = filterRunningPipelineRunOnTargetTest(testPipeline, prs)
	assert.Assert(t, ret == nil)
}
