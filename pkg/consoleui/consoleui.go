package consoleui

import (
	"context"
	"fmt"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	openShiftConsoleNS        = "openshift-console"
	openShiftConsoleRouteName = "console"
	openShiftPipelineViewURL  = "https://%s/k8s/ns/%s/tekton.dev~v1beta1~PipelineRun/%s/logs"
	openShiftRouteGroup       = "route.openshift.io"
	openShiftRouteVersion     = "v1"
	openShiftRouteResource    = "routes"
)

// getOpenshiftConsole use dynamic client to get the route of the openshift
// console where we can point to.
func getOpenshiftConsole(ctx context.Context, cs *params.Run, ns, pr string) (string, error) {
	gvr := schema.GroupVersionResource{
		Group: openShiftRouteGroup, Version: openShiftRouteVersion, Resource: openShiftRouteResource,
	}

	route, err := cs.Clients.Dynamic.Resource(gvr).Namespace(openShiftConsoleNS).Get(ctx, openShiftConsoleRouteName, metav1.GetOptions{})
	if err != nil {
		return "", err
	}

	spec, ok := route.Object["spec"].(map[string]interface{})
	if !ok {
		// this condition is satisfied if there's no metadata at all in the provided CR
		return "", fmt.Errorf("couldn't find spec in the OpenShift Console route")
	}

	host, ok := spec["host"].(string)
	if !ok {
		// this condition is satisfied if there's no metadata at all in the provided CR
		return "", fmt.Errorf("couldn't find spec.host in the OpenShift Console route")
	}

	// If we don't have a target ns/pr just print it.
	if ns == "" && pr == "" {
		return fmt.Sprintf("https://%s", host), nil
	}

	return fmt.Sprintf(openShiftPipelineViewURL, host, ns, pr), nil
}

// GetConsoleUI Get a Console URL, OpenShift Console or Tekton Dashboard.
// don't error if we can't find it.
func GetConsoleUI(ctx context.Context, cs *params.Run, ns, pr string) (string, error) {
	var url string
	var err error
	url, err = getOpenshiftConsole(ctx, cs, ns, pr)
	if err != nil {
		return "", err
	}
	cs.Clients.Log.Infof("Web Console PR url: %s", url)
	return url, nil
}
