package customparams

import (
	"context"
	"fmt"
	"strings"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/formatting"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider"
	"go.uber.org/zap"
)

func (p *CustomParams) getChangedFiles(ctx context.Context) provider.ChangedFiles {
	if p.vcx == nil {
		return provider.ChangedFiles{}
	}
	changedFiles, err := p.vcx.GetFiles(ctx, p.event)
	if err != nil {
		p.eventEmitter.EmitMessage(p.repo, zap.ErrorLevel, "ParamsError", fmt.Sprintf("error getting changed files: %s", err.Error()))
	}

	return changedFiles
}

// makeStandardParamsFromEvent will create a map of standard params out of the event
func (p *CustomParams) makeStandardParamsFromEvent(ctx context.Context) map[string]string {
	repoURL := p.event.URL
	// On bitbucket server you are have a special url for checking it out, they
	// seemed to fix it in 2.0 but i guess we have to live with this until then.
	if p.event.CloneURL != "" {
		repoURL = p.event.CloneURL
	}
	changedFiles := p.getChangedFiles(ctx)
	return map[string]string{
		"revision":          p.event.SHA,
		"repo_url":          repoURL,
		"repo_owner":        strings.ToLower(p.event.Organization),
		"repo_name":         strings.ToLower(p.event.Repository),
		"target_branch":     formatting.SanitizeBranch(p.event.BaseBranch),
		"source_branch":     formatting.SanitizeBranch(p.event.HeadBranch),
		"sender":            strings.ToLower(p.event.Sender),
		"target_namespace":  p.repo.GetNamespace(),
		"event_type":        p.event.EventType,
		"all_changed_files": strings.Join(formatting.UniqueStringArray(changedFiles.All), ","),
		"added_files":       strings.Join(formatting.UniqueStringArray(changedFiles.Added), ","),
		"deleted_files":     strings.Join(formatting.UniqueStringArray(changedFiles.Deleted), ","),
		"modified_files":    strings.Join(formatting.UniqueStringArray(changedFiles.Modified), ","),
		"renamed_files":     strings.Join(formatting.UniqueStringArray(changedFiles.Renamed), ","),
	}
}
