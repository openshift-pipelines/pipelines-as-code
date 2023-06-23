//go:build e2e
// +build e2e

package test

import (
	"fmt"
	"os"
	"regexp"
	"testing"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/settings"
	tgitea "github.com/openshift-pipelines/pipelines-as-code/test/pkg/gitea"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/options"
	"gotest.tools/v3/assert"
)

// TestGiteaPolicyPullRequest tests the pull_request policy
// create a CRD which a policy allowing only users in the team pull_requester to allow a PR
// we create a org
// we create a team normal on org and add the user normal-$RANDOM onto it
// we create a pull request form a fork
// we test that it was denied
// we create a pull request from a fork with the user pull_requester which is in the allowed pull_requester team
// we test that it was allowed succeeded
//
// this test paths is mostly to test the logic in the pkg/policy package, there
// is not much gitea specifics compared to github
func TestGiteaPolicyPullRequest(t *testing.T) {
	topts := &tgitea.TestOpts{
		OnOrg:           true,
		SkipEventsCheck: true,
		TargetEvent:     options.PullRequestEvent,
		Settings: &v1alpha1.Settings{
			Policy: &v1alpha1.Policy{
				PullRequest: []string{"pull_requester"},
			},
		},
		YAMLFiles: map[string]string{".tekton/pr.yaml": "testdata/pipelinerun.yaml"},
	}
	tgitea.TestPR(t, topts)
	topts.ParamsRun.Clients.Log.Infof("Repo CRD %s has been created with Policy: %+v", topts.TargetRefName, topts.Settings.Policy)

	orgName := "org-" + topts.TargetRefName
	topts.Opts.Organization = orgName

	// create normal team on org and add user normal onto it
	normalTeam, err := tgitea.CreateTeam(topts, orgName, "normal")
	assert.NilError(t, err)
	normalUserNamePasswd := fmt.Sprintf("normal-%s", topts.TargetRefName)
	normalUserCnx, normalUser, err := tgitea.CreateGiteaUserSecondCnx(topts, normalUserNamePasswd, normalUserNamePasswd)
	assert.NilError(t, err)
	_, err = topts.GiteaCNX.Client.AddTeamMember(normalTeam.ID, normalUser.UserName)
	assert.NilError(t, err)
	topts.ParamsRun.Clients.Log.Infof("User %s has been added to team %s", normalUser.UserName, normalTeam.Name)
	tgitea.CreateForkPullRequest(t, topts, normalUserCnx, "", "echo Hello from user "+topts.TargetRefName)
	topts.CheckForStatus = "failure"
	topts.Regexp = regexp.MustCompile(
		fmt.Sprintf(`.*policy check: pull_request, user: %s is not a member of any of the allowed teams.*`, normalUserNamePasswd))
	tgitea.WaitForPullRequestCommentMatch(t, topts)
	tgitea.WaitForStatus(t, topts, "heads/"+topts.TargetRefName, settings.PACApplicationNameDefaultValue, false)

	pullRequesterTeam, err := tgitea.CreateTeam(topts, orgName, "pull_requester")
	assert.NilError(t, err)
	pullRequesterUserNamePasswd := fmt.Sprintf("pullRequester-%s", topts.TargetRefName)
	pullRequesterUserCnx, pullRequesterUser, err := tgitea.CreateGiteaUserSecondCnx(topts, pullRequesterUserNamePasswd, pullRequesterUserNamePasswd)
	assert.NilError(t, err)
	_, err = topts.GiteaCNX.Client.AddTeamMember(pullRequesterTeam.ID, pullRequesterUser.UserName)
	assert.NilError(t, err)
	topts.ParamsRun.Clients.Log.Infof("User %s has been added to team %s", pullRequesterUser.UserName, pullRequesterTeam.Name)
	tgitea.CreateForkPullRequest(t, topts, pullRequesterUserCnx, "", "echo Hello from user "+topts.TargetRefName)
	topts.Regexp = successRegexp
	tgitea.WaitForPullRequestCommentMatch(t, topts)
}

// TestGiteaPolicyOkToTestRetest test the ok-to-test and retest policy
// create a CRD which a policy allowing only users in the team /ok-to-test to allow a PR
// we create a org
// we create a team ok-to-test on org and add the user ok-to-test-$RANDOM onto it
// we create a team normal on org and add the user normal-$RANDOM onto it
// we issue a /ok-to-test as user normal and check it was denied
// we delete the old pac comment to make the pac reliable checking it was denied.
// we issue a /retest as user normal and check it was denied
// we issue a /ok-to-test as user ok-to-test and check it was succeeded
//
// this test paths is mostly to test the logic in the pkg/policy package, there
// is not much gitea specifics compared to github
func TestGiteaPolicyOkToTestRetest(t *testing.T) {
	topts := &tgitea.TestOpts{
		Regexp:          regexp.MustCompile(fmt.Sprintf(`.*User %s is not allowed to run CI on this repo`, os.Getenv("TEST_GITEA_USERNAME"))),
		OnOrg:           true,
		SkipEventsCheck: true,
		TargetEvent:     options.PullRequestEvent,
		Settings: &v1alpha1.Settings{
			Policy: &v1alpha1.Policy{
				OkToTest: []string{"ok-to-test"},
			},
		},
		YAMLFiles: map[string]string{".tekton/pr.yaml": "testdata/pipelinerun.yaml"},
	}
	tgitea.TestPR(t, topts)
	topts.ParamsRun.Clients.Log.Infof("Repo CRD %s has been created with Policy: %+v", topts.TargetRefName, topts.Settings.Policy)

	orgName := "org-" + topts.TargetRefName
	adminCNX := topts.GiteaCNX

	// create ok-to-test team on org and add user ok-to-test onto it
	oktotestTeam, err := tgitea.CreateTeam(topts, orgName, "ok-to-test")
	assert.NilError(t, err)
	okToTestUserNamePasswd := fmt.Sprintf("ok-to-test-%s", topts.TargetRefName)
	okToTestUserCnx, okToTestUser, err := tgitea.CreateGiteaUserSecondCnx(topts, okToTestUserNamePasswd, okToTestUserNamePasswd)
	assert.NilError(t, err)
	_, err = topts.GiteaCNX.Client.AddTeamMember(oktotestTeam.ID, okToTestUser.UserName)
	assert.NilError(t, err)
	topts.ParamsRun.Clients.Log.Infof("User %s has been added to team %s", okToTestUser.UserName, oktotestTeam.Name)

	// create normal team on org and add user normal onto it
	normalTeam, err := tgitea.CreateTeam(topts, orgName, "normal")
	assert.NilError(t, err)
	normalUserNamePasswd := fmt.Sprintf("normal-%s", topts.TargetRefName)
	normalUserCnx, normalUser, err := tgitea.CreateGiteaUserSecondCnx(topts, normalUserNamePasswd, normalUserNamePasswd)
	assert.NilError(t, err)
	_, err = topts.GiteaCNX.Client.AddTeamMember(normalTeam.ID, normalUser.UserName)
	assert.NilError(t, err)
	topts.ParamsRun.Clients.Log.Infof("User %s has been added to team %s", normalUser.UserName, normalTeam.Name)

	topts.ParamsRun.Clients.Log.Infof("Sending a /ok-to-test comment as a user not belonging to an allowed team in Repo CR policy but part of the organization")
	topts.GiteaCNX = normalUserCnx
	tgitea.PostCommentOnPullRequest(t, topts, "/ok-to-test")
	topts.Regexp = regexp.MustCompile(
		fmt.Sprintf(`.*policy check: ok-to-test, user: %s is not a member of any of the allowed teams.*`, normalUserNamePasswd))
	topts.CheckForStatus = "failure"
	tgitea.WaitForPullRequestCommentMatch(t, topts)
	tgitea.WaitForStatus(t, topts, "heads/"+topts.TargetRefName, settings.PACApplicationNameDefaultValue, true)

	// make sure we delete the old comment to don't have a false positive
	topts.GiteaCNX = adminCNX
	assert.NilError(t, tgitea.RemoveCommentMatching(topts, regexp.MustCompile(`.*policy check:`)))

	topts.ParamsRun.Clients.Log.Infof("Sending a /retest comment as a user not belonging to an allowed team in Repo CR policy but part of the organization")
	topts.GiteaCNX = normalUserCnx
	tgitea.PostCommentOnPullRequest(t, topts, "/retest")
	topts.Regexp = regexp.MustCompile(
		fmt.Sprintf(`.*policy check: retest, user: %s is not a member of any of the allowed teams.*`, normalUserNamePasswd))
	topts.CheckForStatus = "failure"
	tgitea.WaitForPullRequestCommentMatch(t, topts)

	tgitea.WaitForStatus(t, topts, "heads/"+topts.TargetRefName, settings.PACApplicationNameDefaultValue, true)
	topts.GiteaCNX = okToTestUserCnx
	topts.ParamsRun.Clients.Log.Infof("Sending a /ok-to-test comment as a user belonging to an allowed team in Repo CR policy")
	tgitea.PostCommentOnPullRequest(t, topts, "/ok-to-test")
	topts.Regexp = successRegexp
	topts.CheckForStatus = "success"
	tgitea.WaitForPullRequestCommentMatch(t, topts)
	tgitea.WaitForStatus(t, topts, "heads/"+topts.TargetRefName, "", true)
}

// TestGiteaACLOrgAllowed tests that the policy check works when the user is part of an allowed org
func TestGiteaACLOrgAllowed(t *testing.T) {
	topts := &tgitea.TestOpts{
		TargetEvent: options.PullRequestEvent,
		YAMLFiles: map[string]string{
			".tekton/pr.yaml": "testdata/pipelinerun.yaml",
		},
		NoCleanup:    true,
		ExpectEvents: false,
	}
	defer tgitea.TestPR(t, topts)()
	secondcnx, _, err := tgitea.CreateGiteaUserSecondCnx(topts, topts.TargetRefName, topts.GiteaPassword)
	assert.NilError(t, err)

	tgitea.CreateForkPullRequest(t, topts, secondcnx, "read", "echo Hello from user "+topts.TargetRefName)
	topts.CheckForStatus = "success"
	tgitea.WaitForStatus(t, topts, "heads/"+topts.TargetRefName, "", false)
}

// TestGiteaACLOrgPendingApproval tests when non authorized user sends a PR the status of CI shows as pending.
func TestGiteaACLOrgPendingApproval(t *testing.T) {
	topts := &tgitea.TestOpts{
		TargetEvent: options.PullRequestEvent,
		YAMLFiles: map[string]string{
			".tekton/pr.yaml": "testdata/pipelinerun.yaml",
		},
		NoCleanup:    true,
		ExpectEvents: false,
	}
	defer tgitea.TestPR(t, topts)()
	secondcnx, _, err := tgitea.CreateGiteaUserSecondCnx(topts, topts.TargetRefName, topts.GiteaPassword)
	assert.NilError(t, err)

	topts.PullRequest = tgitea.CreateForkPullRequest(t, topts, secondcnx, "", "echo Hello from user "+topts.TargetRefName)
	topts.CheckForStatus = "Skipped"
	tgitea.WaitForStatus(t, topts, topts.PullRequest.Head.Sha, "", false)
	topts.Regexp = regexp.MustCompile(`.*is skipping this commit.*`)
	tgitea.WaitForPullRequestCommentMatch(t, topts)
}

// TestGiteaACLCommentsAllowing tests that the gitops comment commands work
func TestGiteaACLCommentsAllowing(t *testing.T) {
	tests := []struct {
		name, comment string
	}{
		{
			name:    "OK to Test",
			comment: "/ok-to-test",
		},
		{
			name:    "Retest",
			comment: "/retest",
		},
		{
			name:    "Test PR",
			comment: "/test pr",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			topts := &tgitea.TestOpts{
				TargetEvent: options.PullRequestEvent,
				YAMLFiles: map[string]string{
					".tekton/pr.yaml": "testdata/pipelinerun.yaml",
				},
				NoCleanup:    true,
				ExpectEvents: false,
			}
			defer tgitea.TestPR(t, topts)()
			secondcnx, _, err := tgitea.CreateGiteaUserSecondCnx(topts, topts.TargetRefName, topts.GiteaPassword)
			assert.NilError(t, err)

			topts.PullRequest = tgitea.CreateForkPullRequest(t, topts, secondcnx, "", "echo Hello from user "+topts.TargetRefName)
			topts.CheckForStatus = "Skipped"
			tgitea.WaitForStatus(t, topts, topts.PullRequest.Head.Sha, "", false)
			topts.Regexp = regexp.MustCompile(`.*is skipping this commit.*`)
			tgitea.WaitForPullRequestCommentMatch(t, topts)

			tgitea.PostCommentOnPullRequest(t, topts, tt.comment)
			topts.Regexp = successRegexp
			tgitea.WaitForPullRequestCommentMatch(t, topts)
		})
	}
}

// Local Variables:
// compile-command: "go test -tags=e2e -v -run TestGiteaPush ."
// End:
