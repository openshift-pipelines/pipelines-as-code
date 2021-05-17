package webvcs

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/google/go-github/v34/github"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
)

type GithubVCS struct {
	Client *github.Client
}

// RunInfo Information about current run
type RunInfo struct {
	BaseBranch    string // branch against where we are making the PR
	HeadBranch    string // branch from where our SHA get tested
	DefaultBranch string
	CheckRunID    *int64
	EventType     string
	Owner         string
	Repository    string
	Sender        string
	SHA           string
	URL           string
	WebConsoleURL string
}

// Check check if the runinfo is properly set
func (r RunInfo) Check() error {
	if r.SHA != "" && r.BaseBranch != "" &&
		r.Repository != "" && r.DefaultBranch != "" &&
		r.HeadBranch != "" && r.Owner != "" && r.URL != "" &&
		r.Sender != "" && r.EventType != "" {
		return nil
	}
	return fmt.Errorf("missing values in runInfo")
}

// DeepCopyInto deep copy runinfo in another instance
func (r *RunInfo) DeepCopyInto(out *RunInfo) {
	*out = *r
}

// NewGithubVCS Create a new GitHub VCS object for token
func NewGithubVCS(token string) GithubVCS {
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(context.Background(), ts)
	return GithubVCS{
		Client: github.NewClient(tc),
	}
}

// payloadFix since we are getting a bunch of \r\n or \n and others from triggers/github, so let just
// workaround it. Originally from https://stackoverflow.com/a/52600147
func payloadFix(payload string) string {
	replacement := " "
	replacer := strings.NewReplacer(
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

func (v GithubVCS) handleReRequestEvent(ctx context.Context, log *zap.SugaredLogger, event *github.CheckRunEvent) (RunInfo, error) {
	runinfo := RunInfo{
		Owner:      event.GetRepo().GetOwner().GetLogin(),
		Repository: event.GetRepo().GetName(),
	}
	prNumber := event.GetCheckRun().GetCheckSuite().PullRequests[0].GetNumber()
	log.Infof("Recheck of PR %s/%s#%d has been requested", runinfo.Owner, runinfo.Repository, prNumber)
	return v.getPullRequest(ctx, runinfo, prNumber)
}

func (v GithubVCS) handleIssueCommentEvent(ctx context.Context, log *zap.SugaredLogger, event *github.IssueCommentEvent) (RunInfo, error) {
	runinfo := RunInfo{
		Owner:      event.GetRepo().GetOwner().GetLogin(),
		Repository: event.GetRepo().GetName(),
	}
	if !event.GetIssue().IsPullRequest() {
		return RunInfo{}, fmt.Errorf("issue comment is not coming from a pull_request")
	}

	// We are getting the full URL so we have to get the last part to get the PR number,
	// we don't have to care about URL query string/hash and other stuff because
	// that comes up from the API.
	prNumber, err := strconv.Atoi(path.Base(event.GetIssue().GetPullRequestLinks().GetHTMLURL()))
	if err != nil {
		return RunInfo{}, err
	}

	log.Infof("PR recheck from issue commment on %s/%s#%d has been requested", runinfo.Owner, runinfo.Repository, prNumber)
	return v.getPullRequest(ctx, runinfo, prNumber)
}

// getPullRequest get a pull request details
func (v GithubVCS) getPullRequest(ctx context.Context, runinfo RunInfo, prNumber int) (RunInfo, error) {
	pr, _, err := v.Client.PullRequests.Get(ctx, runinfo.Owner, runinfo.Repository, prNumber)
	if err != nil {
		return runinfo, err
	}
	// Make sure to use the Base for Default BaseBranch or there would be a potential hijack
	runinfo.DefaultBranch = pr.GetBase().GetRepo().GetDefaultBranch()
	runinfo.URL = pr.GetBase().GetRepo().GetHTMLURL()
	runinfo.SHA = pr.GetHead().GetSHA()
	// TODO: Maybe if we wanted to allow rerequest from non approved user we
	// would use the CheckRun Sender instead of the rerequest sender, could it
	// be a room for abuse? 🤔
	runinfo.Sender = pr.GetUser().GetLogin()
	runinfo.HeadBranch = pr.GetHead().GetRef()
	runinfo.BaseBranch = pr.GetBase().GetRef()
	return runinfo, nil
}

// ParsePayload parse payload event
func (v GithubVCS) ParsePayload(ctx context.Context, log *zap.SugaredLogger, eventType, payload string) (*RunInfo, error) {
	var runinfo RunInfo
	payload = payloadFix(payload)
	event, err := github.ParseWebHook(eventType, []byte(payloadFix(payload)))
	if err != nil {
		return &runinfo, err
	}
	err = json.Unmarshal([]byte(payload), &event)
	if err != nil {
		return &runinfo, err
	}

	switch event := event.(type) {
	case *github.CheckRunEvent:
		if event.GetAction() == "rerequested" {
			runinfo, err = v.handleReRequestEvent(ctx, log, event)
			if err != nil {
				return &runinfo, err
			}

		}
	case *github.IssueCommentEvent:
		runinfo, err = v.handleIssueCommentEvent(ctx, log, event)
		if err != nil {
			return &runinfo, err
		}
	case *github.PullRequestEvent:
		runinfo = RunInfo{
			Owner:         event.GetRepo().Owner.GetLogin(),
			Repository:    event.GetRepo().GetName(),
			DefaultBranch: event.GetRepo().GetDefaultBranch(),
			SHA:           event.GetPullRequest().Head.GetSHA(),
			URL:           event.GetRepo().GetHTMLURL(),
			BaseBranch:    event.GetPullRequest().Base.GetRef(),
			HeadBranch:    event.GetPullRequest().Head.GetRef(),
			Sender:        event.GetPullRequest().GetUser().GetLogin(),
		}
	default:
		return &runinfo, errors.New("this event is not supported")
	}
	runinfo.EventType = eventType
	return &runinfo, nil
}

// CheckSenderOrgMembership Get sender user's organization. We can
// only get the one that the user sets as public 🤷
func (v GithubVCS) CheckSenderOrgMembership(ctx context.Context, runinfo *RunInfo) (bool, error) {
	users, resp, err := v.Client.Organizations.ListMembers(ctx, runinfo.Owner,
		&github.ListMembersOptions{
			PublicOnly: true, // We can't list private member in a org
		})

	// If we are 404 it means we are checking a repo owner and not a org so let's bail out with grace
	if resp != nil && resp.Response.StatusCode == http.StatusNotFound {
		return false, nil
	}

	if err != nil {
		return false, err
	}
	for _, v := range users {
		if v.GetLogin() == runinfo.Sender {
			return true, nil
		}
	}

	return false, nil
}

// GetTektonDir Get tekton directory from a repository
func (v GithubVCS) GetTektonDir(ctx context.Context, path string, runinfo *RunInfo) ([]*github.RepositoryContent, error) {
	fp, objects, resp, err := v.Client.Repositories.GetContents(ctx, runinfo.Owner,
		runinfo.Repository, path, &github.RepositoryContentGetOptions{Ref: runinfo.SHA})

	if fp != nil {
		return nil, fmt.Errorf("the object %s is a file instead of a directory", path)
	}
	if resp != nil && resp.Response.StatusCode == http.StatusNotFound {
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	return objects, nil
}

// GetFileInsideRepo Get a file via Github API using the runinfo information, we
// branch is true, the user the branch as ref isntead of the SHA
func (v GithubVCS) GetFileInsideRepo(ctx context.Context, path string, branch bool, runinfo *RunInfo) (string, error) {
	ref := runinfo.SHA
	if branch {
		ref = runinfo.BaseBranch
	}

	fp, objects, resp, err := v.Client.Repositories.GetContents(ctx, runinfo.Owner,
		runinfo.Repository, path, &github.RepositoryContentGetOptions{Ref: ref})
	if err != nil {
		return "", err
	}
	if objects != nil {
		return "", fmt.Errorf("referenced file inside the Github Repository %s is a directory", path)
	}
	if resp.Response.StatusCode == http.StatusNotFound {
		return "", fmt.Errorf("cannot find %s in this repository", path)
	}

	getobj, err := v.GetObject(ctx, fp.GetSHA(), runinfo)
	if err != nil {
		return "", err
	}

	return string(getobj), nil
}

// GetFileFromDefaultBranch will get a file directly from the Default BaseBranch as
// configured in runinfo which is directly set in webhook by Github
func (v GithubVCS) GetFileFromDefaultBranch(ctx context.Context, path string, runinfo *RunInfo) (string, error) {
	runInfoOnMain := &RunInfo{}
	runinfo.DeepCopyInto(runInfoOnMain)
	runInfoOnMain.BaseBranch = runInfoOnMain.DefaultBranch

	tektonyaml, err := v.GetFileInsideRepo(ctx, path, true, runInfoOnMain)
	if err != nil {
		return "", fmt.Errorf("cannot find %s inside the %s branch: %s", path, runInfoOnMain.BaseBranch, err)
	}
	return tektonyaml, err
}

// ConcatAllYamlFiles concat all yaml files from a directory as one big multi document yaml string
// TODO: trash the tekton.yaml condition when we don't need it anymore
func (v GithubVCS) ConcatAllYamlFiles(ctx context.Context, objects []*github.RepositoryContent, runinfo *RunInfo) (string, error) {
	var allTemplates string

	for _, value := range objects {
		if value.GetName() != "tekton.yaml" && (strings.HasSuffix(value.GetName(), ".yaml") ||
			strings.HasSuffix(value.GetName(), ".yml")) {
			data, err := v.GetObject(ctx, value.GetSHA(), runinfo)
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

// GetObject Get an object from a repository
func (v GithubVCS) GetObject(ctx context.Context, sha string, runinfo *RunInfo) ([]byte, error) {
	blob, _, err := v.Client.Git.GetBlob(ctx, runinfo.Owner, runinfo.Repository, sha)
	if err != nil {
		return nil, err
	}

	decoded, err := base64.StdEncoding.DecodeString(blob.GetContent())
	if err != nil {
		return nil, err
	}
	return decoded, err
}

func (v GithubVCS) CreateCheckRun(ctx context.Context, status string, runinfo *RunInfo) (*github.CheckRun, error) {
	now := github.Timestamp{Time: time.Now()}
	checkrunoption := github.CreateCheckRunOptions{
		Name:       "Tekton Pipeline as Code CI",
		HeadSHA:    runinfo.SHA,
		Status:     &status,
		DetailsURL: &runinfo.WebConsoleURL,
		StartedAt:  &now,
	}

	checkRun, _, err := v.Client.Checks.CreateCheckRun(ctx, runinfo.Owner, runinfo.Repository, checkrunoption)
	return checkRun, err
}

func (v GithubVCS) CreateStatus(ctx context.Context, runinfo *RunInfo, status, conclusion, text, detailsURL string) (*github.CheckRun, error) {
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
		Name:   "Tekton Pipeline as Code CI",
		Status: &status,
		Output: checkRunOutput,
	}

	if detailsURL != "" {
		opts.DetailsURL = &detailsURL
	}

	// Only set completed-at if conclusion is set (which means finished)
	if conclusion != "" {
		opts.CompletedAt = &now
		opts.Conclusion = &conclusion
	}

	checkRun, _, err := v.Client.Checks.UpdateCheckRun(ctx, runinfo.Owner, runinfo.Repository, *runinfo.CheckRunID, opts)
	return checkRun, err
}
