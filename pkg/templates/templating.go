package templates

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/formatting"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
)

var reTemplate = regexp.MustCompile(`{{([^}]{2,})}}`)

// ReplacePlaceHoldersVariables Replace those {{var}} placeholders to the runinfo variable
func ReplacePlaceHoldersVariables(template string, dico map[string]string) string {
	return reTemplate.ReplaceAllStringFunc(template, func(s string) string {
		parts := reTemplate.FindStringSubmatch(s)
		key := strings.TrimSpace(parts[1])
		if _, ok := dico[key]; !ok {
			return s
		}
		return dico[key]
	})
}

// Process process all templates replacing
func Process(event *info.Event, repo *v1alpha1.Repository, template string) string {
	repoURL := event.URL
	// On bitbucket server you are have a special url for checking it out, they
	// seemed to fix it in 2.0 but i guess we have to live with this until then.
	if event.CloneURL != "" {
		repoURL = event.CloneURL
	}

	maptemplate := map[string]string{
		"revision":         event.SHA,
		"repo_url":         repoURL,
		"repo_owner":       strings.ToLower(event.Organization),
		"repo_name":        strings.ToLower(event.Repository),
		"target_branch":    formatting.SanitizeBranch(event.BaseBranch),
		"source_branch":    formatting.SanitizeBranch(event.HeadBranch),
		"sender":           strings.ToLower(event.Sender),
		"target_namespace": repo.GetNamespace(),
	}
	// we don't want to get a 0 replaced
	if event.PullRequestNumber != 0 {
		maptemplate["pull_request_number"] = fmt.Sprintf("%d", event.PullRequestNumber)
	}
	return ReplacePlaceHoldersVariables(template, maptemplate)
}
