package pipelineascode

import (
	"context"
	"strings"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/webvcs"
	"sigs.k8s.io/yaml"
)

// OwnersConfig prow owner, only supporting approvers or reviewers in yaml
type OwnersConfig struct {
	Approvers []string `json:"approvers,omitempty"`
	Reviewers []string `json:"reviewers,omitempty"`
}

// aclCheck check if we are allowed to run pipeline
func aclCheck(ctx context.Context, cs *cli.Clients, runinfo *webvcs.RunInfo) (bool, error) {
	// If the user who submitted the PR is the same as the Owner of the repo
	// then allow it.
	if runinfo.Owner == runinfo.Sender {
		return true, nil
	}

	// If the user who has submitted the pr is a owner on the repo then allows
	// the CI to be run.
	isUserMemberRepo, err := cs.GithubClient.CheckSenderOrgMembership(ctx, runinfo)
	if err != nil {
		return false, err
	}

	if isUserMemberRepo {
		return true, nil
	}

	// If we have a prow OWNERS file in the defaultBranch (ie: master) then
	// parse it in approvers and reviewers field and check if sender is in there.
	ownerFile, err := cs.GithubClient.GetFileFromDefaultBranch(ctx, "OWNERS", runinfo)

	// Don't error out if the OWNERS file cannot be found
	if err != nil && !strings.Contains(err.Error(), "cannot find") {
		return false, err
	} else if ownerFile != "" {
		var ownerConfig OwnersConfig
		err := yaml.Unmarshal([]byte(ownerFile), &ownerConfig)
		if err != nil {
			return false, err
		}
		for _, owner := range append(ownerConfig.Approvers, ownerConfig.Reviewers...) {
			if owner == runinfo.Sender {
				return true, nil
			}
		}
	}

	return false, nil
}
