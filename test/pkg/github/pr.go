package github

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	ghlib "github.com/google/go-github/v70/github"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/triggertype"
	ghprovider "github.com/openshift-pipelines/pipelines-as-code/pkg/provider/github"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/options"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/payload"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/repository"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/scm"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/wait"
	"github.com/tektoncd/pipeline/pkg/names"
	"go.uber.org/zap"
	"gotest.tools/v3/assert"
)

func PushFilesToRef(ctx context.Context, client *ghlib.Client, commitMessage, baseBranch, targetRef, owner, repo string, files map[string]string) (string, *ghlib.Reference, error) {
	maintree, _, err := client.Git.GetTree(ctx, owner, repo, baseBranch, false)
	if err != nil {
		return "", nil, fmt.Errorf("error getting tree: %w", err)
	}
	mainSha := maintree.GetSHA()
	entries := []*ghlib.TreeEntry{}
	defaultMode := "100644"
	for path, fcontent := range files {
		content := base64.StdEncoding.EncodeToString([]byte(fcontent))
		encoding := "base64"
		blob, _, err := client.Git.CreateBlob(ctx, owner, repo, &ghlib.Blob{
			Content:  &content,
			Encoding: &encoding,
		})
		if err != nil {
			return "", nil, fmt.Errorf("error creating blobs: %w", err)
		}
		sha := blob.GetSHA()

		_path := path
		entries = append(entries,
			&ghlib.TreeEntry{
				Path: &_path,
				Mode: &defaultMode,
				SHA:  &sha,
			})
	}

	tree, _, err := client.Git.CreateTree(ctx, owner, repo, mainSha, entries)
	if err != nil {
		return "", nil, err
	}

	commitAuthor := "OpenShift Pipelines E2E test"
	commitEmail := "e2e-pipelines@redhat.com"
	commit, _, err := client.Git.CreateCommit(ctx, owner, repo, &ghlib.Commit{
		Author: &ghlib.CommitAuthor{
			Name:  &commitAuthor,
			Email: &commitEmail,
		},
		Message: &commitMessage,
		Tree:    tree,
		Parents: []*ghlib.Commit{
			{
				SHA: &mainSha,
			},
		},
	}, &ghlib.CreateCommitOptions{})
	if err != nil {
		return "", nil, fmt.Errorf("error creating commit: %w", err)
	}

	ref := &ghlib.Reference{
		Ref: &targetRef,
		Object: &ghlib.GitObject{
			SHA: commit.SHA,
		},
	}
	vref, _, err := client.Git.CreateRef(ctx, owner, repo, ref)
	if err != nil {
		return "", nil, fmt.Errorf("error creating ref: %w", err)
	}
	return commit.GetSHA(), vref, nil
}

func PRCreate(ctx context.Context, cs *params.Run, ghcnx *ghprovider.Provider, owner, repo, targetRef, defaultBranch, title string) (int, error) {
	pr, _, err := ghcnx.Client().PullRequests.Create(ctx, owner, repo, &ghlib.NewPullRequest{
		Title: &title,
		Head:  &targetRef,
		Base:  &defaultBranch,
		Body:  ghlib.Ptr("Add a new PR for testing"),
	})
	if err != nil {
		return -1, err
	}
	cs.Clients.Log.Infof("Pull request created: %s", pr.GetHTMLURL())
	return pr.GetNumber(), nil
}

type PRTest struct {
	Label            string
	YamlFiles        []string
	SecondController bool
	Webhook          bool
	NoStatusCheck    bool

	Cnx             *params.Run
	Options         options.E2E
	Provider        *ghprovider.Provider
	TargetNamespace string
	TargetRefName   string
	PRNumber        int
	SHA             string
	Logger          *zap.SugaredLogger
	CommitTitle     string
}

func (g *PRTest) RunPullRequest(ctx context.Context, t *testing.T) {
	targetNS := names.SimpleNameGenerator.RestrictLengthWithRandomSuffix("pac-e2e-ns")

	ctx, runcnx, opts, ghcnx, err := Setup(ctx, g.SecondController, g.Webhook)
	assert.NilError(t, err)
	g.Logger = runcnx.Clients.Log

	g.CommitTitle = fmt.Sprintf("Testing %s with Github APPS integration on %s", g.Label, targetNS)
	g.Logger.Info(g.CommitTitle)

	repoinfo, resp, err := ghcnx.Client().Repositories.Get(ctx, opts.Organization, opts.Repo)
	assert.NilError(t, err)
	if resp != nil && resp.StatusCode == http.StatusNotFound {
		t.Errorf("Repository %s not found in %s", opts.Organization, opts.Repo)
	}

	err = CreateCRD(ctx, t, repoinfo, runcnx, opts, targetNS)
	assert.NilError(t, err)

	yamlEntries := map[string]string{}
	for _, v := range g.YamlFiles {
		yamlEntries[filepath.Join(".tekton", filepath.Base(v))] = v
	}

	entries, err := payload.GetEntries(yamlEntries, targetNS, options.MainBranch, triggertype.PullRequest.String(),
		map[string]string{"TargetURL": repoinfo.GetHTMLURL(), "SourceURL": repoinfo.GetHTMLURL()})
	assert.NilError(t, err)

	targetRefName := fmt.Sprintf("refs/heads/%s",
		names.SimpleNameGenerator.RestrictLengthWithRandomSuffix("pac-e2e-test"))

	sha, vref, err := PushFilesToRef(ctx, ghcnx.Client(), g.CommitTitle, repoinfo.GetDefaultBranch(), targetRefName,
		opts.Organization, opts.Repo, entries)
	assert.NilError(t, err)

	g.Logger.Infof("Commit %s has been created and pushed to %s", sha, vref.GetURL())
	number, err := PRCreate(ctx, runcnx, ghcnx, opts.Organization,
		opts.Repo, targetRefName, repoinfo.GetDefaultBranch(), g.CommitTitle)
	assert.NilError(t, err)

	if !g.NoStatusCheck {
		sopt := wait.SuccessOpt{
			Title:           g.CommitTitle,
			OnEvent:         triggertype.PullRequest.String(),
			TargetNS:        targetNS,
			NumberofPRMatch: len(g.YamlFiles),
			SHA:             sha,
		}
		wait.Succeeded(ctx, t, runcnx, opts, sopt)
	}
	g.Cnx = runcnx
	g.Options = opts
	g.Provider = ghcnx
	g.TargetNamespace = targetNS
	g.TargetRefName = targetRefName
	g.PRNumber = number
	g.SHA = sha
}

func (g *PRTest) TearDown(ctx context.Context, t *testing.T) {
	if os.Getenv("TEST_NOCLEANUP") == "true" {
		g.Logger.Infof("Not cleaning up and closing PR since TEST_NOCLEANUP is set")
		return
	}
	g.Logger.Infof("Closing PR %d", g.PRNumber)
	if g.PRNumber != -1 {
		state := "closed"
		_, _, err := g.Provider.Client().PullRequests.Edit(ctx,
			g.Options.Organization, g.Options.Repo, g.PRNumber,
			&ghlib.PullRequest{State: &state})
		if err != nil {
			t.Fatal(err)
		}
	}
	if g.TargetNamespace != "" {
		repository.NSTearDown(ctx, t, g.Cnx, g.TargetNamespace)
	}
	if g.TargetRefName != options.MainBranch {
		branch := fmt.Sprintf("heads/%s", filepath.Base(g.TargetRefName))
		g.Logger.Infof("Deleting Ref %s", branch)
		_, err := g.Provider.Client().Git.DeleteRef(ctx, g.Options.Organization, g.Options.Repo, branch)
		assert.NilError(t, err)
	}
}

func (g *PRTest) RunPushRequest(ctx context.Context, t *testing.T) {
	targetNS := names.SimpleNameGenerator.RestrictLengthWithRandomSuffix("pac-e2e-push")

	targetBranch := g.TargetRefName
	if targetBranch == "" {
		targetBranch = targetNS
	}
	targetEvent := "push"
	ctx, runcnx, opts, ghcnx, err := Setup(ctx, g.SecondController, g.Webhook)
	assert.NilError(t, err)
	g.Logger = runcnx.Clients.Log

	var logmsg string
	if g.Webhook {
		logmsg = fmt.Sprintf("Testing %s with Direct Webhook integration on %s", g.Label, targetNS)
		g.Logger.Info(logmsg)
	} else {
		logmsg = fmt.Sprintf("Testing %s with Github APPS integration on %s", g.Label, targetNS)
		g.Logger.Info(logmsg)
	}
	repoinfo, resp, err := ghcnx.Client().Repositories.Get(ctx, opts.Organization, opts.Repo)
	assert.NilError(t, err)
	if resp != nil && resp.StatusCode == http.StatusNotFound {
		t.Errorf("Repository %s not found in %s", opts.Organization, opts.Repo)
	}
	err = CreateCRD(ctx, t, repoinfo, runcnx, opts, targetNS)
	assert.NilError(t, err)

	yamlEntries := map[string]string{}
	for _, v := range g.YamlFiles {
		yamlEntries[filepath.Join(".tekton", filepath.Base(v))] = v
	}

	entries, err := payload.GetEntries(yamlEntries,
		targetNS, targetBranch, targetEvent, map[string]string{})
	assert.NilError(t, err)

	targetRefName := targetBranch
	cloneURL, err := scm.MakeGitCloneURL(repoinfo.GetCloneURL(), "git", *ghcnx.Token)
	assert.NilError(t, err)
	scmOpts := scm.Opts{
		GitURL:        cloneURL,
		TargetRefName: targetRefName,
		BaseRefName:   repoinfo.GetDefaultBranch(),
		WebURL:        repoinfo.GetHTMLURL(),
		Log:           runcnx.Clients.Log,
		CommitTitle:   logmsg,
	}
	scm.PushFilesToRefGit(t, &scmOpts, entries)
	branch, _, err := ghcnx.Client().Repositories.GetBranch(ctx, opts.Organization, opts.Repo, targetBranch, 1)
	assert.NilError(t, err)
	sha := branch.GetCommit().GetSHA()
	g.Logger.Infof("Commit %s has been created and pushed to %s in branch %s", sha, branch.GetCommit().GetHTMLURL(), branch.GetName())
	assert.NilError(t, err)

	if !g.NoStatusCheck {
		sopt := wait.SuccessOpt{
			Title:           logmsg,
			OnEvent:         triggertype.Push.String(),
			TargetNS:        targetNS,
			NumberofPRMatch: len(g.YamlFiles),
			SHA:             sha,
		}
		wait.Succeeded(ctx, t, runcnx, opts, sopt)
	}

	g.Cnx = runcnx
	g.Options = opts
	g.Provider = ghcnx
	g.TargetNamespace = targetNS
	g.TargetRefName = targetRefName
	g.PRNumber = -1
	g.SHA = sha
}
