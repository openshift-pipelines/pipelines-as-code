package consoleui

import (
	"context"
	"fmt"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli"
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

// getOpenshiftConsole use dynamic client to get the route of the openshit
// console where we can point to.
func getOpenshiftConsole(cs *cli.Clients, ns, pr string) (string, error) {
	gvr := schema.GroupVersionResource{Group: openShiftRouteGroup, Version: openShiftRouteVersion, Resource: openShiftRouteResource}
	route, err := cs.Dynamic.Resource(gvr).Namespace(openShiftConsoleNS).Get(context.Background(), openShiftConsoleRouteName, metav1.GetOptions{})

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

// GetConsoleURL Get a Console URL, OpenShift Console or Tekton Dashboard.
// don't error if we can't find it.
func GetConsoleUI(cs *cli.Clients, ns, pr string) (string, error) {
	var url string
	var err error
	url, err = getOpenshiftConsole(cs, ns, pr)
	if err != nil {
		return "", err
	}
	cs.Log.Info("Console view url: %s", url)
	return url, nil
}
