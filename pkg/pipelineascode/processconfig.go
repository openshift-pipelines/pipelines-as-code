package pipelineascode

import (
	"context"
	"io/ioutil"
	"strings"

	"github.com/ghodss/yaml"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/webvcs"
	"github.com/tektoncd/hub/api/pkg/cli/hub"
)

type tektonYaml struct {
	Tasks     []string `yaml:"metadata,omitempty"`
	Namespace string   `yaml:"namespace,omitempty"`
}

type TektonYamlConfig struct {
	RemoteTasks string
	Namespace   string
}

const (
	tektonCatalogHubName = "tekton"
)

func addTaskYamlDocuments(data string) string {
	return "\n---\n" + strings.TrimSpace(data) + "\n"
}

func processTektonYaml(ctx context.Context, cs *cli.Clients, runinfo *webvcs.RunInfo, data string) (TektonYamlConfig, error) {
	tyConfig := TektonYamlConfig{}
	ty := tektonYaml{}
	err := yaml.Unmarshal([]byte(data), &ty)
	if err != nil {
		return tyConfig, err
	}

	tyConfig.Namespace = ty.Namespace

	// parse tasks syntax:
	// - http(s)?://foo -> url
	// - foo/bar -> internal file inside repo
	// - word -> latest task from catalog via hub
	// - word:version -> task specific version from catalog via hub
	for _, task := range ty.Tasks {
		switch {
		case strings.HasPrefix(task, "http://"):
		case strings.HasPrefix(task, "https://"):
			res, err := cs.HTTPClient.Get(task)
			if err != nil {
				return tyConfig, err
			}
			data, _ := ioutil.ReadAll(res.Body)
			defer res.Body.Close()
			tyConfig.RemoteTasks += addTaskYamlDocuments(string(data))
		case strings.Contains(task, "/"):
			data, err := cs.GithubClient.GetFileInsideRepo(ctx, task, false, runinfo)
			if err != nil {
				return tyConfig, err
			}
			tyConfig.RemoteTasks += addTaskYamlDocuments(string(data))
		default:
			var version string
			taskName := task
			if strings.Contains(task, ":") {
				split := strings.Split(task, ":")
				version = split[len(split)-1]
				taskName = split[0]
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
