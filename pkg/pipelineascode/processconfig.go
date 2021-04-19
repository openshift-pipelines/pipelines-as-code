package pipelineascode

import (
	"io/ioutil"
	"strings"

	"github.com/ghodss/yaml"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/webvcs"
	"github.com/tektoncd/hub/api/pkg/cli/hub"
)

type tektonYaml struct {
	Tasks []string `yaml:"metadata,omitempty"`
}

type TektonYamlConfig struct {
	RemoteTasks string
}

const (
	tektonCatalogHubName = "tekton"
)

func addTaskYamlDocuments(data string) string {
	return "\n---\n" + strings.TrimSpace(data) + "\n"
}

func processTektonYaml(cs *cli.Clients, runinfo *webvcs.RunInfo, data string) (TektonYamlConfig, error) {
	tyConfig := TektonYamlConfig{}
	ty := tektonYaml{}
	err := yaml.Unmarshal([]byte(data), &ty)
	if err != nil {
		return tyConfig, err
	}

	for _, task := range ty.Tasks {
		if strings.HasPrefix(task, "https://") || strings.HasPrefix(task, "http://") {
			res, err := cs.HTTPClient.Get(task)
			if err != nil {
				return tyConfig, err
			}
			data, _ := ioutil.ReadAll(res.Body)
			defer res.Body.Close()
			tyConfig.RemoteTasks += addTaskYamlDocuments(string(data))

			// If it's not an http URL but we have slash in URL assume it's a
			// reference from inside the tekton directory.
		} else if strings.Contains(task, "/") {
			data, err := cs.GithubClient.GetFileInsideRepo(task, runinfo)
			if err != nil {
				return tyConfig, err
			}
			tyConfig.RemoteTasks += addTaskYamlDocuments(string(data))
		} else {
			var version string
			var taskName = task
			if strings.Contains(task, ":") {
				split := strings.Split(task, ":")
				version = split[len(split)]
				taskName = split[0] // TODO: if there is a task with ":" ??
			}
			resource := cs.Hub.GetResource(hub.ResourceOption{
				Name:    taskName,
				Catalog: tektonCatalogHubName,
				Kind:    "task",
				Version: version,
			})
			data, err := resource.Manifest()
			if err != nil {
				return tyConfig, err
			}
			tyConfig.RemoteTasks += addTaskYamlDocuments(string(data))
		}
	}

	return tyConfig, nil
}
