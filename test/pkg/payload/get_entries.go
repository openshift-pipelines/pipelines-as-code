package payload

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
)

func GetEntries(yamlfile []string, targetNS, targetBranch, targetEvent string) (map[string]string, error) {
	entries := map[string]string{}
	for _, file := range yamlfile {
		yamlprun, err := ioutil.ReadFile(file)
		if err != nil {
			return nil, fmt.Errorf("failed to read yaml file: %w", err)
		}
		entries[filepath.Join(".tekton", filepath.Base(file))] = fmt.Sprintf(string(yamlprun), targetNS, targetBranch,
			targetEvent)
	}
	return entries, nil
}
