package generate

import (
	"bytes"
	"fmt"
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

var pipelineRunTmpl = `---
apiVersion: tekton.dev/v1beta1
kind: PipelineRun
metadata:
  name: [[ .prName ]]
  annotations:
    # The event we are targeting (ie: pull_request, push)
    pipelinesascode.tekton.dev/on-event: "[ [[ .event.EventType ]] ]"

    # The branch or tag we are targeting (ie: main, refs/tags/*)
    pipelinesascode.tekton.dev/on-target-branch: "[ [[ .event.BaseBranch ]] ]"

    # Fetch the git-clone task from hub, we are able to reference it with taskRef
    pipelinesascode.tekton.dev/task: "[ git-clone ]"
    [[- if .extra_task.AnnotationTask ]]

    # Task for [[.extra_task.Language ]]
    pipelinesascode.tekton.dev/task-1: "[ [[ .extra_task.AnnotationTask ]] ]"
    [[ end ]]
    # You can add more tasks in here to reuse, browse the one you like from here
    # https://hub.tekton.dev/
    # example:
    # pipelinesascode.tekton.dev/task-2: "[ maven, buildah ]"

    # How many runs we want to keep attached to this event
    pipelinesascode.tekton.dev/max-keep-runs: "5"
spec:
  params:
    # The variable with brackets are special to Pipelines as Code
    # They will automatically be expanded with the events from Github.
    - name: repo_url
      value: "{{ repo_url }}"
    - name: revision
      value: "{{ revision }}"
  pipelineSpec:
    params:
      - name: repo_url
      - name: revision
    workspaces:
      - name: source
      - name: basic-auth
    tasks:
      - name: fetch-repository
        taskRef:
          name: git-clone
        workspaces:
          - name: output
            workspace: source
          - name: basic-auth
            workspace: basic-auth
        params:
          - name: url
            value: $(params.repo_url)
          - name: revision
            value: $(params.revision)
      [[ .extra_task.Task ]]
      # Customize this task if you like, or just do a taskRef
      # to one of the hub task.
      - name: noop-task
        runAfter:
          - fetch-repository
        workspaces:
          - name: source
            workspace: source
        taskSpec:
          workspaces:
            - name: source
          steps:
            - name: noop-task
              image: registry.access.redhat.com/ubi8/ubi-micro:8.4
              workingDir: $(workspaces.source.path)
              script: |
                exit 0
  workspaces:
  - name: source
    volumeClaimTemplate:
      spec:
        accessModes:
          - ReadWriteOnce
        resources:
          requests:
            storage: 1Gi
  # This workspace will inject secret to help the git-clone task to be able to
  # checkout the private repositories
  - name: basic-auth
    secret:
      secretName: "pac-git-basic-auth-{{repo_owner}}-{{repo_name}}"
`

func (o *Opts) detectLanguage() langOpts {
	cs := o.IOStreams.ColorScheme()
	for _, v := range languageDetection {
		fpath := filepath.Join(o.GitInfo.TopLevelPath, v.detectionFile)
		if _, err := os.Stat(fpath); !os.IsNotExist(err) {
			fmt.Fprintf(o.IOStreams.Out, "%s We have detected your repository using the programming language %s.\n",
				cs.SuccessIcon(),
				cs.Bold(v.Language),
			)
			return v
		}
	}
	return langOpts{}
}

func (o *Opts) genTmpl() (bytes.Buffer, error) {
	var outputBuffer bytes.Buffer
	t := template.Must(template.New("PipelineRun").Delims("[[", "]]").Parse(pipelineRunTmpl))
	prName := fmt.Sprintf("%s-%s",
		filepath.Base(o.GitInfo.URL),
		strings.ReplaceAll(o.event.EventType, "_", "-"))
	data := map[string]interface{}{
		"prName":                  prName,
		"event":                   o.event,
		"extra_task":              o.detectLanguage(),
		"language_specific_tasks": "",
	}
	if err := t.Execute(&outputBuffer, data); err != nil {
		return bytes.Buffer{}, err
	}
	return outputBuffer, nil
}
