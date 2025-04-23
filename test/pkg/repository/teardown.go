package repository

import (
	"context"
	"testing"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NSTearDown deletes the repositories in the specified namespace and then deletes the namespace itself.
// Deleting the namespace triggers the deletion of all resources within it.
func NSTearDown(ctx context.Context, t *testing.T, runcnx *params.Run, targetNS string) {
	runcnx.Clients.Log.Infof("Deleting Repository in %s", targetNS)
	err := runcnx.Clients.PipelineAsCode.PipelinesascodeV1alpha1().Repositories(targetNS).DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{})
	assert.NilError(t, err)
	runcnx.Clients.Log.Infof("Deleting NS %s", targetNS)
	err = runcnx.Clients.Kube.CoreV1().Namespaces().Delete(ctx, targetNS, metav1.DeleteOptions{})
	assert.NilError(t, err)
}
