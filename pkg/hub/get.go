package hub

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/settings"
)

func getSpecificVersion(ctx context.Context, cs *params.Run, catalogName, resource, kind string) (string, error) {
	split := strings.Split(resource, ":")
	version := split[len(split)-1]
	resourceName := split[0]
	value, _ := cs.Info.Pac.HubCatalogs.Load(catalogName)
	catalogValue, ok := value.(settings.HubCatalog)
	if !ok {
		return "", fmt.Errorf("could not get details for catalog name: %s", catalogName)
	}
	url := fmt.Sprintf("%s/resource/%s/%s/%s/%s", catalogValue.URL, catalogValue.Name, kind, resourceName, version)
	hr := hubResourceVersion{}
	data, err := cs.Clients.GetURL(ctx, url)
	if err != nil {
		return "", fmt.Errorf("could not fetch specific %s version from the hub %s:%s: %w", kind, resource, version, err)
	}
	err = json.Unmarshal(data, &hr)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s/raw", url), nil
}

func getLatestVersion(ctx context.Context, cs *params.Run, catalogName, resource, kind string) (string, error) {
	value, _ := cs.Info.Pac.HubCatalogs.Load(catalogName)
	catalogValue, ok := value.(settings.HubCatalog)
	if !ok {
		return "", fmt.Errorf("could not get details for catalog name: %s", catalogName)
	}
	url := fmt.Sprintf("%s/resource/%s/%s/%s", catalogValue.URL, catalogValue.Name, kind, resource)
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

func GetResource(ctx context.Context, cli *params.Run, catalogName, resource, kind string) (string, error) {
	var rawURL string
	var err error

	if strings.Contains(resource, ":") {
		rawURL, err = getSpecificVersion(ctx, cli, catalogName, resource, kind)
	} else {
		rawURL, err = getLatestVersion(ctx, cli, catalogName, resource, kind)
	}
	if err != nil {
		return "", fmt.Errorf("could not fetch remote %s %s, hub API returned: %w", kind, resource, err)
	}

	data, err := cli.Clients.GetURL(ctx, rawURL)
	if err != nil {
		return "", fmt.Errorf("could not fetch remote %s %s, hub API returned: %w", kind, resource, err)
	}
	return string(data), err
}
