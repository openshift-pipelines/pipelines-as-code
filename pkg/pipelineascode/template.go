package pipelineascode

import (
	"context"
	"fmt"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/customparams"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/templates"
	"go.uber.org/zap"
)

// makeTemplate will process all templates replacing the value from the event and from the
// params as set on Repo CR
func (p *PacRun) makeTemplate(ctx context.Context, repo *v1alpha1.Repository, template string) string {
	cp := customparams.NewCustomParams(p.event, repo, p.run, p.k8int, p.eventEmitter)
	maptemplate, err := cp.GetParams(ctx)
	if err != nil {
		p.eventEmitter.EmitMessage(repo, zap.ErrorLevel, "ParamsError",
			fmt.Sprintf("error processing repository CR custom params: %s", err.Error()))
	}

	// convert pull request number to string
	if p.event.PullRequestNumber != 0 {
		maptemplate["pull_request_number"] = fmt.Sprintf("%d", p.event.PullRequestNumber)
	}

	return templates.ReplacePlaceHoldersVariables(template, maptemplate)
}
