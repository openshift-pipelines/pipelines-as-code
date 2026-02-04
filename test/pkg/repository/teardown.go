package repository

import (
	"context"
	"os"
	"testing"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/random"
	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func NSTearDown(ctx context.Context, t *testing.T, runcnx *params.Run, targetNS string) {
	repos, err := runcnx.Clients.PipelineAsCode.PipelinesascodeV1alpha1().Repositories(targetNS).List(ctx, metav1.ListOptions{})
	assert.NilError(t, err)

	// Add random suffix to the repo URL so we can keep it for logs collection (if PAC_E2E_KEEP_NS is set)
	// so we can investigate later if needed and capture its status it captured
	// we need to rename it so webhooks don't trigger any more runs
	for _, repo := range repos.Items {
		repo.Spec.URL += random.AlphaString(5)
		if _, err := runcnx.Clients.PipelineAsCode.PipelinesascodeV1alpha1().Repositories(targetNS).Update(ctx, &repo, metav1.UpdateOptions{}); err != nil {
			runcnx.Clients.Log.Warnf("Failed to renamae Repository URL %s: %v", repo.Name, err)
		}
	}

	if os.Getenv("PAC_E2E_KEEP_NS") != "true" {
		runcnx.Clients.Log.Infof("Deleting NS %s", targetNS)
		err = runcnx.Clients.Kube.CoreV1().Namespaces().Delete(ctx, targetNS, metav1.DeleteOptions{})
		assert.NilError(t, err)
	}
}
