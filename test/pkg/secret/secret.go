package secret

import (
	"context"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	v1 "k8s.io/api/core/v1"
	v12 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Create(ctx context.Context, runcnx *params.Run, secretData map[string]string, targetNamespace, secretName string) error {
	secret := &v1.Secret{
		ObjectMeta: v12.ObjectMeta{
			Name:   secretName,
			Labels: map[string]string{"app.kubernetes.io/managed-by": "pipelines-as-code"},
		},
	}
	secret.StringData = secretData
	_, err := runcnx.Clients.Kube.CoreV1().Secrets(targetNamespace).Create(ctx, secret, v12.CreateOptions{})
	return err
}
