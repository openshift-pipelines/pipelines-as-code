package bootstrap

import (
	"context"
	"crypto/x509"
	"errors"
	"fmt"
	"net/http"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

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
	if err != nil && isTLSError(err) {
		return "⚠️ your eventlistenner route is using self signed certificate\n⚠️ make sure you allow connecting to self signed url in your github app setting."
	} else if err != nil {
		return fmt.Sprintf("⚠️ could not connect to the route %s, make sure the eventlistenner is running", url)
	}
	resp.Body.Close()
	return ""
}

func isTLSError(err error) bool {
	return errors.As(err, &x509.UnknownAuthorityError{}) || errors.As(err, &x509.CertificateInvalidError{})
}
