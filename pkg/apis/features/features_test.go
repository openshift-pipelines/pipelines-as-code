package features

import (
	"testing"

	tektonconfig "github.com/tektoncd/pipeline/pkg/apis/config"
	"gotest.tools/v3/assert"
	rtesting "knative.dev/pkg/reconciler/testing"
)

func TestSetFeatureFlag(t *testing.T) {
	ctx, _ := rtesting.SetupFakeContext(t)
	cfg := tektonconfig.FromContext(SetFeatureFlag(ctx))
	assert.Assert(t, cfg.FeatureFlags.EnableTektonOCIBundles)
}
