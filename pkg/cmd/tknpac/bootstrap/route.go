package bootstrap

import (
	"context"
	"fmt"
	"net/http"
	"strings"

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

func detectSelfSignedCertificate(ctx context.Context, url string) string {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Sprintf("invalid url?? %s", url)
	}

	client := http.Client{}
	resp, err := client.Do(req)

	if err != nil && strings.Contains(err.Error(), "x509: certificate is not valid") {
		resp.Body.Close()
		return "⚠️ your eventlistenner route is using self signed certificate\n⚠️ make sure you allow connecting to self signed url in your github app setting."
	} else if err != nil {
		resp.Body.Close()
		return fmt.Sprintf("⚠️ could not connect to the route %s, make sure the eventlistenner is running", url)
	}
	// golangci-lint makes me do it, can't use defer :\
	resp.Body.Close()
	return ""
}
