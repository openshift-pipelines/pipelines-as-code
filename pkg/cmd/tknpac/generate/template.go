package generate

import (
	"bytes"
	"embed"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

type langOpts struct {
	Language       string
	detectionFile  string
	Task           string
	AnnotationTask string
}

// I hate this part of the code so much.. but we are waiting for UBI images
// having >1.6 golang for integrated templates.
var languageDetection = map[string]langOpts{
	"go": {
		Language:       "Golang",
		detectionFile:  "go.mod",
		AnnotationTask: "golangci-lint",
		Task: `- name: golangci-lint
        taskRef:
          name: golangci-lint
        runAfter:
          - fetch-repository
        params:
          - name: package
            value: .
        workspaces:
        - name: source
          workspace: source
`,
	},
	"python": {
		Language:       "Python",
		detectionFile:  "setup.py",
		AnnotationTask: "pylint",
		Task: `- name: pylint
        taskRef:
          name: pylint
        runAfter:
          - fetch-repository
        workspaces:
        - name: source
          workspace: source
`,
	},
}

//go:embed templates
var resource embed.FS

func (o *Opts) detectLanguage() (langOpts, error) {
	if o.language != "" {
		if _, ok := languageDetection[o.language]; !ok {
			return langOpts{}, fmt.Errorf("no template available for %s", o.language)
		}
		return languageDetection[o.language], nil
	}

	cs := o.IOStreams.ColorScheme()
	for _, v := range languageDetection {
		fpath := filepath.Join(o.GitInfo.TopLevelPath, v.detectionFile)
		if _, err := os.Stat(fpath); !os.IsNotExist(err) {
			fmt.Fprintf(o.IOStreams.Out, "%s We have detected your repository using the programming language %s.\n",
				cs.SuccessIcon(),
				cs.Bold(v.Language),
			)
			return v, nil
		}
	}
	return langOpts{}, nil
}

func (o *Opts) genTmpl() (bytes.Buffer, error) {
	var outputBuffer bytes.Buffer
	embedfile, err := resource.Open("templates/pipelinerun.yaml.tmpl")
	if err != nil {
		log.Fatal(err)
	}
	defer embedfile.Close()
	tmplB, _ := ioutil.ReadAll(embedfile)
	// can't figure out how ParseFS works, so doing this manually..
	t := template.Must(template.New("PipelineRun").Delims("<<", ">>").Parse(string(tmplB)))

	prName := fmt.Sprintf("%s-%s",
		filepath.Base(o.GitInfo.URL),
		strings.ReplaceAll(o.event.EventType, "_", "-"))
	lang, err := o.detectLanguage()
	if err != nil {
		return bytes.Buffer{}, err
	}
	data := map[string]interface{}{
		"prName":                  prName,
		"event":                   o.event,
		"extra_task":              lang,
		"use_cluster_task":        o.generateWithClusterTask,
		"language_specific_tasks": "",
	}
	if err := t.Execute(&outputBuffer, data); err != nil {
		return bytes.Buffer{}, err
	}
	return outputBuffer, nil
}
