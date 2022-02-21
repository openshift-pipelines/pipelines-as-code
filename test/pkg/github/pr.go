package github

import (
	"context"
	"encoding/base64"

	"github.com/google/go-github/v42/github"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	ghprovider "github.com/openshift-pipelines/pipelines-as-code/pkg/provider/github"
)

func PushFilesToRef(ctx context.Context, client *github.Client, commitMessage, baseBranch, targetRef, owner, repo string, files map[string]string) (string, error) {
	maintree, _, err := client.Git.GetTree(ctx, owner, repo, baseBranch, false)
	if err != nil {
		return "", err
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
			return "", err
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
		return "", err
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
	})
	if err != nil {
		return "", err
	}

	ref := &github.Reference{
		Ref: &targetRef,
		Object: &github.GitObject{
			SHA: commit.SHA,
		},
	}
	_, _, err = client.Git.CreateRef(ctx, owner, repo, ref)
	if err != nil {
		return "", err
	}

	return commit.GetSHA(), nil
}

func PRCreate(ctx context.Context, cs *params.Run, ghcnx ghprovider.Provider, owner, repo, targetRef, defaultBranch, title string) (int, error) {
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
