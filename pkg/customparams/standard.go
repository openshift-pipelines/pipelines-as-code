package customparams

import (
	"context"
	"fmt"
	"strings"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/formatting"
	"go.uber.org/zap"
)

func (p *CustomParams) getFilesAsCommaSeparated(ctx context.Context) string {
	if p.vcx == nil {
		return ""
	}
	filechanges, err := p.vcx.GetFiles(ctx, p.event)
	if err != nil {
		p.eventEmitter.EmitMessage(p.repo, zap.ErrorLevel, "ParamsError", fmt.Sprintf("error getting changed files: %s", err.Error()))
		return ""
	}
	return strings.Join(filechanges, ",")
}

// makeStandardParamsFromEvent will create a map of standard params out of the event
func (p *CustomParams) makeStandardParamsFromEvent(ctx context.Context) map[string]string {
	repoURL := p.event.URL
	// On bitbucket server you are have a special url for checking it out, they
	// seemed to fix it in 2.0 but i guess we have to live with this until then.
	if p.event.CloneURL != "" {
		repoURL = p.event.CloneURL
	}

	return map[string]string{
		"revision":         p.event.SHA,
		"repo_url":         repoURL,
		"repo_owner":       strings.ToLower(p.event.Organization),
		"repo_name":        strings.ToLower(p.event.Repository),
		"target_branch":    formatting.SanitizeBranch(p.event.BaseBranch),
		"source_branch":    formatting.SanitizeBranch(p.event.HeadBranch),
		"sender":           strings.ToLower(p.event.Sender),
		"target_namespace": p.repo.GetNamespace(),
		"event_type":       p.event.EventType,
		"change_files":     p.getFilesAsCommaSeparated(ctx),
	}
}
