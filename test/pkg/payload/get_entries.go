package payload

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
	"text/template"
)

func GetEntries(yamlfile map[string]string, targetNS, targetBranch, targetEvent string) (map[string]string, error) {
	params := map[string]string{
		"TargetNamespace": targetNS,
		"TargetBranch":    targetBranch,
		"TargetEvent":     targetEvent,
	}
	entries := map[string]string{}
	for target, file := range yamlfile {
		name := strings.TrimSuffix(filepath.Base(target), filepath.Ext(target))
		params["PipelineName"] = name
		output, err := ApplyTemplate(file, params)
		if err != nil {
			return nil, fmt.Errorf("failed to read yaml file: %w", err)
		}
		entries[target] = output
	}
	return entries, nil
}

func ApplyTemplate(templateFile string, params map[string]string) (string, error) {
	// read templates from file and  apply variables
	var buf bytes.Buffer
	tmpl := template.Must(template.ParseFiles(templateFile))
	if err := tmpl.Execute(&buf, params); err != nil {
		return "", fmt.Errorf("failed to apply template: %w", err)
	}
	return buf.String(), nil
}
