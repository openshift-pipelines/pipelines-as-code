package pipelineascode

import (
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/webvcs"
)

// aclCheck check if we are allowed to run pipeline
func aclCheck(cs *cli.Clients, runinfo *webvcs.RunInfo) (bool, error) {
	// If the user who submitted the PR is the same as the Owner of the repo then allow it.
	if runinfo.Owner == runinfo.Sender {
		return true, nil
	}

	// If the user who has submitted the pr is a sender on the repo then allows to be run.
	isUserMemberRepo, err := cs.GithubClient.CheckSenderOrgMembership(runinfo)
	if err != nil {
		return false, err
	}

	if isUserMemberRepo {
		return true, nil
	}

	// TODO: there is going to be more stuff in there
	return false, err
}
