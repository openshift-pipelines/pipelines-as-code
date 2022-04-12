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

const (
	routePacLabel = "pipelines-as-code/route=controller"
	infoConfigMap = "pipelines-as-code-info"
)

// detectOpenShiftRoute detect the openshift route where the pac controller is running
func detectOpenShiftRoute(ctx context.Context, run *params.Run, opts *bootstrapOpts) (string, error) {
	gvr := schema.GroupVersionResource{
		Group: openShiftRouteGroup, Version: openShiftRouteVersion, Resource: openShiftRouteResource,
	}
	routes, err := run.Clients.Dynamic.Resource(gvr).Namespace(opts.targetNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: routePacLabel,
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
		return "", fmt.Errorf("couldn't find spec in the PAC Controller route")
	}

	host, ok := spec["host"].(string)
	if !ok {
		// this condition is satisfied if there's no metadata at all in the provided CR
		return "", fmt.Errorf("couldn't find spec.host in the PAC controller route")
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
		return "⚠️ your controller route is using a self signed certificate\n⚠️ make sure you allow to connect to self signed url in your github app setting."
	} else if err != nil {
		return fmt.Sprintf("⚠️ could not connect to the route %s, make sure the pipelines-as-code controller is running", url)
	}
	resp.Body.Close()
	return ""
}

func isTLSError(err error) bool {
	return errors.As(err, &x509.UnknownAuthorityError{}) || errors.As(err, &x509.CertificateInvalidError{})
}

type infoOptions struct {
	targetNamespace string
	controllerURL   string
	provider        string
}

func updateInfoConfigMap(ctx context.Context, run *params.Run, opts *infoOptions) error {
	cm, err := run.Clients.Kube.CoreV1().ConfigMaps(opts.targetNamespace).Get(ctx, infoConfigMap, metav1.GetOptions{})
	if err != nil {
		return err
	}

	cm.Data["controller-url"] = opts.controllerURL
	cm.Data["provider"] = opts.provider

	// the user will have read access to configmap
	// but it might be the case, user is not admin and don't have access to update
	// so don't error out, continue with printing a warning
	_, err = run.Clients.Kube.CoreV1().ConfigMaps(opts.targetNamespace).Update(ctx, cm, metav1.UpdateOptions{})
	if err != nil {
		run.Clients.Log.Warnf("failed to update pipelines-as-code-info configmap: %v", err)
		return nil
	}
	return nil
}
