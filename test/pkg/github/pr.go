package github

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/google/go-github/v56/github"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	ghprovider "github.com/openshift-pipelines/pipelines-as-code/pkg/provider/github"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/options"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/payload"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/wait"
	"github.com/tektoncd/pipeline/pkg/names"
	"gotest.tools/v3/assert"
)

func PushFilesToRef(ctx context.Context, client *github.Client, commitMessage, baseBranch, targetRef, owner, repo string, files map[string]string) (string, *github.Reference, error) {
	maintree, _, err := client.Git.GetTree(ctx, owner, repo, baseBranch, false)
	if err != nil {
		return "", nil, fmt.Errorf("error getting tree: %w", err)
	}
	mainSha := maintree.GetSHA()
	entries := []*github.TreeEntry{}
	defaultMode := "100644"
	for path, fcontent := range files {
		content := base64.StdEncoding.EncodeToString([]byte(fcontent))
		encoding := "base64"
		blob, _, err := client.Git.CreateBlob(ctx, owner, repo, &github.Blob{
			Content:  &content,
			Encoding: &encoding,
		})
		if err != nil {
			return "", nil, fmt.Errorf("error creating blobs: %w", err)
		}
		sha := blob.GetSHA()

		_path := path
		entries = append(entries,
			&github.TreeEntry{
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
	commit, _, err := client.Git.CreateCommit(ctx, owner, repo, &github.Commit{
		Author: &github.CommitAuthor{
			Name:  &commitAuthor,
			Email: &commitEmail,
		},
		Message: &commitMessage,
		Tree:    tree,
		Parents: []*github.Commit{
			{
				SHA: &mainSha,
			},
		},
	}, &github.CreateCommitOptions{})
	if err != nil {
		return "", nil, err
	}

	ref := &github.Reference{
		Ref: &targetRef,
		Object: &github.GitObject{
			SHA: commit.SHA,
		},
	}
	vref, _, err := client.Git.CreateRef(ctx, owner, repo, ref)
	if err != nil {
		return "", nil, err
	}

	return commit.GetSHA(), vref, nil
}

func PRCreate(ctx context.Context, cs *params.Run, ghcnx *ghprovider.Provider, owner, repo, targetRef, defaultBranch, title string) (int, error) {
	pr, _, err := ghcnx.Client.PullRequests.Create(ctx, owner, repo, &github.NewPullRequest{
		Title: &title,
		Head:  &targetRef,
		Base:  &defaultBranch,
		Body:  github.String("Add a new PR for testing"),
	})
	if err != nil {
		return -1, err
	}
	cs.Clients.Log.Infof("Pull request created: %s", pr.GetHTMLURL())
	return pr.GetNumber(), nil
}

func RunPullRequest(ctx context.Context, t *testing.T, label string, yamlFiles []string, secondcontroller, webhook bool) (*params.Run, *ghprovider.Provider, options.E2E, string, string, int, string) {
	targetNS := names.SimpleNameGenerator.RestrictLengthWithRandomSuffix("pac-e2e-ns")

	ctx, runcnx, opts, ghcnx, err := Setup(ctx, secondcontroller, webhook)
	assert.NilError(t, err)

	logmsg := fmt.Sprintf("Testing %s with Github APPS integration on %s", label, targetNS)
	runcnx.Clients.Log.Info(logmsg)

	repoinfo, resp, err := ghcnx.Client.Repositories.Get(ctx, opts.Organization, opts.Repo)
	assert.NilError(t, err)
	if resp != nil && resp.Response.StatusCode == http.StatusNotFound {
		t.Errorf("Repository %s not found in %s", opts.Organization, opts.Repo)
	}

	err = CreateCRD(ctx, t, repoinfo, runcnx, opts, targetNS)
	assert.NilError(t, err)

	yamlEntries := map[string]string{}
	for _, v := range yamlFiles {
		yamlEntries[filepath.Join(".tekton", filepath.Base(v))] = v
	}

	entries, err := payload.GetEntries(yamlEntries, targetNS, options.MainBranch, options.PullRequestEvent,
		map[string]string{"TargetURL": repoinfo.GetHTMLURL(), "SourceURL": repoinfo.GetHTMLURL()})
	assert.NilError(t, err)

	targetRefName := fmt.Sprintf("refs/heads/%s",
		names.SimpleNameGenerator.RestrictLengthWithRandomSuffix("pac-e2e-test"))

	sha, vref, err := PushFilesToRef(ctx, ghcnx.Client, logmsg, repoinfo.GetDefaultBranch(), targetRefName,
		opts.Organization, opts.Repo, entries)
	assert.NilError(t, err)
	runcnx.Clients.Log.Infof("Commit %s has been created and pushed to %s", sha, vref.GetURL())
	number, err := PRCreate(ctx, runcnx, ghcnx, opts.Organization,
		opts.Repo, targetRefName, repoinfo.GetDefaultBranch(), logmsg)
	assert.NilError(t, err)

	sopt := wait.SuccessOpt{
		Title:           logmsg,
		OnEvent:         options.PullRequestEvent,
		TargetNS:        targetNS,
		NumberofPRMatch: len(yamlFiles),
		SHA:             sha,
	}
	wait.Succeeded(ctx, t, runcnx, opts, sopt)
	return runcnx, ghcnx, opts, targetNS, targetRefName, number, sha
}

func RunPushRequest(ctx context.Context, t *testing.T, label string, yamlFiles []string, onSecondController, onWebhook bool) (*params.Run, *ghprovider.Provider, options.E2E, string, string, int, string) {
	targetNS := names.SimpleNameGenerator.RestrictLengthWithRandomSuffix("pac-e2e-push")
	targetBranch := targetNS
	targetEvent := "push"
	ctx, runcnx, opts, ghcnx, err := Setup(ctx, onSecondController, onWebhook)
	assert.NilError(t, err)

	var logmsg string
	if onWebhook {
		logmsg = fmt.Sprintf("Testing %s with Direct Webhook integration on %s", label, targetNS)
		runcnx.Clients.Log.Info(logmsg)
	} else {
		logmsg = fmt.Sprintf("Testing %s with Github APPS integration on %s", label, targetNS)
		runcnx.Clients.Log.Info(logmsg)
	}
	repoinfo, resp, err := ghcnx.Client.Repositories.Get(ctx, opts.Organization, opts.Repo)
	assert.NilError(t, err)
	if resp != nil && resp.StatusCode == http.StatusNotFound {
		t.Errorf("Repository %s not found in %s", opts.Organization, opts.Repo)
	}
	err = CreateCRD(ctx, t, repoinfo, runcnx, opts, targetNS)
	assert.NilError(t, err)

	yamlEntries := map[string]string{}
	for _, v := range yamlFiles {
		yamlEntries[filepath.Join(".tekton", filepath.Base(v))] = v
	}

	entries, err := payload.GetEntries(yamlEntries,
		targetNS, targetBranch, targetEvent, map[string]string{})
	assert.NilError(t, err)

	targetRefName := fmt.Sprintf("refs/heads/%s", targetBranch)
	sha, vref, err := PushFilesToRef(ctx, ghcnx.Client, logmsg, repoinfo.GetDefaultBranch(), targetRefName, opts.Organization, opts.Repo, entries)
	runcnx.Clients.Log.Infof("Commit %s has been created and pushed to %s", sha, vref.GetURL())
	assert.NilError(t, err)

	sopt := wait.SuccessOpt{
		Title:           logmsg,
		OnEvent:         options.PushEvent,
		TargetNS:        targetNS,
		NumberofPRMatch: len(yamlFiles),
		SHA:             sha,
	}
	wait.Succeeded(ctx, t, runcnx, opts, sopt)
	return runcnx, ghcnx, opts, targetNS, targetRefName, -1, sha
}
