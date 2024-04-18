package gitea

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"code.gitea.io/sdk/gitea"
	"github.com/google/go-github/v61/github"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/keys"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/formatting"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	pgitea "github.com/openshift-pipelines/pipelines-as-code/pkg/provider/gitea"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/cctx"
	tlogs "github.com/openshift-pipelines/pipelines-as-code/test/pkg/logs"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/options"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/payload"
	pacrepo "github.com/openshift-pipelines/pipelines-as-code/test/pkg/repository"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/scm"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/names"
	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type TestOpts struct {
	StatusOnlyLatest      bool
	OnOrg                 bool
	NoPullRequestCreation bool
	SkipEventsCheck       bool
	TargetNS              string
	TargetEvent           string
	Settings              *v1alpha1.Settings
	Regexp                *regexp.Regexp
	YAMLFiles             map[string]string
	ExtraArgs             map[string]string
	RepoCRParams          *[]v1alpha1.Params
	GlobalRepoCRParams    *[]v1alpha1.Params
	CheckForStatus        string
	TargetRefName         string
	CheckForNumberStatus  int
	ConcurrencyLimit      *int
	ParamsRun             *params.Run
	GiteaCNX              pgitea.Provider
	Opts                  options.E2E
	PullRequest           *gitea.PullRequest
	DefaultBranch         string
	GitCloneURL           string
	GitHTMLURL            string
	GiteaAPIURL           string
	GiteaPassword         string
	ExpectEvents          bool
	InternalGiteaURL      string
	Token                 string
	FileChanges           []scm.FileChange
}

func PostCommentOnPullRequest(t *testing.T, topt *TestOpts, body string) {
	_, _, err := topt.GiteaCNX.Client.CreateIssueComment(topt.Opts.Organization,
		topt.Opts.Repo, topt.PullRequest.Index,
		gitea.CreateIssueCommentOption{Body: body})
	topt.ParamsRun.Clients.Log.Infof("Posted comment \"%s\" in %s", body, topt.PullRequest.HTMLURL)
	assert.NilError(t, err)
}

// TestPR will test the pull request event and grab comments from the PR.
func TestPR(t *testing.T, topts *TestOpts) (context.Context, func()) {
	ctx := context.Background()
	if topts.ParamsRun == nil {
		runcnx, opts, giteacnx, err := Setup(ctx)
		assert.NilError(t, err, fmt.Errorf("cannot do gitea setup: %w", err))
		topts.GiteaCNX = giteacnx
		topts.ParamsRun = runcnx
		topts.Opts = opts
	}
	ctx, err := cctx.GetControllerCtxInfo(ctx, topts.ParamsRun)
	assert.NilError(t, err)
	giteaURL := os.Getenv("TEST_GITEA_API_URL")
	giteaPassword := os.Getenv("TEST_GITEA_PASSWORD")
	topts.GiteaAPIURL = giteaURL
	topts.GiteaPassword = giteaPassword
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

	repoInfo, err := CreateGiteaRepo(topts.GiteaCNX.Client, topts.Opts.Organization, topts.TargetRefName, hookURL, topts.OnOrg, topts.ParamsRun.Clients.Log)
	assert.NilError(t, err)
	topts.Opts.Repo = repoInfo.Name
	topts.Opts.Organization = repoInfo.Owner.UserName
	topts.DefaultBranch = repoInfo.DefaultBranch
	topts.GitHTMLURL = repoInfo.HTMLURL

	topts.Token, err = CreateToken(topts)
	assert.NilError(t, err)

	gp := &v1alpha1.GitProvider{
		Type: "gitea",
		// caveat this assume gitea running on the same cluster, which
		// we do and need for e2e tests but that may be changed somehow
		URL:    topts.InternalGiteaURL,
		Secret: &v1alpha1.Secret{Name: topts.TargetNS, Key: "token"},
	}
	spec := v1alpha1.RepositorySpec{
		URL:              topts.GitHTMLURL,
		ConcurrencyLimit: topts.ConcurrencyLimit,
		Params:           topts.RepoCRParams,
		Settings:         topts.Settings,
	}
	if topts.GlobalRepoCRParams == nil {
		spec.GitProvider = gp
	} else {
		spec.GitProvider = &v1alpha1.GitProvider{Type: "gitea"}
	}
	assert.NilError(t, CreateCRD(ctx, topts, spec, false))

	// we only test params for global repo settings for now we may change that if we want
	if topts.GlobalRepoCRParams != nil {
		spec := v1alpha1.RepositorySpec{
			Params:      topts.GlobalRepoCRParams,
			GitProvider: gp,
		}
		assert.NilError(t, CreateCRD(ctx, topts, spec, true))
	}

	cleanup := func() {
		if os.Getenv("TEST_NOCLEANUP") != "true" {
			defer TearDown(ctx, t, topts)
		}
	}

	url, err := scm.MakeGitCloneURL(repoInfo.CloneURL, os.Getenv("TEST_GITEA_USERNAME"), os.Getenv("TEST_GITEA_PASSWORD"))
	assert.NilError(t, err)
	topts.GitCloneURL = url

	if topts.NoPullRequestCreation {
		return ctx, cleanup
	}

	entries, err := payload.GetEntries(topts.YAMLFiles,
		topts.TargetNS,
		repoInfo.DefaultBranch,
		topts.TargetEvent,
		topts.ExtraArgs)
	assert.NilError(t, err)

	scmOpts := &scm.Opts{
		GitURL:        topts.GitCloneURL,
		Log:           topts.ParamsRun.Clients.Log,
		WebURL:        topts.GitHTMLURL,
		TargetRefName: topts.TargetRefName,
		BaseRefName:   topts.DefaultBranch,
	}
	scm.PushFilesToRefGit(t, scmOpts, entries)

	topts.ParamsRun.Clients.Log.Infof("Creating PullRequest")
	for i := 0; i < 5; i++ {
		if topts.PullRequest, _, err = topts.GiteaCNX.Client.CreatePullRequest(topts.Opts.Organization, repoInfo.Name, gitea.CreatePullRequestOption{
			Title: "Test Pull Request - " + topts.TargetRefName,
			Head:  topts.TargetRefName,
			Base:  options.MainBranch,
		}); err == nil {
			break
		}
		topts.ParamsRun.Clients.Log.Infof("Creating PullRequest has failed, retrying %d/%d, err", i, 5, err)
		if i == 4 {
			t.Fatalf("cannot create pull request: %v", err)
		}
		time.Sleep(5 * time.Second)
	}
	topts.ParamsRun.Clients.Log.Infof("PullRequest %s has been created", topts.PullRequest.HTMLURL)

	if topts.CheckForStatus != "" {
		WaitForStatus(t, topts, topts.TargetRefName, "", topts.StatusOnlyLatest)
	}

	if topts.Regexp != nil {
		WaitForPullRequestCommentMatch(t, topts)
	}

	events, err := topts.ParamsRun.Clients.Kube.CoreV1().Events(topts.TargetNS).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", keys.Repository, formatting.CleanValueKubernetes(topts.TargetNS)),
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
					LabelSelector: fmt.Sprintf("%s=%s", keys.Repository, formatting.CleanValueKubernetes(topts.TargetNS)),
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
	return ctx, cleanup
}

func NewPR(t *testing.T, topts *TestOpts) func() {
	ctx := context.Background()
	if topts.ParamsRun == nil {
		runcnx, opts, giteacnx, err := Setup(ctx)
		assert.NilError(t, err, fmt.Errorf("cannot do gitea setup: %w", err))
		topts.GiteaCNX = giteacnx
		topts.ParamsRun = runcnx
		topts.Opts = opts
	}
	giteaURL := os.Getenv("TEST_GITEA_API_URL")
	giteaPassword := os.Getenv("TEST_GITEA_PASSWORD")
	topts.GiteaAPIURL = giteaURL
	topts.GiteaPassword = giteaPassword
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

	repoInfo, err := GetGiteaRepo(topts.GiteaCNX.Client, topts.Opts.Organization, topts.TargetRefName, topts.ParamsRun.Clients.Log)
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
	// topts.Token, err = CreateToken(topts)
	// assert.NilError(t, err)

	// assert.NilError(t, CreateCRD(ctx, topts))

	url, err := scm.MakeGitCloneURL(repoInfo.CloneURL, os.Getenv("TEST_GITEA_USERNAME"), os.Getenv("TEST_GITEA_PASSWORD"))
	assert.NilError(t, err)
	topts.GitCloneURL = url

	if topts.NoPullRequestCreation {
		return cleanup
	}

	scmOpts := &scm.Opts{
		GitURL:        topts.GitCloneURL,
		Log:           topts.ParamsRun.Clients.Log,
		WebURL:        topts.GitHTMLURL,
		TargetRefName: topts.TargetRefName,
		BaseRefName:   topts.DefaultBranch,
	}
	scm.ChangeFilesRefGit(t, scmOpts, topts.FileChanges)

	topts.ParamsRun.Clients.Log.Infof("Creating PullRequest")
	for i := 0; i < 5; i++ {
		if topts.PullRequest, _, err = topts.GiteaCNX.Client.CreatePullRequest(topts.Opts.Organization, repoInfo.Name, gitea.CreatePullRequestOption{
			Title: "Test Pull Request - " + topts.TargetRefName,
			Head:  topts.TargetRefName,
			Base:  options.MainBranch,
		}); err == nil {
			break
		}
		topts.ParamsRun.Clients.Log.Infof("Creating PullRequest has failed, retrying %d/%d, err", i, 5, err)
		if i == 4 {
			t.Fatalf("cannot create pull request: %v", err)
		}
		time.Sleep(5 * time.Second)
	}
	topts.ParamsRun.Clients.Log.Infof("PullRequest %s has been created", topts.PullRequest.HTMLURL)

	if topts.CheckForStatus != "" {
		WaitForStatus(t, topts, topts.TargetRefName, "", topts.StatusOnlyLatest)
	}

	if topts.Regexp != nil {
		WaitForPullRequestCommentMatch(t, topts)
	}

	events, err := topts.ParamsRun.Clients.Kube.CoreV1().Events(topts.TargetNS).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", keys.Repository, formatting.CleanValueKubernetes(topts.TargetNS)),
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
					LabelSelector: fmt.Sprintf("%s=%s", keys.Repository, formatting.CleanValueKubernetes(topts.TargetNS)),
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

func WaitForStatus(t *testing.T, topts *TestOpts, ref, forcontext string, onlylatest bool) {
	i := 0
	if strings.HasPrefix(ref, "heads/") {
		refo, _, err := topts.GiteaCNX.Client.GetRepoRefs(topts.Opts.Organization, topts.Opts.Repo, ref)
		assert.NilError(t, err)
		ref = refo[0].Object.SHA
	}
	checkNumberOfStatus := topts.CheckForNumberStatus
	if checkNumberOfStatus == 0 {
		checkNumberOfStatus = 1
	}
	for {
		numstatus := 0
		// get first sha of tree ref
		statuses, _, err := topts.GiteaCNX.Client.ListStatuses(topts.Opts.Organization, topts.Opts.Repo, ref, gitea.ListStatusesOption{})
		assert.NilError(t, err)
		// sort statuses by id
		sort.Slice(statuses, func(i, j int) bool {
			return statuses[i].ID < statuses[j].ID
		})
		if onlylatest {
			if len(statuses) > 1 {
				statuses = statuses[len(statuses)-1:]
			} else {
				time.Sleep(5 * time.Second)
				continue
			}
		}
		for _, cstatus := range statuses {
			if topts.CheckForStatus == "Skipped" {
				if strings.HasSuffix(cstatus.Description, "Pending approval") {
					numstatus++
					break
				}
			}
			if cstatus.State == "pending" {
				continue
			}
			if forcontext != "" && cstatus.Context != forcontext {
				continue
			}
			statuscheck := topts.CheckForStatus
			if statuscheck != "" && statuscheck != string(cstatus.State) {
				if statuscheck != cstatus.Description {
					t.Fatalf("Status on SHA: %s is %s from %s", ref, cstatus.State, cstatus.Context)
				}
			}
			topts.ParamsRun.Clients.Log.Infof("Status on SHA: %s is %s from %s", ref, cstatus.State, cstatus.Context)
			numstatus++
		}
		topts.ParamsRun.Clients.Log.Infof("Number of gitea status on PR: %d/%d", numstatus, checkNumberOfStatus)
		if numstatus == checkNumberOfStatus {
			return
		}
		if numstatus > checkNumberOfStatus {
			t.Fatalf("Number of statuses is greater than expected, statuses: %d, expected: %d", numstatus, checkNumberOfStatus)
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
				LabelSelector: fmt.Sprintf("%s=%s\n", keys.URLRepository, topts.TargetNS),
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

func WaitForPullRequestCommentMatch(t *testing.T, topts *TestOpts) {
	i := 0
	topts.ParamsRun.Clients.Log.Infof("Looking for regexp \"%s\" in PR comments", topts.Regexp.String())
	for {
		comments, _, err := topts.GiteaCNX.Client.ListRepoIssueComments(topts.PullRequest.Base.Repository.Owner.UserName, topts.PullRequest.Base.Repository.Name, gitea.ListIssueCommentOptions{})
		assert.NilError(t, err)
		for _, v := range comments {
			if topts.Regexp.MatchString(v.Body) {
				topts.ParamsRun.Clients.Log.Infof("Found regexp in comment: %s", v.Body)
				return
			}
		}
		if i > 60 {
			t.Fatalf("gitea driver has not been posted any comment")
		}
		time.Sleep(2 * time.Second)
		i++
	}
}

func CheckIfPipelineRunsCancelled(t *testing.T, topts *TestOpts) {
	i := 0
	for {
		list, err := topts.ParamsRun.Clients.Tekton.TektonV1().PipelineRuns(topts.TargetNS).
			List(context.Background(), metav1.ListOptions{
				LabelSelector: fmt.Sprintf("%v=%v", keys.Repository, formatting.CleanValueKubernetes((topts.TargetNS))),
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

func GetStandardParams(t *testing.T, topts *TestOpts, eventType string) (repoURL, sourceURL, sourceBranch, targetBranch string) {
	t.Helper()
	var err error
	prs := &v1.PipelineRunList{}
	for i := 0; i < 21; i++ {
		prs, err = topts.ParamsRun.Clients.Tekton.TektonV1().PipelineRuns(topts.TargetNS).List(context.Background(), metav1.ListOptions{
			LabelSelector: keys.EventType + "=" + eventType,
		})
		assert.NilError(t, err)
		// get all pipelinerun names
		names := []string{}
		for _, pr := range prs.Items {
			names = append(names, pr.Name)
		}
		assert.Equal(t, len(prs.Items), 1, "should have only one "+eventType+" pipelinerun", names)

		if prs.Items[0].Status.Status.Conditions[0].Reason == "Succeeded" || prs.Items[0].Status.Status.Conditions[0].Reason == "Failed" {
			break
		}
		time.Sleep(5 * time.Second)
		if i == 20 {
			t.Fatalf("pipelinerun has not finished, something is fishy")
		}
	}
	out, err := tlogs.GetPodLog(context.Background(),
		topts.ParamsRun.Clients.Kube.CoreV1(),
		topts.TargetNS, fmt.Sprintf("tekton.dev/pipelineRun=%s",
			prs.Items[0].Name), "step-test-standard-params-value",
		github.Int64(10))
	assert.NilError(t, err)
	assert.Assert(t, out != "")
	out = strings.TrimSpace(out)
	outputDataForPR := strings.Split(out, "--")
	if len(outputDataForPR) != 5 {
		t.Fatalf("expected 5 values in outputDataForPR, got %d: %v", len(outputDataForPR), outputDataForPR)
	}

	repoURL = outputDataForPR[0]
	sourceURL = strings.TrimPrefix(outputDataForPR[1], "\n")
	sourceBranch = strings.TrimPrefix(outputDataForPR[2], "\n")
	targetBranch = strings.TrimPrefix(outputDataForPR[3], "\n")

	return repoURL, sourceURL, sourceBranch, targetBranch
}
