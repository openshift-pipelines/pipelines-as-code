package gitea

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"code.gitea.io/sdk/gitea"
	"github.com/google/go-github/v52/github"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/git"
	pgitea "github.com/openshift-pipelines/pipelines-as-code/pkg/provider/gitea"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/payload"
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

func PushFilesToRefGit(t *testing.T, topts *TestOpts, entries map[string]string, baseRefFrom string) {
	tmpdir := fs.NewDir(t, t.Name())
	defer tmpdir.Remove()
	defer env.ChangeWorkingDir(t, tmpdir.Path())()
	path := tmpdir.Path()
	_, err := git.RunGit(path, "init")
	assert.NilError(t, err)

	_, err = git.RunGit(path, "config", "user.name", "OpenShift Pipelines E2E test")
	assert.NilError(t, err)
	_, err = git.RunGit(path, "config", "user.email", "e2e-pipeline@redhat.com")
	assert.NilError(t, err)

	_, err = git.RunGit(path, "remote", "add", "-f", "origin", topts.GitCloneURL)
	assert.NilError(t, err)

	_, err = git.RunGit(path, "fetch", "-a", "origin")
	assert.NilError(t, err)

	_, err = git.RunGit(path, "checkout", "-B", topts.TargetRefName, "origin/"+baseRefFrom)
	assert.NilError(t, err)

	for filename, content := range entries {
		assert.NilError(t, os.MkdirAll(filepath.Dir(filename), 0o755))
		// write content to filename
		assert.NilError(t, os.WriteFile(filename, []byte(content), 0o600))
	}
	_, err = git.RunGit(path, "add", ".")
	assert.NilError(t, err)

	_, err = git.RunGit(path, "-c", "commit.gpgsign=false", "commit", "-m", "Committing files from test on "+topts.TargetRefName)
	assert.NilError(t, err)

	_, err = git.RunGit(path, "push", "origin", topts.TargetRefName)
	assert.NilError(t, err)
	// parse url topts.GitURL
	topts.ParamsRun.Clients.Log.Infof("Pushed files to repo %s branch %s", topts.GitHTMLURL, topts.TargetRefName)
}

// Make a clone url with username and password
func MakeGitCloneURL(targetURL, giteaUsername, giteaPassword string) (string, error) {
	// parse hostname of giteaURL
	parsedURL, err := url.Parse(targetURL)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s://%s:%s@%s%s", parsedURL.Scheme, giteaUsername, giteaPassword, parsedURL.Host, parsedURL.Path), nil
}

// PushFilesToRefAPI will push files to a given ref via API
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
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
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

func CreateGiteaOrg(giteaClient *gitea.Client, org, user string) (*gitea.Organization, error) {
	ret, _, err := giteaClient.AdminCreateOrg(user, gitea.CreateOrgOption{
		Name:                      org,
		Visibility:                "public",
		RepoAdminChangeTeamAccess: false,
	})
	return ret, err
}

func CreateGiteaRepo(giteaClient *gitea.Client, user, name, hookURL string) (*gitea.Repository, error) {
	// Create a new repo
	repo, _, err := giteaClient.AdminCreateRepo(user, gitea.CreateRepoOption{
		Name:        name,
		Description: "This is a repo it's a wonderful thing",
		AutoInit:    true,
	})
	if err != nil {
		return nil, err
	}
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

func CreateGiteaUser(giteaClient *gitea.Client, apiURL, username, password string) (pgitea.Provider, error) {
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
		return pgitea.Provider{}, err
	}
	secondprovider, err := CreateProvider(context.Background(),
		apiURL, newuser.UserName, password)
	if err != nil {
		return pgitea.Provider{}, err
	}
	return secondprovider, err
}

func CreateForkPullRequest(t *testing.T, topts *TestOpts, secondcnx pgitea.Provider,
	accessMode, command string,
) *gitea.PullRequest {
	forkrepo, _, err := secondcnx.Client.CreateFork(topts.Opts.Organization, topts.TargetRefName,
		gitea.CreateForkOption{})
	assert.NilError(t, err)
	cloneURL, err := MakeGitCloneURL(forkrepo.CloneURL, topts.TargetRefName, topts.GiteaPassword)
	assert.NilError(t, err)
	newopts := &TestOpts{
		GitCloneURL: cloneURL, TargetRefName: topts.TargetRefName, ParamsRun: topts.ParamsRun,
	}
	processed, err := payload.ApplyTemplate("testdata/pipelinerun-alt.yaml", map[string]string{
		"TargetNamespace": topts.TargetNS,
		"TargetEvent":     topts.TargetEvent,
		"TargetBranch":    topts.TargetRefName,
		"PipelineName":    "pr",
		"Command":         command,
	})
	assert.NilError(t, err)
	entries := map[string]string{".tekton/pr.yaml": processed}
	PushFilesToRefGit(t, newopts, entries, topts.TargetRefName)

	if accessMode != "" {
		assert.NilError(t, CreateAccess(topts, topts.TargetRefName, accessMode))
	}

	pr, _, err := secondcnx.Client.CreatePullRequest(topts.Opts.Organization, topts.TargetRefName,
		gitea.CreatePullRequestOption{
			Head:  fmt.Sprintf("%s:%s", topts.TargetRefName, topts.TargetRefName),
			Base:  topts.TargetRefName,
			Title: fmt.Sprintf("New PR from %s", topts.TargetRefName),
		})
	assert.NilError(t, err)
	topts.ParamsRun.Clients.Log.Infof("Created pr %s", pr.HTMLURL)
	return pr
}

func CreateAccess(topts *TestOpts, touser, accessMode string) error {
	accessmode := gitea.AccessMode(accessMode)
	_, err := topts.GiteaCNX.Client.AddCollaborator(topts.Opts.Organization, topts.Opts.Repo, touser,
		gitea.AddCollaboratorOption{
			Permission: &accessmode,
		})
	return err
}
