package hub

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli"
)

var (
	tektonCatalogHubName = `tekton`
	hubBaseURL           = `https://api.hub.tekton.dev/v1`
)

func getURL(ctx context.Context, cli *cli.Clients, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return []byte{}, err
	}
	res, err := cli.HTTPClient.Do(req)
	if err != nil {
		return []byte{}, err
	}
	defer res.Body.Close()
	data, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return []byte{}, err
	}
	return data, nil
}

func getSpecificVersion(ctx context.Context, cli *cli.Clients, task string) (string, error) {
	split := strings.Split(task, ":")
	version := split[len(split)-1]
	taskName := split[0]
	hr := new(hubResourceVersion)
	data, err := getURL(ctx, cli,
		fmt.Sprintf("%s/resource/%s/task/%s/%s", hubBaseURL, tektonCatalogHubName, taskName, version))
	if err != nil {
		return "", err
	}
	err = json.Unmarshal(data, &hr)
	if err != nil {
		return "", err
	}
	return *hr.Data.RawURL, nil
}

func getLatestVersion(ctx context.Context, cli *cli.Clients, task string) (string, error) {
	data, err := getURL(ctx, cli, fmt.Sprintf("%s/resource/%s/task/%s", hubBaseURL, tektonCatalogHubName, task))
	hr := new(hubResource)
	if err != nil {
		return "", err
	}
	err = json.Unmarshal(data, &hr)
	if err != nil {
		return "", err
	}
	return *hr.Data.LatestVersion.RawURL, nil
}

func GetTask(ctx context.Context, cli *cli.Clients, task string) (string, error) {
	var rawURL string
	var err error

	if strings.Contains(task, ":") {
		rawURL, err = getSpecificVersion(ctx, cli, task)
	} else {
		rawURL, err = getLatestVersion(ctx, cli, task)
	}
	if err != nil {
		return "", err
	}

	data, err := getURL(ctx, cli, rawURL)
	if err != nil {
		return "", err
	}
	return string(data), err
}
