package kubeinteraction

import (
	"context"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GetNamespace get a namespace
func (k Interaction) GetNamespace(ctx context.Context, namespace string) error {
	_, err := k.Run.Clients.Kube.CoreV1().Namespaces().Get(ctx, namespace, v1.GetOptions{})
	if err != nil {
		k.Run.Clients.Log.Infof("namespace: %s cannot be found", namespace)
		return err
	}
	k.Run.Clients.Log.Infof("namespace is: %s", namespace)
	return nil
}
