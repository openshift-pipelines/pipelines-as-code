package adapter

import (
	"context"
	"os"
	"path/filepath"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/system"
)

const tlsMountPath = "/etc/pipelines-as-code/tls"

// isTLSEnabled validates if tls secret exist and if the required fields are defined
// this is used to enable tls on the listener.
func (l listener) isTLSEnabled() (bool, string, string) {
	tlsSecret := os.Getenv("TLS_SECRET_NAME")
	tlsKey := os.Getenv("TLS_KEY")
	tlsCert := os.Getenv("TLS_CERT")

	// TODO: Should we make different TLS by controller?
	tls, err := l.run.Clients.Kube.CoreV1().Secrets(system.Namespace()).
		Get(context.Background(), tlsSecret, v1.GetOptions{})
	if err != nil {
		return false, "", ""
	}
	_, ok := tls.Data[tlsKey]
	if !ok {
		return false, "", ""
	}
	_, ok = tls.Data[tlsCert]
	if !ok {
		return false, "", ""
	}

	return true,
		filepath.Join(tlsMountPath, tlsCert),
		filepath.Join(tlsMountPath, tlsKey)
}
