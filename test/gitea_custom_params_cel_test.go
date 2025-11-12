//go:build e2e

package test

import (
	"context"
	"fmt"
	"regexp"
	"testing"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/triggertype"
	tgitea "github.com/openshift-pipelines/pipelines-as-code/test/pkg/gitea"
	twait "github.com/openshift-pipelines/pipelines-as-code/test/pkg/wait"
	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TestGiteaCustomParamsInCELExpression tests that custom parameters from Repository CR
// can be used directly in CEL expressions for matching PipelineRuns. This test verifies:
// - Custom params work in CEL expressions (enable_ci and environment)
// - Only PipelineRuns with matching CEL expressions are triggered
// - PipelineRuns with non-matching custom params are skipped (enable_ci == "false").
// - Custom params are available for template expansion in the PipelineRun.
func TestGiteaCustomParamsInCELExpression(t *testing.T) {
	topts := &tgitea.TestOpts{
		Regexp:      successRegexp,
		TargetEvent: triggertype.PullRequest.String(),
		YAMLFiles: map[string]string{
			".tekton/pr.yaml":      "testdata/pipelinerun-cel-custom-params-pullrequest.yaml",
			".tekton/nomatch.yaml": "testdata/pipelinerun-cel-custom-params-nomatch.yaml",
		},
		CheckForStatus: "success",
		ExpectEvents:   false,
		RepoCRParams: &[]v1alpha1.Params{
			{
				Name:  "enable_ci",
				Value: "true",
			},
			{
				Name:  "environment",
				Value: "staging",
			},
		},
	}
	_, f := tgitea.TestPR(t, topts)
	defer f()

	// List PipelineRuns to verify only one was created
	// The pr.yaml should match (event == "pull_request" && enable_ci == "true" && environment != "")
	// The nomatch.yaml should NOT match (enable_ci == "false" && environment == "production")
	prs, err := topts.ParamsRun.Clients.Tekton.TektonV1().PipelineRuns(topts.TargetNS).List(
		context.Background(), metav1.ListOptions{})
	assert.NilError(t, err)
	assert.Equal(t, len(prs.Items), 1, "Expected exactly 1 PipelineRun to match (nomatch.yaml should be skipped)")

	// Verify the output contains the custom params expanded via template
	output := `Custom params: enable_ci=true environment=staging`
	err = twait.RegexpMatchingInPodLog(context.Background(), topts.ParamsRun, topts.TargetNS,
		fmt.Sprintf("tekton.dev/pipelineRun=%s,tekton.dev/pipelineTask=custom-params-cel-test", prs.Items[0].Name),
		"step-test-custom-params-cel", *regexp.MustCompile(output), "", 2)
	assert.NilError(t, err)
}
