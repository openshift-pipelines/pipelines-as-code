package gitea

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"code.gitea.io/sdk/gitea"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	pgitea "github.com/openshift-pipelines/pipelines-as-code/pkg/provider/gitea"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/options"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/payload"
	"github.com/tektoncd/pipeline/pkg/names"
	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type TestOpts struct {
	NoCleanup            bool
	TargetNS             string
	TargetEvent          string
	Regexp               *regexp.Regexp
	YAMLFiles            map[string]string
	ExtraArgs            map[string]string
	CheckForStatus       string
	TargetRefName        string
	CheckForNumberStatus int
	ConcurrencyLimit     *int
	Clients              *params.Run
	GiteaCNX             pgitea.Provider
	Opts                 options.E2E
	PullRequest          *gitea.PullRequest
	DefaultBranch        string
	GitCloneURL          string
	GitHTMLURL           string
	GiteaAPIURL          string
	GiteaPassword        string
	ExpectEvents         bool
}

func PostCommentOnPullRequest(t *testing.T, topt *TestOpts, body string) {
	_, _, err := topt.GiteaCNX.Client.CreateIssueComment(topt.Opts.Organization,
		topt.Opts.Repo, topt.PullRequest.Index,
		gitea.CreateIssueCommentOption{Body: body})
	topt.Clients.Clients.Log.Infof("Posted comment \"%s\" in %s", body, topt.PullRequest.HTMLURL)
	assert.NilError(t, err)
}

// TestPR will test the pull request event and grab comments from the PR
func TestPR(t *testing.T, topts *TestOpts) func() {
	ctx := context.Background()
	runcnx, opts, giteacnx, err := Setup(ctx)
	topts.GiteaCNX = giteacnx
	topts.Clients = runcnx
	topts.Opts = opts
	assert.NilError(t, err, fmt.Errorf("cannot do gitea setup: %w", err))
	hookURL := os.Getenv("TEST_GITEA_SMEEURL")
	if topts.ExtraArgs == nil {
		topts.ExtraArgs = map[string]string{}
	}
	if topts.TargetRefName == "" {
		topts.TargetRefName = names.SimpleNameGenerator.RestrictLengthWithRandomSuffix("pac-e2e-test")
	}
	if topts.TargetNS == "" {
		topts.TargetNS = topts.TargetRefName
	}
	repoInfo, err := CreateGiteaRepo(giteacnx.Client, opts.Organization, topts.TargetRefName, hookURL)
	assert.NilError(t, err)
	topts.Opts.Repo = repoInfo.Name
	topts.Opts.Organization = repoInfo.Owner.UserName
	topts.DefaultBranch = repoInfo.DefaultBranch
	topts.GitHTMLURL = repoInfo.HTMLURL

	cleanup := func() {
		if os.Getenv("TEST_NOCLEANUP") != "true" {
			defer TearDown(ctx, t, topts)
		}
	}
	err = CreateCRD(ctx, topts)
	assert.NilError(t, err)

	entries, err := payload.GetEntries(topts.YAMLFiles, topts.TargetNS, repoInfo.DefaultBranch, topts.TargetEvent, topts.ExtraArgs)
	assert.NilError(t, err)

	url, err := MakeGitCloneURL(repoInfo.CloneURL, os.Getenv("TEST_GITEA_USERNAME"), os.Getenv("TEST_GITEA_PASSWORD"))
	assert.NilError(t, err)
	topts.GitCloneURL = url
	PushFilesToRefGit(t, topts, entries, topts.DefaultBranch)
	pr, _, err := giteacnx.Client.CreatePullRequest(opts.Organization, repoInfo.Name, gitea.CreatePullRequestOption{
		Title: "Test Pull Request - " + topts.TargetRefName,
		Head:  topts.TargetRefName,
		Base:  options.MainBranch,
	})
	assert.NilError(t, err)
	topts.PullRequest = pr
	runcnx.Clients.Log.Infof("PullRequest %s has been created", pr.HTMLURL)
	giteaURL := os.Getenv("TEST_GITEA_API_URL")
	giteaPassword := os.Getenv("TEST_GITEA_PASSWORD")
	topts.GiteaAPIURL = giteaURL
	topts.GiteaPassword = giteaPassword

	if topts.CheckForStatus != "" {
		WaitForStatus(t, topts, topts.TargetRefName)
	}

	if topts.Regexp != nil {
		WaitForPullRequestCommentMatch(ctx, t, topts)
	}

	events, err := topts.Clients.Clients.Kube.CoreV1().Events(topts.TargetNS).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", filepath.Join(pipelinesascode.GroupName, "repository"), topts.TargetNS),
	})
	assert.NilError(t, err)
	if topts.ExpectEvents {
		// in some cases event is expected but it takes time
		// to emit and before that this check gets executed
		// so adds a sleep for that case eg. TestGiteaBadYaml
		if len(events.Items) == 0 {
			time.Sleep(time.Second * 5)
			events, err = topts.Clients.Clients.Kube.CoreV1().Events(topts.TargetNS).List(ctx, metav1.ListOptions{
				LabelSelector: fmt.Sprintf("%s=%s", filepath.Join(pipelinesascode.GroupName, "repository"), topts.TargetNS),
			})
			assert.NilError(t, err)
		}
		assert.Assert(t, len(events.Items) != 0, "events expected in case of failure but got 0")
	} else {
		assert.Assert(t, len(events.Items) == 0, fmt.Sprintf("no events expected but got %v in %v ns", len(events.Items), topts.TargetNS))
	}
	return cleanup
}

func WaitForStatus(t *testing.T, topts *TestOpts, ref string) {
	i := 0
	for {
		cs, _, err := topts.GiteaCNX.Client.GetCombinedStatus(topts.Opts.Organization, topts.Opts.Repo, ref)
		assert.NilError(t, err)
		if cs.State != "" && cs.State != "pending" {
			assert.Equal(t, string(cs.State), topts.CheckForStatus)
			topts.Clients.Clients.Log.Infof("Status is %s", cs.State)

			if topts.CheckForNumberStatus != 0 {
				assert.Equal(t, len(cs.Statuses), topts.CheckForNumberStatus)
			}
			break
		}
		if i > 50 {
			t.Fatalf("gitea status has not been updated")
		}
		time.Sleep(5 * time.Second)
		i++
	}
}

func WaitForPullRequestCommentMatch(ctx context.Context, t *testing.T, topts *TestOpts) {
	i := 0
	for {
		tls, err := GetIssueTimeline(ctx, topts)
		assert.NilError(t, err)
		for _, v := range tls {
			// look for the regexp "Pipelines as Code CI.*has.*successfully" in v.body
			if topts.Regexp.MatchString(v.Body) {
				topts.Clients.Clients.Log.Infof("Found regexp \"%s\" in PR comments", topts.Regexp.String())
				return
			}
		}
		if i > 60 {
			t.Fatalf("gitea driver has not been posted any comment")
		}
		time.Sleep(5 * time.Second)
		i++
	}
}
