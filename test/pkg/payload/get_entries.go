package payload

import (
	"fmt"
	"io/ioutil"
)

func GetEntries(yamlfile, targetNS, targetBranch, targetEvent string) (map[string]string, error) {
	prun, err := ioutil.ReadFile(yamlfile)
	if err != nil {
		return nil, err
	}

	return map[string]string{
		".tekton/pr.yaml": fmt.Sprintf(string(prun), targetNS, targetBranch, targetEvent),
	}, nil
}
