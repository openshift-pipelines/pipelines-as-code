package azuredevops

import (
	"context"
	"fmt"
	"strings"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/acl"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
)

// ToDo: implement this function.
func (v *Provider) CheckPolicyAllowing(context.Context, *info.Event, []string) (bool, string) {
	return false, ""
}

// ToDo: implement this function.
func (v *Provider) IsAllowed(context.Context, *info.Event) (bool, error) {
	return true, nil
}

func (v *Provider) IsAllowedOwnersFile(ctx context.Context, event *info.Event) (bool, error) {
	ownerContent, err := v.getFileFromDefaultBranch(ctx, "OWNERS", event)
	if err != nil {
		if strings.Contains(err.Error(), "cannot find") {
			// no owner file, skipping
			return false, nil
		}
		return false, err
	}

	return acl.UserInOwnerFile(ownerContent, event.Sender)
}

func (v *Provider) getFileFromDefaultBranch(ctx context.Context, path string, runevent *info.Event) (string, error) {
	tektonyaml, err := v.GetFileInsideRepo(ctx, runevent, path, runevent.DefaultBranch)
	if err != nil {
		return "", fmt.Errorf("cannot find %s inside the %s branch: %w", path, runevent.DefaultBranch, err)
	}
	return tektonyaml, err
}
