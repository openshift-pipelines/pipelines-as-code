package kubeinteraction

import (
	"context"
	"io"

	"github.com/google/go-github/v47/github"
	corev1 "k8s.io/api/core/v1"
)

func (k Interaction) GetPodLogs(ctx context.Context, ns, podName, containerName string, maxNumberLines int64) (string, error) {
	kclient := k.Run.Clients.Kube.CoreV1()
	// maybe one day there is going to be multiple controller containers and then we would need to handle it here
	ios, err := kclient.Pods(ns).GetLogs(podName, &corev1.PodLogOptions{
		Container: containerName,
		TailLines: github.Int64(maxNumberLines),
	}).Stream(ctx)
	if err != nil {
		return "", err
	}
	log, err := io.ReadAll(ios)
	return string(log), err
}
