package pipelineascode

import (
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/ghodss/yaml"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/webvcs"
)

type tektonYaml struct {
	Tasks []string `yaml:"metadata,omitempty"`
}

type TektonYamlConfig struct {
	RemoteTasks string
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
			if !strings.HasPrefix(string(data), "---") {
				tyConfig.RemoteTasks = fmt.Sprintf("%s\n---\n", tyConfig.RemoteTasks)
			}
			tyConfig.RemoteTasks = strings.TrimSpace(tyConfig.RemoteTasks + strings.TrimSpace(string(data)))
			// If it's not an http URL but we have slash in URL assume it's a
			// reference from inside the tekton directory.
		} else if strings.Contains(task, "/") {
			data, err := cs.GithubClient.GetFileInsideRepo(task, runinfo)
			if err != nil {
				return tyConfig, err
			}
			tyConfig.RemoteTasks = strings.TrimSpace(tyConfig.RemoteTasks + strings.TrimSpace(string(data)))
		}
	}
	return tyConfig, nil
}
