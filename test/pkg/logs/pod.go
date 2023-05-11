package log

import (
	"context"
	"fmt"
	"io"

	"github.com/google/go-github/v52/github"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1i "k8s.io/client-go/kubernetes/typed/core/v1"
)

func GetControllerLog(ctx context.Context, kclient corev1i.CoreV1Interface, labelselector, containerName string) (string, error) {
	_, err := kclient.Namespaces().Get(ctx, "pipelines-as-code", metav1.GetOptions{})
	var ns string
	if err == nil {
		ns = "pipelines-as-code"
	} else {
		_, err = kclient.Namespaces().Get(ctx, "openshift-pipelines", metav1.GetOptions{})
		if err != nil {
			return "", err
		}
		ns = "openshift-pipelines"
	}
	return GetPodLog(ctx, kclient, ns, labelselector, containerName)
}

func GetPodLog(ctx context.Context, kclient corev1i.CoreV1Interface, ns, labelselector, containerName string) (string, error) {
	nsO, err := kclient.Namespaces().Get(ctx, ns, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	items, err := kclient.Pods(nsO.GetName()).List(ctx, metav1.ListOptions{
		LabelSelector: labelselector,
	})
	if err != nil {
		return "", err
	}
	if len(items.Items) == 0 {
		return "", fmt.Errorf("could not match any pod to label selector: %s", labelselector)
	}
	// maybe one day there is going to be multiple controller containers and then we would need to handle it here
	ios, err := kclient.Pods(nsO.GetName()).GetLogs(items.Items[0].GetName(), &v1.PodLogOptions{
		Container: containerName,
		TailLines: github.Int64(10),
	}).Stream(ctx)
	if err != nil {
		return "", err
	}
	log, err := io.ReadAll(ios)
	return string(log), err
}
