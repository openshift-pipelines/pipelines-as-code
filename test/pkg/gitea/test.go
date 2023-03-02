package gitea

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"testing"
	"time"

	"code.gitea.io/sdk/gitea"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/keys"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	pgitea "github.com/openshift-pipelines/pipelines-as-code/pkg/provider/gitea"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/options"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/payload"
	pacrepo "github.com/openshift-pipelines/pipelines-as-code/test/pkg/repository"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/names"
	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type TestOpts struct {
	SkipEventsCheck      bool
	NoCleanup            bool
	TargetNS             string
	TargetEvent          string
	Regexp               *regexp.Regexp
	YAMLFiles            map[string]string
	ExtraArgs            map[string]string
	RepoCRParams         *[]v1alpha1.Params
	CheckForStatus       string
	TargetRefName        string
	CheckForNumberStatus int
	ConcurrencyLimit     *int
	ParamsRun            *params.Run
	GiteaCNX             pgitea.Provider
	Opts                 options.E2E
	PullRequest          *gitea.PullRequest
	DefaultBranch        string
	GitCloneURL          string
	GitHTMLURL           string
	GiteaAPIURL          string
	GiteaPassword        string
	ExpectEvents         bool
	InternalGiteaURL     string
}

func PostCommentOnPullRequest(t *testing.T, topt *TestOpts, body string) {
	_, _, err := topt.GiteaCNX.Client.CreateIssueComment(topt.Opts.Organization,
		topt.Opts.Repo, topt.PullRequest.Index,
		gitea.CreateIssueCommentOption{Body: body})
	topt.ParamsRun.Clients.Log.Infof("Posted comment \"%s\" in %s", body, topt.PullRequest.HTMLURL)
	assert.NilError(t, err)
}

// TestPR will test the pull request event and grab comments from the PR
func TestPR(t *testing.T, topts *TestOpts) func() {
	ctx := context.Background()
	if topts.ParamsRun == nil {
		runcnx, opts, giteacnx, err := Setup(ctx)
		topts.GiteaCNX = giteacnx
		topts.ParamsRun = runcnx
		topts.Opts = opts
		assert.NilError(t, err, fmt.Errorf("cannot do gitea setup: %w", err))
	}
	hookURL := os.Getenv("TEST_GITEA_SMEEURL")
	topts.InternalGiteaURL = os.Getenv("TEST_GITEA_INTERNAL_URL")
	if topts.InternalGiteaURL == "" {
		topts.InternalGiteaURL = "http://gitea.gitea:3000"
	}
	if topts.ExtraArgs == nil {
		topts.ExtraArgs = map[string]string{}
	}
	topts.ExtraArgs["ProviderURL"] = topts.InternalGiteaURL
	if topts.TargetNS == "" {
		topts.TargetNS = topts.TargetRefName
	}
	if topts.TargetRefName == "" {
		topts.TargetRefName = names.SimpleNameGenerator.RestrictLengthWithRandomSuffix("pac-e2e-test")
		topts.TargetNS = topts.TargetRefName
		assert.NilError(t, pacrepo.CreateNS(ctx, topts.TargetNS, topts.ParamsRun))
	}

	repoInfo, err := CreateGiteaRepo(topts.GiteaCNX.Client, topts.Opts.Organization, topts.TargetRefName, hookURL)
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

	entries, err := payload.GetEntries(topts.YAMLFiles,
		topts.TargetNS,
		repoInfo.DefaultBranch,
		topts.TargetEvent,
		topts.ExtraArgs)
	assert.NilError(t, err)

	url, err := MakeGitCloneURL(repoInfo.CloneURL, os.Getenv("TEST_GITEA_USERNAME"), os.Getenv("TEST_GITEA_PASSWORD"))
	assert.NilError(t, err)
	topts.GitCloneURL = url
	PushFilesToRefGit(t, topts, entries, topts.DefaultBranch)
	pr, _, err := topts.GiteaCNX.Client.CreatePullRequest(topts.Opts.Organization, repoInfo.Name, gitea.CreatePullRequestOption{
		Title: "Test Pull Request - " + topts.TargetRefName,
		Head:  topts.TargetRefName,
		Base:  options.MainBranch,
	})
	assert.NilError(t, err)
	topts.PullRequest = pr
	topts.ParamsRun.Clients.Log.Infof("PullRequest %s has been created", pr.HTMLURL)
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

	events, err := topts.ParamsRun.Clients.Kube.CoreV1().Events(topts.TargetNS).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", keys.Repository, topts.TargetNS),
	})
	assert.NilError(t, err)
	if topts.ExpectEvents {
		// in some cases event is expected but it takes time
		// to emit and before that this check gets executed
		// so adds a sleep for that case eg. TestGiteaBadYaml
		if len(events.Items) == 0 {
			// loop 30 times over a 5 second period and try to get any events
			for i := 0; i < 30; i++ {
				events, err = topts.ParamsRun.Clients.Kube.CoreV1().Events(topts.TargetNS).List(ctx, metav1.ListOptions{
					LabelSelector: fmt.Sprintf("%s=%s", keys.Repository, topts.TargetNS),
				})
				assert.NilError(t, err)
				if len(events.Items) > 0 {
					break
				}
				time.Sleep(2 * time.Second)
			}
		}
		assert.Assert(t, len(events.Items) != 0, "events expected in case of failure but got 0")
	} else if !topts.SkipEventsCheck {
		assert.Assert(t, len(events.Items) == 0, fmt.Sprintf("no events expected but got %v in %v ns, items: %+v", len(events.Items), topts.TargetNS, events.Items))
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
			topts.ParamsRun.Clients.Log.Infof("Status is %s", cs.State)

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

func WaitForSecretDeletion(t *testing.T, topts *TestOpts, _ string) {
	i := 0
	for {
		// make sure pipelineRuns are deleted, before checking secrets
		list, err := topts.ParamsRun.Clients.Tekton.TektonV1().PipelineRuns(topts.TargetNS).
			List(context.Background(), metav1.ListOptions{
				LabelSelector: fmt.Sprintf("app.kubernetes.io/managed-by=%v", pipelinesascode.GroupName),
			})
		assert.NilError(t, err)

		if i > 5 {
			t.Fatalf("pipelineruns are not removed from the target namespace, something is fishy")
		}
		if len(list.Items) == 0 {
			break
		}
		topts.ParamsRun.Clients.Log.Infof("deleting pipelineRuns in %v namespace", topts.TargetNS)
		err = topts.ParamsRun.Clients.Tekton.TektonV1().PipelineRuns(topts.TargetNS).
			DeleteCollection(context.Background(), metav1.DeleteOptions{}, metav1.ListOptions{
				LabelSelector: fmt.Sprintf("app.kubernetes.io/managed-by=%v", pipelinesascode.GroupName),
			})
		assert.NilError(t, err)

		time.Sleep(5 * time.Second)
		i++
	}

	topts.ParamsRun.Clients.Log.Infof("checking secrets in %v namespace", topts.TargetNS)
	i = 0
	for {
		list, err := topts.ParamsRun.Clients.Kube.CoreV1().Secrets(topts.TargetNS).
			List(context.Background(), metav1.ListOptions{
				LabelSelector: fmt.Sprintf("pipelinesascode.tekton.dev/url-repository=%s\n", topts.TargetNS),
			})
		assert.NilError(t, err)

		if len(list.Items) == 0 {
			break
		}

		if i > 5 {
			t.Fatalf("secret has not removed from the target namespace, something is fishy")
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
				topts.ParamsRun.Clients.Log.Infof("Found regexp \"%s\" in PR comments", topts.Regexp.String())
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

func CheckIfPipelineRunsCancelled(t *testing.T, topts *TestOpts) {
	i := 0
	for {
		list, err := topts.ParamsRun.Clients.Tekton.TektonV1().PipelineRuns(topts.TargetNS).
			List(context.Background(), metav1.ListOptions{
				LabelSelector: fmt.Sprintf("%v=%v", keys.Repository, topts.TargetNS),
			})
		assert.NilError(t, err)

		if len(list.Items) == 0 {
			t.Fatalf("pipelineruns not found, where are they???")
		}

		if list.Items[0].Spec.Status == v1.PipelineRunSpecStatusCancelledRunFinally {
			topts.ParamsRun.Clients.Log.Info("PipelineRun is cancelled, yay!")
			break
		}

		if i > 5 {
			t.Fatalf("pipelineruns are not cancelled, something is fishy")
		}
		time.Sleep(5 * time.Second)
		i++
	}
}
