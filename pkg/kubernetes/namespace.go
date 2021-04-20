package kubernetes

import (
	"context"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli"
	kcorev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CreateNamespace Check or create a namespace
func CreateNamespace(cs *cli.Clients, namespace string) error {
	_, err := cs.Kube.CoreV1().Namespaces().Get(context.Background(), namespace, v1.GetOptions{})
	if err != nil {
		cs.Log.Infof("Creating Namespace: %s", namespace)
		_, err = cs.Kube.CoreV1().Namespaces().Create(context.Background(), &kcorev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: namespace,
			},
		}, v1.CreateOptions{})
		if err != nil {
			return (err)
		}
	}
	cs.Log.Infof("Using Namespace is: %s", namespace)
	return nil
}
