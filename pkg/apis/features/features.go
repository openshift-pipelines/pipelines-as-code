package features

import (
	"context"

	tektonconfig "github.com/tektoncd/pipeline/pkg/apis/config"
)

// SetFeatureFlag sets the feature flag for Tekton parsers in the context.
func SetFeatureFlag(ctx context.Context) context.Context {
	featureFlags, _ := tektonconfig.NewFeatureFlagsFromMap(map[string]string{
		"enable-tekton-oci-bundles": "true",
	})
	cfg := &tektonconfig.Config{
		FeatureFlags: featureFlags,
	}
	return tektonconfig.ToContext(ctx, cfg)
}
