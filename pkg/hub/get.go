package hub

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
)

var (
	tektonCatalogHubName = `tekton`
	hubBaseURL           = `https://api.hub.tekton.dev/v1`
)

func getSpecificVersion(ctx context.Context, cli *params.Run, task string) (string, error) {
	split := strings.Split(task, ":")
	version := split[len(split)-1]
	taskName := split[0]
	hr := hubResourceVersion{}
	data, err := cli.Clients.GetURL(ctx,
		fmt.Sprintf("%s/resource/%s/task/%s/%s", hubBaseURL, tektonCatalogHubName, taskName, version))
	if err != nil {
		return "", fmt.Errorf("could not fetch specific task version from the hub %s:%s: %w", task, version, err)
	}
	err = json.Unmarshal(data, &hr)
	if err != nil {
		return "", err
	}
	return *hr.Data.RawURL, nil
}

func getLatestVersion(ctx context.Context, cli *params.Run, task string) (string, error) {
	hr := new(hubResource)
	data, err := cli.Clients.GetURL(ctx, fmt.Sprintf("%s/resource/%s/task/%s", hubBaseURL, tektonCatalogHubName, task))
	if err != nil {
		return "", err
	}
	err = json.Unmarshal(data, &hr)
	if err != nil {
		return "", err
	}

	return *hr.Data.LatestVersion.RawURL, nil
}

func GetTask(ctx context.Context, cli *params.Run, task string) (string, error) {
	var rawURL string
	var err error

	if strings.Contains(task, ":") {
		rawURL, err = getSpecificVersion(ctx, cli, task)
	} else {
		rawURL, err = getLatestVersion(ctx, cli, task)
	}
	if err != nil {
		return "", fmt.Errorf("could not fetch remote task %s: %w", task, err)
	}

	data, err := cli.Clients.GetURL(ctx, rawURL)
	if err != nil {
		return "", fmt.Errorf("could not fetch remote task %s: %w", task, err)
	}
	return string(data), err
}
