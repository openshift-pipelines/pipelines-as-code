package azuredevops

import (
	"context"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
)

// CheckPolicyAllowing implements provider.Interface.
func (v *Provider) CheckPolicyAllowing(context.Context, *info.Event, []string) (bool, string) {
	panic("unimplemented")
}

// ToDo: implement this function
func (v *Provider) IsAllowed(context.Context, *info.Event) (bool, error) {
	return true, nil
}

// IsAllowedOwnersFile implements provider.Interface.
func (v *Provider) IsAllowedOwnersFile(context.Context, *info.Event) (bool, error) {
	panic("unimplemented")
}
