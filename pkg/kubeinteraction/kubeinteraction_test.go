package kubeinteraction

import (
	"testing"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"gotest.tools/v3/assert"
)

func TestNewKubernetesInteraction(t *testing.T) {
	_, err := NewKubernetesInteraction(params.New())
	assert.NilError(t, err)
}
