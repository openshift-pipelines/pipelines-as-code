package pipelineascode

import (
	"regexp"
	"strings"
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
