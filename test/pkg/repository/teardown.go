package repository

import (
	"context"
	"testing"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func NSTearDown(ctx context.Context, t *testing.T, runcnx *params.Run, targetNS string) {
	runcnx.Clients.Log.Infof("Deleting NS %s", targetNS)
	err := runcnx.Clients.Kube.CoreV1().Namespaces().Delete(ctx, targetNS, metav1.DeleteOptions{})
	assert.NilError(t, err)
}
