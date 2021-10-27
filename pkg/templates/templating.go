package templates

import (
	"regexp"
	"strings"

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

// processTemplates process all templates replacing
func Process(event *info.Event, template string) string {
	repoURL := event.URL
	// On bitbucket server you are have a special url for checking it out, they
	// seemed to fix it in 2.0 but i guess we have to live with this until then.
	if event.CloneURL != "" {
		repoURL = event.CloneURL
	}

	return ReplacePlaceHoldersVariables(template, map[string]string{
		"revision":   event.SHA,
		"repo_url":   repoURL,
		"repo_owner": strings.ToLower(event.Owner),
		"repo_name":  strings.ToLower(event.Repository),
	})
}
