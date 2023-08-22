package hub

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/settings"
)

func getSpecificVersion(ctx context.Context, cs *params.Run, catalogName, task string) (string, error) {
	split := strings.Split(task, ":")
	version := split[len(split)-1]
	taskName := split[0]
	value, _ := cs.Info.Pac.HubCatalogs.Load(catalogName)
	catalogValue, ok := value.(settings.HubCatalog)
	if !ok {
		return "", fmt.Errorf("could not get details for catalog name: %s", catalogName)
	}
	url := fmt.Sprintf("%s/resource/%s/task/%s/%s", catalogValue.URL, catalogValue.Name, taskName, version)
	hr := hubResourceVersion{}
	data, err := cs.Clients.GetURL(ctx, url)
	if err != nil {
		return "", fmt.Errorf("could not fetch specific task version from the hub %s:%s: %w", task, version, err)
	}
	err = json.Unmarshal(data, &hr)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s/raw", url), nil
}

func getLatestVersion(ctx context.Context, cs *params.Run, catalogName, task string) (string, error) {
	value, _ := cs.Info.Pac.HubCatalogs.Load(catalogName)
	catalogValue, ok := value.(settings.HubCatalog)
	if !ok {
		return "", fmt.Errorf("could not get details for catalog name: %s", catalogName)
	}
	url := fmt.Sprintf("%s/resource/%s/task/%s", catalogValue.URL, catalogValue.Name, task)
	hr := new(hubResource)
	data, err := cs.Clients.GetURL(ctx, url)
	if err != nil {
		return "", err
	}
	err = json.Unmarshal(data, &hr)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s/%s/raw", url, *hr.Data.LatestVersion.Version), nil
}

func GetTask(ctx context.Context, cli *params.Run, catalogName, task string) (string, error) {
	var rawURL string
	var err error

	if strings.Contains(task, ":") {
		rawURL, err = getSpecificVersion(ctx, cli, catalogName, task)
	} else {
		rawURL, err = getLatestVersion(ctx, cli, catalogName, task)
	}
	if err != nil {
		return "", fmt.Errorf("could not fetch remote task %s, hub API returned: %w", task, err)
	}

	data, err := cli.Clients.GetURL(ctx, rawURL)
	if err != nil {
		return "", fmt.Errorf("could not fetch remote task %s, hub API returned: %w", task, err)
	}
	return string(data), err
}
