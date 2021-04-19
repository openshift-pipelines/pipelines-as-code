package pipelineascode

import (
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/ghodss/yaml"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli"
)

type tektonYaml struct {
	Tasks []string `yaml:"metadata,omitempty"`
}

type TektonYamlConfig struct {
	RemoteTasks string
}

func processTektonYaml(cs *cli.Clients, data string) (TektonYamlConfig, error) {
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
			tyConfig.RemoteTasks = strings.TrimSpace(tyConfig.RemoteTasks + string(data))
		}
	}
	return tyConfig, nil
}
