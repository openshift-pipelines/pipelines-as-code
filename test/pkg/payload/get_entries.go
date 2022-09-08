package payload

import (
	"bytes"
	"fmt"
	"path/filepath"
	"text/template"
)

func GetEntries(yamlfile map[string]string, targetNS, targetBranch, targetEvent string) (map[string]string, error) {
	entries := map[string]string{}
	for target, file := range yamlfile {
		output, err := applyTemplate(file, filepath.Base(target), targetNS, targetBranch, targetEvent)
		if err != nil {
			return nil, fmt.Errorf("failed to read yaml file: %w", err)
		}
		entries[target] = output
	}
	return entries, nil
}

func applyTemplate(tmplFile, pipelineName, targetNS, targetBranch, targetEvent string) (string, error) {
	// read templates from file and  apply variables
	var buf bytes.Buffer
	tmpl := template.Must(template.ParseFiles(tmplFile))
	if err := tmpl.Execute(&buf, map[string]string{
		"TargetNamespace": targetNS,
		"TargetBranch":    targetBranch,
		"TargetEvent":     targetEvent,
		"PipelineName":    pipelineName,
	}); err != nil {
		return "", fmt.Errorf("failed to apply template: %w", err)
	}
	return buf.String(), nil
}
