package webvcs

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/go-github/v34/github"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli"
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

func (v GithubVCS) GetTektonDirTemplate(cs *cli.Clients, objects []*github.RepositoryContent, runinfo RunInfo) (string, error) {
	var all_objects string
	var all_templates string

	for _, value := range objects {
		if all_objects != "" {
			all_objects += ", "
		}
		all_objects += value.GetName()
		if value.GetName() != "tekton.yaml" && (strings.HasSuffix(value.GetName(), ".yaml") ||
			strings.HasSuffix(value.GetName(), ".yml")) {
			data, err := v.GetObject(value.GetSHA(), runinfo)
			if err != nil {
				return "", err
			}
			if all_templates != "" && !strings.HasPrefix(string(data), "---") {
				all_templates += "---"
			}
			all_templates += "\n" + string(data)
		}
	}
	cs.Log.Infof("Templates in .tekton directory: %s", all_objects)
	return all_templates, nil
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

func (v GithubVCS) CreateCheckRun(status string, runinfo RunInfo) (*github.CheckRun, error) {
	now := github.Timestamp{Time: time.Now()}
	checkrunoption := github.CreateCheckRunOptions{
		Name:    "Tekton Pipeline as Code CI",
		HeadSHA: runinfo.SHA,
		Status:  &status,
		// DetailsURL: "http://todo", // TODO: OpenShift or Tekton Dashboard Target
		StartedAt: &now,
	}

	checkRun, _, err := v.Client.Checks.CreateCheckRun(v.Context, runinfo.Owner, runinfo.Repository, checkrunoption)
	return checkRun, err
}

func (v GithubVCS) CreateStatus(runinfo RunInfo, checkrunid int64, status, conclusion, text, detailURL string) (*github.CheckRun, error) {
	now := github.Timestamp{Time: time.Now()}

	var summary, title string

	if status == "success" {
		title = "CI Run Report"
		summary = "✅ CI has succeeded"
	} else if status == "neutral" {
		title = "CI Run: Skipped"
		summary = "🤷 Skipping this check"
	}

	checkRunOutput := github.CheckRunOutput{
		Title:   &title,
		Summary: &summary,
		Text:    &text,
	}

	opts := github.UpdateCheckRunOptions{
		Name:        "Tekton Pipeline as Code CI",
		Status:      &status,
		Conclusion:  &conclusion,
		CompletedAt: &now,
		Output:      &checkRunOutput,
	}

	if detailURL != "" {
		opts.DetailsURL = &detailURL
	}

	checkRun, _, err := v.Client.Checks.UpdateCheckRun(v.Context, runinfo.Owner, runinfo.Repository, checkrunid, opts)
	return checkRun, err
}
