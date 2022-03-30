package bootstrap

import (
	"context"
	"fmt"

	"github.com/google/go-github/v43/github"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const configMapPacLabel = "app.kubernetes.io/part-of=pipelines-as-code"

// deleteSecret delete secret first if it exists
func deleteSecret(ctx context.Context, run *params.Run, opts *bootstrapOpts) error {
	return run.Clients.Kube.CoreV1().Secrets(opts.targetNamespace).Delete(ctx, secretName, metav1.DeleteOptions{})
}

// create a kubernetes secret from the manifest file values
func createPacSecret(ctx context.Context, run *params.Run, opts *bootstrapOpts, manifest *github.AppConfig) error {
	_, err := run.Clients.Kube.CoreV1().Secrets(opts.targetNamespace).Create(ctx, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: secretName,
		},
		Data: map[string][]byte{
			"github-application-id": []byte(fmt.Sprintf("%d", manifest.GetID())),
			"github-private-key":    []byte(manifest.GetPEM()),
			"webhook.secret":        []byte(manifest.GetWebhookSecret()),
		},
	}, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	// nolint:forbidigo
	fmt.Printf("🔑 Secret %s has been created in the %s namespace\n", secretName, opts.targetNamespace)

	return nil
}

// checkSecret checks if the secret exists
func checkSecret(ctx context.Context, run *params.Run, opts *bootstrapOpts) bool {
	_, err := run.Clients.Kube.CoreV1().Secrets(opts.targetNamespace).Get(ctx, secretName, metav1.GetOptions{})
	return err == nil
}

// check if we have the namespace created
func checkNS(ctx context.Context, run *params.Run, targetNamespace string) (bool, error) {
	ns, err := run.Clients.Kube.CoreV1().Namespaces().Get(ctx, targetNamespace, metav1.GetOptions{})
	if err != nil {
		return false, err
	}

	// check if there is a configmap with the pipelines-as-code label in targetNamespace
	cms, err := run.Clients.Kube.CoreV1().ConfigMaps(ns.GetName()).List(ctx, metav1.ListOptions{LabelSelector: configMapPacLabel})
	if err != nil {
		return false, err
	}
	if cms.Items == nil || len(cms.Items) == 0 {
		return false, nil
	}
	return true, nil
}

func checkPipelinesInstalled(run *params.Run) (bool, error) {
	sg, err := run.Clients.Kube.Discovery().ServerGroups()
	if err != nil {
		return false, err
	}
	tektonFound := false
	for _, t := range sg.Groups {
		if t.Name == "tekton.dev" {
			tektonFound = true
		}
	}
	return tektonFound, nil
}
