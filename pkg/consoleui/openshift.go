package consoleui

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

const (
	openShiftConsoleNS             = "openshift-console"
	openShiftConsoleRouteName      = "console"
	openShiftPipelineDetailViewURL = "https://%s/k8s/ns/%s/tekton.dev~v1beta1~PipelineRun/%s"
	openShiftPipelineTaskLogURL    = "%s/logs/%s"
	openShiftRouteGroup            = "route.openshift.io"
	openShiftRouteVersion          = "v1"
	openShiftRouteResource         = "routes"
)

type OpenshiftConsole struct {
	host string
}

func (o *OpenshiftConsole) URL() string {
	return "https://" + o.host
}

func (o *OpenshiftConsole) DetailURL(ns, pr string) string {
	return fmt.Sprintf(openShiftPipelineDetailViewURL, o.host, ns, pr)
}

func (o *OpenshiftConsole) TaskLogURL(ns, pr, task string) string {
	return fmt.Sprintf(openShiftPipelineTaskLogURL, o.DetailURL(ns, pr), task)
}

// UI use dynamic client to get the route of the openshift
// console where we can point to.
func (o *OpenshiftConsole) UI(ctx context.Context, kdyn dynamic.Interface) error {
	gvr := schema.GroupVersionResource{
		Group: openShiftRouteGroup, Version: openShiftRouteVersion, Resource: openShiftRouteResource,
	}

	route, err := kdyn.Resource(gvr).Namespace(openShiftConsoleNS).Get(ctx, openShiftConsoleRouteName,
		metav1.GetOptions{})
	if err != nil {
		return err
	}

	spec, ok := route.Object["spec"].(map[string]interface{})
	if !ok {
		// this condition is satisfied if there's no metadata at all in the provided CR
		return fmt.Errorf("couldn't find spec in the OpenShift Console route")
	}

	if o.host, ok = spec["host"].(string); !ok {
		// this condition is satisfied if there's no metadata at all in the provided CR
		return fmt.Errorf("couldn't find spec.host in the OpenShift Console route")
	}

	return nil
}
