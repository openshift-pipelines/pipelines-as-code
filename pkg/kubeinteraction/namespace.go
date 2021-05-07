package kubeinteraction

import (
	"context"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GetNamespace get a namespace
func (k Interaction) GetNamespace(namespace string) error {
	_, err := k.Clients.Kube.CoreV1().Namespaces().Get(context.Background(), namespace, v1.GetOptions{})
	if err != nil {
		k.Clients.Log.Infof("Namespace: %s cannot be found")
		return err
	}
	k.Clients.Log.Infof("Namespace is: %s", namespace)
	return nil
}
