package azuredevops

import (
	"context"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
)

// CheckPolicyAllowing implements provider.Interface.
func (v *Provider) CheckPolicyAllowing(context.Context, *info.Event, []string) (bool, string) {
	panic("unimplemented")
}

// IsAllowed implements provider.Interface.
func (v *Provider) IsAllowed(context.Context, *info.Event) (bool, error) {
	panic("unimplemented")
}

// IsAllowedOwnersFile implements provider.Interface.
func (v *Provider) IsAllowedOwnersFile(context.Context, *info.Event) (bool, error) {
	panic("unimplemented")
}
