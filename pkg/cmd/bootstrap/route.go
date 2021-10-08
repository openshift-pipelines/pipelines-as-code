package bootstrap

import (
	"context"
	"fmt"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// detectOpenShiftRoute detect the openshift route where the eventlistener is running
func detectOpenShiftRoute(ctx context.Context, run *params.Run, opts *bootstrapOpts) (string, error) {
	gvr := schema.GroupVersionResource{
		Group: openShiftRouteGroup, Version: openShiftRouteVersion, Resource: openShiftRouteResource,
	}
	routes, err := run.Clients.Dynamic.Resource(gvr).Namespace(opts.targetNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: pacLabel,
	})
	if err != nil {
		return "", err
	}
	if len(routes.Items) != 1 {
		return "", err
	}
	route := routes.Items[0]

	spec, ok := route.Object["spec"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("couldn't find spec in the EL route")
	}

	host, ok := spec["host"].(string)
	if !ok {
		// this condition is satisfied if there's no metadata at all in the provided CR
		return "", fmt.Errorf("couldn't find spec.host in the EL route")
	}

	return fmt.Sprintf("https://%s", host), nil
}
