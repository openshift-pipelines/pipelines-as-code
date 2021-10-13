package bootstrap

import (
	"context"
	"fmt"

	"github.com/google/go-github/v39/github"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

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
	secret, _ := run.Clients.Kube.CoreV1().Secrets(opts.targetNamespace).Get(ctx, secretName, metav1.GetOptions{})
	return secret.GetName() != ""
}

// check if we have the namespace created
func checkNS(ctx context.Context, run *params.Run, opts *bootstrapOpts) (bool, error) {
	ns, err := run.Clients.Kube.CoreV1().Namespaces().Get(ctx, opts.targetNamespace, metav1.GetOptions{})
	return ns.GetName() != "", err
}
