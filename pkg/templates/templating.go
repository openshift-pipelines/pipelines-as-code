package templates

import (
	"strings"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/keys"
)

// ReplacePlaceHoldersVariables Replace those {{var}} placeholders to the runinfo variable
func ReplacePlaceHoldersVariables(template string, dico map[string]string) string {
	return keys.ParamsRe.ReplaceAllStringFunc(template, func(s string) string {
		parts := keys.ParamsRe.FindStringSubmatch(s)
		key := strings.TrimSpace(parts[1])
		if _, ok := dico[key]; !ok {
			return s
		}
		return dico[key]
	})
}
