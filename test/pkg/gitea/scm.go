package gitea

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"testing"

	"code.gitea.io/sdk/gitea"
	"github.com/google/go-github/v59/github"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/git"
	pgitea "github.com/openshift-pipelines/pipelines-as-code/pkg/provider/gitea"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/payload"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/scm"
	"go.uber.org/zap"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/env"
	"gotest.tools/v3/fs"
)

func InitGitRepo(t *testing.T) (string, func()) {
	tmpdir := fs.NewDir(t, t.Name())
	envCleanups := env.ChangeWorkingDir(t, tmpdir.Path())
	path := tmpdir.Path()
	_, err := git.RunGit(path, "init")
	assert.NilError(t, err)

	_, err = git.RunGit(path, "config", "user.name", "OpenShift Pipelines E2E test")
	assert.NilError(t, err)
	_, err = git.RunGit(path, "config", "user.email", "e2e-pipeline@redhat.com")
	assert.NilError(t, err)
	return path, func() {
		tmpdir.Remove()
		envCleanups()
	}
}

// PushFilesToRefAPI will push files to a given ref via API.
func PushFilesToRefAPI(t *testing.T, topts *TestOpts, entries map[string]string) (string, error) {
	var sha string
	commitAuthor := "OpenShift Pipelines E2E test"
	commitEmail := "e2e-pipelines@redhat.com"

	firstOne := true
	for filename, content := range entries {
		// encode content as base64
		bContent := base64.StdEncoding.EncodeToString([]byte(content))
		fromBranch := topts.TargetRefName
		if firstOne {
			fromBranch = topts.DefaultBranch
			firstOne = false
		}
		fOpts := gitea.CreateFileOptions{
			Content: bContent,
			FileOptions: gitea.FileOptions{
				Message:       "Committing " + filename,
				BranchName:    fromBranch,
				NewBranchName: topts.TargetRefName,
				Author:        gitea.Identity{Name: commitAuthor, Email: commitEmail},
			},
		}
		fr, _, err := topts.GiteaCNX.Client.CreateFile(topts.Opts.Organization, topts.Opts.Repo, filename, fOpts)
		sha = fr.Commit.SHA
		assert.NilError(t, err)
	}
	return sha, nil
}

func MakeRequest(ctx context.Context, httpClient http.Client, url, username, password string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(username, password)
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

type Timelines []struct {
	Body string `json:"body"`
}

func GetIssueTimeline(ctx context.Context, topts *TestOpts) (Timelines, error) {
	timelineURL := fmt.Sprintf("%s/api/v1/repos/%s/issues/%d/timeline", topts.GiteaAPIURL,
		topts.PullRequest.Base.Repository.FullName, topts.PullRequest.Index)
	resp, err := MakeRequest(ctx, topts.ParamsRun.Clients.HTTP, timelineURL, topts.Opts.Organization, topts.GiteaPassword)
	if err != nil {
		return nil, fmt.Errorf("error on URL %s: %w", timelineURL, err)
	}
	defer resp.Body.Close()
	var tls Timelines
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get issue timeline: %s", resp.Status)
	}

	err = json.NewDecoder(resp.Body).Decode(&tls)
	if err != nil {
		return nil, err
	}
	return tls, nil
}

func CreateGiteaRepo(giteaClient *gitea.Client, user, name, hookURL string, onOrg bool, logger *zap.SugaredLogger) (*gitea.Repository, error) {
	var repo *gitea.Repository
	var err error
	// Create a new repo
	if onOrg {
		logger.Infof("Creating org %s", name)
		user = "org-" + name
		_, _, err := giteaClient.CreateOrg(gitea.CreateOrgOption{
			Name: user,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create org: %w", err)
		}
		logger.Infof("Creating gitea repository on org %s", name)
		repo, _, err = giteaClient.CreateOrgRepo(user, gitea.CreateRepoOption{
			Name:        name,
			Description: "This is a repo it's a wonderful thing",
			AutoInit:    true,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create repo: %w", err)
		}
	} else {
		logger.Infof("Creating gitea repository %s for user %s", name, user)
		repo, _, err = giteaClient.AdminCreateRepo(user, gitea.CreateRepoOption{
			Name:        name,
			Description: "This is a repo it's a wonderful thing",
			AutoInit:    true,
		})
	}
	if err != nil {
		return nil, fmt.Errorf("failed to create repo: %w", err)
	}
	logger.Infof("Creating webhook to smee url on gitea repository %s", name)
	_, _, err = giteaClient.CreateRepoHook(user, repo.Name, gitea.CreateHookOption{
		Type:   "gitea",
		Active: true,
		Config: map[string]string{
			"name":         "hook to smee url",
			"url":          hookURL,
			"content_type": "json",
		},
		Events: []string{"push", "issue_comments", "pull_request"},
	})
	return repo, err
}

func GetGiteaRepo(giteaClient *gitea.Client, user, name string, _ *zap.SugaredLogger) (*gitea.Repository, error) {
	var repo *gitea.Repository
	var err error
	repo, _, err = giteaClient.GetRepo(user, name)
	if err != nil {
		return nil, fmt.Errorf("failed to get repo: %w", err)
	}
	return repo, err
}

func CreateTeam(topts *TestOpts, orgName, teamName string) (*gitea.Team, error) {
	team, _, err := topts.GiteaCNX.Client.CreateTeam(orgName, gitea.CreateTeamOption{
		Permission: gitea.AccessModeWrite,
		Units: []gitea.RepoUnitType{
			gitea.RepoUnitPulls,
		},
		Name: teamName,
	})
	topts.ParamsRun.Clients.Log.Infof("Team %s has been created on Org %s", team.Name, orgName)
	return team, err
}

func RemoveCommentMatching(topts *TestOpts, commentString *regexp.Regexp) error {
	comments, _, err := topts.GiteaCNX.Client.ListIssueComments(topts.Opts.Organization, topts.Opts.Repo, topts.PullRequest.Index, gitea.ListIssueCommentOptions{})
	if err != nil {
		return err
	}
	for _, c := range comments {
		if commentString.MatchString(c.Body) {
			topts.ParamsRun.Clients.Log.Infof("Removing comment %d matching %s", c.ID, commentString.String())
			_, err := topts.GiteaCNX.Client.DeleteIssueComment(topts.Opts.Organization, topts.Opts.Repo, c.ID)
			return err
		}
	}
	return fmt.Errorf("no comment matching %s found", commentString.String())
}

func CreateGiteaUser(giteaClient *gitea.Client, username, password string) (*gitea.User, error) {
	visibility := gitea.VisibleTypePublic
	opts := gitea.CreateUserOption{
		LoginName:          username,
		Username:           username,
		Email:              username + "@redhat.com",
		Password:           password,
		MustChangePassword: github.Bool(false),
		Visibility:         &visibility,
	}
	newuser, _, err := giteaClient.AdminCreateUser(opts)
	if err != nil {
		return &gitea.User{}, err
	}
	return newuser, nil
}

// CreateGiteaUserSecondCnx creates a new user and a new provider for this user.
func CreateGiteaUserSecondCnx(topts *TestOpts, username, password string) (pgitea.Provider, *gitea.User, error) {
	newuser, err := CreateGiteaUser(topts.GiteaCNX.Client, username, password)
	if err != nil {
		return pgitea.Provider{}, newuser, fmt.Errorf("failed to create user: %w", err)
	}
	secondprovider, err := CreateProvider(context.Background(), topts.GiteaAPIURL, newuser.UserName, password)
	if err != nil {
		return pgitea.Provider{}, newuser, fmt.Errorf("failed to create provider: %w", err)
	}
	return secondprovider, newuser, err
}

func CreateForkPullRequest(t *testing.T, topts *TestOpts, secondcnx pgitea.Provider, accessMode string) *gitea.PullRequest {
	forkrepo, _, err := secondcnx.Client.CreateFork(topts.Opts.Organization, topts.TargetRefName,
		gitea.CreateForkOption{})
	assert.NilError(t, err)
	topts.ParamsRun.Clients.Log.Infof("Forked repository %s has been created", forkrepo.CloneURL)

	if accessMode != "" {
		assert.NilError(t, CreateAccess(topts, topts.TargetRefName, accessMode))
	}

	pr, _, err := secondcnx.Client.CreatePullRequest(topts.Opts.Organization, topts.TargetRefName,
		gitea.CreatePullRequestOption{
			Base:  topts.DefaultBranch,
			Head:  fmt.Sprintf("%s:%s", forkrepo.Owner.UserName, topts.TargetRefName),
			Title: fmt.Sprintf("New PR from %s", topts.TargetRefName),
		})
	assert.NilError(t, err)
	topts.ParamsRun.Clients.Log.Infof("Created pr %s branch:%s from fork %s, branch:%s", pr.HTMLURL, topts.DefaultBranch, forkrepo.FullName, topts.TargetRefName)
	return pr
}

func PushToPullRequest(t *testing.T, topts *TestOpts, secondcnx pgitea.Provider, command string) {
	forkuserinfo, _, err := secondcnx.Client.GetMyUserInfo()
	assert.NilError(t, err)
	cloneURL, err := scm.MakeGitCloneURL(topts.PullRequest.Head.Repository.HTMLURL, forkuserinfo.UserName, secondcnx.Password)
	assert.NilError(t, err)
	processed, err := payload.ApplyTemplate("testdata/pipelinerun-alt.yaml", map[string]string{
		"TargetNamespace": topts.TargetNS,
		"TargetEvent":     topts.TargetEvent,
		"TargetBranch":    topts.TargetRefName,
		"PipelineName":    "pr-push",
		"Command":         command,
	})
	assert.NilError(t, err)
	entries := map[string]string{".tekton/pr-push.yaml": processed}
	scmOpts := &scm.Opts{
		GitURL:        cloneURL,
		Log:           topts.ParamsRun.Clients.Log,
		WebURL:        topts.GitHTMLURL,
		TargetRefName: topts.TargetRefName,
		BaseRefName:   topts.DefaultBranch,
	}
	scm.PushFilesToRefGit(t, scmOpts, entries)
}

func CreateAccess(topts *TestOpts, touser, accessMode string) error {
	accessmode := gitea.AccessMode(accessMode)
	_, err := topts.GiteaCNX.Client.AddCollaborator(topts.Opts.Organization, topts.Opts.Repo, touser,
		gitea.AddCollaboratorOption{
			Permission: &accessmode,
		})
	return err
}
