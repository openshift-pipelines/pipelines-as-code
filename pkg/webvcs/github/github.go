package github

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/go-github/v39/github"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"golang.org/x/oauth2"
)

const apiPublicURL = "https://api.github.com/"

type VCS struct {
	Client        *github.Client
	Token, APIURL *string
}

func (v *VCS) GetConfig() *info.VCSConfig {
	return &info.VCSConfig{
		TaskStatusTMPL: taskStatusTemplate,
		APIURL:         apiPublicURL,
	}
}

func (v *VCS) SetClient(ctx context.Context, info *info.PacOpts) error {
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: info.VCSToken},
	)
	tc := oauth2.NewClient(ctx, ts)

	var client *github.Client
	apiURL := info.VCSAPIURL
	if apiURL != "" {
		if !strings.HasPrefix(apiURL, "https") {
			apiURL = "https://" + apiURL
		}
	}
	if apiURL != "" && apiURL != apiPublicURL {
		client, _ = github.NewEnterpriseClient(apiURL, apiURL, tc)
	} else {
		client = github.NewClient(tc)
		apiURL = client.BaseURL.String()
	}

	// Make sure Client is not already set, so we don't override our fakeclient
	// from unittesting.
	if v.Client == nil {
		v.Client = client
	}
	v.APIURL = &apiURL
	v.Token = &info.VCSToken

	return nil
}

// GetTektonDir Get all yaml files in tekton directory return as a single concated file
func (v *VCS) GetTektonDir(ctx context.Context, runevent *info.Event, path string) (string, error) {
	fp, objects, resp, err := v.Client.Repositories.GetContents(ctx, runevent.Owner,
		runevent.Repository, path, &github.RepositoryContentGetOptions{Ref: runevent.SHA})

	if fp != nil {
		return "", fmt.Errorf("the object %s is a file instead of a directory", path)
	}
	if resp != nil && resp.Response.StatusCode == http.StatusNotFound {
		return "", nil
	}

	if err != nil {
		return "", err
	}

	return v.concatAllYamlFiles(ctx, objects, runevent)
}

// GetCommitInfo get info (url and title) on a commit in runevent, this needs to
// be run after parsewebhook while we already matched a token.
func (v *VCS) GetCommitInfo(ctx context.Context, runevent *info.Event) error {
	commit, _, err := v.Client.Git.GetCommit(ctx, runevent.Owner, runevent.Repository, runevent.SHA)
	if err != nil {
		return err
	}

	runevent.SHAURL = commit.GetHTMLURL()
	runevent.SHATitle = strings.Split(commit.GetMessage(), "\n\n")[0]

	return nil
}

// GetFileInsideRepo Get a file via Github API using the runinfo information, we
// branch is true, the user the branch as ref isntead of the SHA
// TODO: merge GetFileInsideRepo amd GetTektonDir
func (v *VCS) GetFileInsideRepo(ctx context.Context, runevent *info.Event, path, target string) (string,
	error) {
	ref := runevent.SHA
	if target != "" {
		ref = runevent.BaseBranch
	}

	fp, objects, resp, err := v.Client.Repositories.GetContents(ctx, runevent.Owner,
		runevent.Repository, path, &github.RepositoryContentGetOptions{Ref: ref})
	if err != nil {
		return "", err
	}
	if objects != nil {
		return "", fmt.Errorf("referenced file inside the Github Repository %s is a directory", path)
	}
	if resp.Response.StatusCode == http.StatusNotFound {
		return "", fmt.Errorf("cannot find %s in this repository", path)
	}

	getobj, err := v.getObject(ctx, fp.GetSHA(), runevent)
	if err != nil {
		return "", err
	}

	return string(getobj), nil
}

// concatAllYamlFiles concat all yaml files from a directory as one big multi document yaml string
func (v *VCS) concatAllYamlFiles(ctx context.Context, objects []*github.RepositoryContent, runevent *info.Event) (string, error) {
	var allTemplates string

	for _, value := range objects {
		if strings.HasSuffix(value.GetName(), ".yaml") ||
			strings.HasSuffix(value.GetName(), ".yml") {
			data, err := v.getObject(ctx, value.GetSHA(), runevent)
			if err != nil {
				return "", err
			}
			if allTemplates != "" && !strings.HasPrefix(string(data), "---") {
				allTemplates += "---"
			}
			allTemplates += "\n" + string(data) + "\n"
		}
	}
	return allTemplates, nil
}

// getPullRequest get a pull request details
func (v *VCS) getPullRequest(ctx context.Context, runevent *info.Event, prNumber int) (*info.Event, error) {
	pr, _, err := v.Client.PullRequests.Get(ctx, runevent.Owner, runevent.Repository, prNumber)
	if err != nil {
		return runevent, err
	}
	// Make sure to use the Base for Default BaseBranch or there would be a potential hijack
	runevent.DefaultBranch = pr.GetBase().GetRepo().GetDefaultBranch()
	runevent.URL = pr.GetBase().GetRepo().GetHTMLURL()
	runevent.SHA = pr.GetHead().GetSHA()
	runevent.SHAURL = fmt.Sprintf("%s/commit/%s", pr.GetHTMLURL(), pr.GetHead().GetSHA())
	// TODO: Maybe if we wanted to allow rerequest from non approved user we
	// would use the CheckRun Sender instead of the rerequest sender, could it
	// be a room for abuse? 🤔
	runevent.Sender = pr.GetUser().GetLogin()
	runevent.HeadBranch = pr.GetHead().GetRef()
	runevent.BaseBranch = pr.GetBase().GetRef()
	runevent.EventType = "pull_request"
	return runevent, nil
}

// getObject Get an object from a repository
func (v *VCS) getObject(ctx context.Context, sha string, runevent *info.Event) ([]byte, error) {
	blob, _, err := v.Client.Git.GetBlob(ctx, runevent.Owner, runevent.Repository, sha)
	if err != nil {
		return nil, err
	}

	decoded, err := base64.StdEncoding.DecodeString(blob.GetContent())
	if err != nil {
		return nil, err
	}
	return decoded, err
}
