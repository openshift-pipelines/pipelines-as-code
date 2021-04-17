package webvcs

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/google/go-github/v34/github"
	"golang.org/x/oauth2"
)

type GithubVCS struct {
	Context context.Context
	Client  *github.Client
}

type RunInfo struct {
	Owner         string
	Repository    string
	DefaultBranch string
	SHA           string
	URL           string
	Branch        string
}

func NewGithubVCS(token string) GithubVCS {
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(ctx, ts)
	return GithubVCS{
		Client:  github.NewClient(tc),
		Context: ctx,
	}
}

func (v GithubVCS) ParsePayload(payload string) (RunInfo, error) {
	prMap := &github.PullRequestEvent{}
	err := json.Unmarshal([]byte(payload), prMap)
	if err != nil {
		return RunInfo{}, err
	}

	if prMap.PullRequest == nil {
		return RunInfo{}, errors.New("Cannot parse payload as PR")
	}

	return RunInfo{
		Owner:         prMap.GetRepo().Owner.GetLogin(),
		Repository:    prMap.GetRepo().GetName(),
		URL:           prMap.GetRepo().GetHTMLURL(),
		DefaultBranch: prMap.GetRepo().GetDefaultBranch(),
		// TODO: this is going to be different on merge
		SHA:    prMap.PullRequest.Head.GetSHA(),
		Branch: prMap.PullRequest.Base.GetRef(),
	}, nil
}

func (v GithubVCS) GetTektonDir(path string, runinfo RunInfo) ([]*github.RepositoryContent, error) {
	fp, objects, resp, err := v.Client.Repositories.GetContents(v.Context, runinfo.Owner,
		runinfo.Repository, path, &github.RepositoryContentGetOptions{Ref: runinfo.SHA})
	if fp != nil {
		return nil, fmt.Errorf("The object %s is a file instead of a directory", path)
	}
	if resp.Response.StatusCode == http.StatusNotFound {
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	return objects, nil
}

func (v GithubVCS) GetObject(sha string, runinfo RunInfo) ([]byte, error) {
	blob, _, err := v.Client.Git.GetBlob(v.Context, runinfo.Owner, runinfo.Repository, sha)
	if err != nil {
		return nil, err
	}
	decoded, err := base64.StdEncoding.DecodeString(blob.GetContent())
	if err != nil {
		return nil, err
	}

	return decoded, err
}
