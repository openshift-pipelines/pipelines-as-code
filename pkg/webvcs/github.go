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
	CheckRunID    *int64
}

func (r RunInfo) Check() error {
	if r.SHA != "" && r.Branch != "" &&
		r.Repository != "" && r.DefaultBranch != "" &&
		r.Owner != "" && r.URL != "" {
		return nil
	}
	return fmt.Errorf("Missing values in runInfo")
}

// DeepCopyInto deep copy runinfo in another instance
func (r *RunInfo) DeepCopyInto(out *RunInfo) {
	*out = *r
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

// We got a bunch of \r\n or \n and others from triggers/github, so let just
// workaround it. Originally from https://stackoverflow.com/a/52600147
func payloadFix(payload string) string {
	var replacement = " "
	var replacer = strings.NewReplacer(
		"\r\n", replacement,
		"\r", replacement,
		"\n", replacement,
		"\v", replacement,
		"\f", replacement,
		"\u0085", replacement,
		"\u2028", replacement,
		"\u2029", replacement,
	)
	return replacer.Replace(payload)
}

func (v GithubVCS) ParsePayload(payload string) (*RunInfo, error) {
	payload = payloadFix(payload)
	prMap := &github.PullRequestEvent{}
	err := json.Unmarshal([]byte(payload), prMap)
	if err != nil {
		return &RunInfo{}, err
	}

	if prMap.PullRequest == nil {
		return &RunInfo{}, errors.New("Cannot parse payload as PR")
	}

	return &RunInfo{
		Owner:         prMap.GetRepo().Owner.GetLogin(),
		Repository:    prMap.GetRepo().GetName(),
		URL:           prMap.GetRepo().GetHTMLURL(),
		DefaultBranch: prMap.GetRepo().GetDefaultBranch(),
		// TODO: this is going to be different on merge
		SHA:    prMap.PullRequest.Head.GetSHA(),
		Branch: prMap.PullRequest.Base.GetRef(),
	}, nil
}

func (v GithubVCS) GetTektonDir(path string, runinfo *RunInfo) ([]*github.RepositoryContent, error) {
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

// GetFileInsideRepo Get a file via Github API using the runinfo information, we
// branch is true, the user the branch as ref isntead of the SHA
func (v GithubVCS) GetFileInsideRepo(path string, branch bool, runinfo *RunInfo) (string, error) {
	ref := runinfo.SHA
	if branch {
		ref = runinfo.Branch
	}

	fp, objects, resp, err := v.Client.Repositories.GetContents(v.Context, runinfo.Owner,
		runinfo.Repository, path, &github.RepositoryContentGetOptions{Ref: ref})

	if err != nil {
		return "", err
	}
	if objects != nil {
		return "", fmt.Errorf("Referenced file inside the Github Repository %s is a directory", path)
	}
	if resp.Response.StatusCode == http.StatusNotFound {
		return "", fmt.Errorf("Cannot find %s in this repository", path)
	}

	getobj, err := v.GetObject(fp.GetSHA(), runinfo)
	if err != nil {
		return "", err
	}

	return string(getobj), nil
}

// GetFileFromDefaultBranch will get a file directly from the Default Branch as
// configured in runinfo which is directly set in webhook by Github
func (v GithubVCS) GetFileFromDefaultBranch(path string, runinfo *RunInfo) (string, error) {
	var runInfoOnMain = &RunInfo{}
	runinfo.DeepCopyInto(runInfoOnMain)
	runInfoOnMain.Branch = runInfoOnMain.DefaultBranch

	tektonyaml, err := v.GetFileInsideRepo(path, true, runInfoOnMain)
	if err != nil {
		return "", fmt.Errorf("Cannot find %s inside the \"%s\" branch: %s", path, runInfoOnMain.Branch, err)
	}
	return tektonyaml, err
}

func (v GithubVCS) GetTektonDirTemplate(objects []*github.RepositoryContent, runinfo *RunInfo) (string, error) {
	var allTemplates string

	for _, value := range objects {
		if value.GetName() != "tekton.yaml" && (strings.HasSuffix(value.GetName(), ".yaml") ||
			strings.HasSuffix(value.GetName(), ".yml")) {
			data, err := v.GetObject(value.GetSHA(), runinfo)
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

func (v GithubVCS) GetObject(sha string, runinfo *RunInfo) ([]byte, error) {
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

func (v GithubVCS) CreateCheckRun(status string, runinfo *RunInfo) (*github.CheckRun, error) {
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

func (v GithubVCS) CreateStatus(runinfo *RunInfo, status, conclusion, text, detailURL string) (*github.CheckRun, error) {
	now := github.Timestamp{Time: time.Now()}

	var summary, title string

	switch conclusion {
	case "success":
		title = "🥳 CI Run Report: Success"
		summary = "Tekton CI has successfully validated your commit."
	case "failure":
		title = "😞 CI Run: Failed"
		summary = "Tekon CI has <b>failed</b>."
	case "skipped":
		title = "🤫 CI Run: Skipped"
		summary = "Tekton CI is skipping this commit."
	case "neutral":
		title = "🤨 CI Run: Unknown"
		summary = "Tekton CI doesn't know what happened with this commit."
	}

	checkRunOutput := &github.CheckRunOutput{
		Title:   &title,
		Summary: &summary,
		Text:    &text,
	}

	opts := github.UpdateCheckRunOptions{
		Name:        "Tekton Pipeline as Code CI",
		Status:      &status,
		Conclusion:  &conclusion,
		CompletedAt: &now,
		Output:      checkRunOutput,
	}

	if detailURL != "" {
		opts.DetailsURL = &detailURL
	}

	checkRun, _, err := v.Client.Checks.UpdateCheckRun(v.Context, runinfo.Owner, runinfo.Repository, *runinfo.CheckRunID, opts)
	return checkRun, err
}
