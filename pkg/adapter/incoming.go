package adapter

import (
	"context"
	"crypto/subtle"
	"fmt"
	"net/http"
	"os"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/formatting"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/matcher"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider/bitbucketcloud"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider/bitbucketserver"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider/gitea"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider/github"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider/github/app"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider/gitlab"
	ktypes "github.com/openshift-pipelines/pipelines-as-code/pkg/secrets/types"
	"go.uber.org/zap"
)

func compareSecret(incomingSecret, secretValue string) bool {
	return subtle.ConstantTimeCompare([]byte(incomingSecret), []byte(secretValue)) != 0
}

func (l *listener) detectIncoming(ctx context.Context, req *http.Request, payload []byte) (bool, *v1alpha1.Repository, error) {
	repository := req.URL.Query().Get("repository")
	querySecret := req.URL.Query().Get("secret")
	pipelineRun := req.URL.Query().Get("pipelinerun")
	branch := req.URL.Query().Get("branch")
	if req.URL.Path != "/incoming" {
		return false, nil, nil
	}
	if pipelineRun == "" || repository == "" || querySecret == "" || branch == "" {
		return false, nil, fmt.Errorf("missing query URL argument: pipelinerun, branch, repository, secret: %+v",
			req.URL.Query())
	}

	repo, err := matcher.GetRepo(ctx, l.run, repository)
	if err != nil {
		return false, nil, fmt.Errorf("error getting repo: %w", err)
	}
	if repo == nil {
		return false, nil, fmt.Errorf("cannot find repository %s", repository)
	}

	if repo.Spec.Incomings == nil {
		return false, nil, fmt.Errorf("you need to have incoming webhooks rules in your repo spec, repo: %s", repository)
	}

	hook := matcher.IncomingWebhookRule(branch, *repo.Spec.Incomings)
	if hook == nil {
		return false, nil, fmt.Errorf("branch '%s' has not matched any rules in repo incoming webhooks spec: %+v", branch, *repo.Spec.Incomings)
	}

	secretOpts := ktypes.GetSecretOpt{
		Namespace: repo.Namespace,
		Name:      hook.Secret.Name,
		Key:       hook.Secret.Key,
	}
	secretValue, err := l.kint.GetSecret(ctx, secretOpts)
	if err != nil {
		return false, nil, fmt.Errorf("error getting secret referenced in incoming-webhook: %w", err)
	}

	// TODO: move to somewhere common to share between gitlab and here
	if !compareSecret(querySecret, secretValue) {
		return false, nil, fmt.Errorf("secret passed to the webhook is %s which does not match with the incoming webhook secret %s in %s", secretValue, querySecret, hook.Secret.Name)
	}

	if repo.Spec.GitProvider == nil || repo.Spec.GitProvider.Type == "" {
		gh := github.New()

		// TODO: move this out of here
		ns := os.Getenv("SYSTEM_NAMESPACE")
		enterpriseURL, token, installationID, err := app.GetAndUpdateInstallationID(ctx, req, l.run, repo, gh, ns)
		if err != nil {
			return false, nil, err
		}
		l.event.Provider.URL = enterpriseURL
		l.event.Provider.Token = token
		l.event.InstallationID = installationID
		// Github app is not installed for provided repository url
		if l.event.InstallationID == 0 {
			return false, nil, fmt.Errorf("GithubApp is not installed for the provided repository url %s ", repo.Spec.URL)
		}
	}

	// TODO: more than i think about it and more i think triggertarget should be
	// eventType and vice versa, but keeping as is for now.
	l.event.EventType = "incoming"
	l.event.TriggerTarget = "push"
	l.event.TargetPipelineRun = pipelineRun
	l.event.HeadBranch = branch
	l.event.BaseBranch = branch
	l.event.Request.Header = req.Header
	l.event.Request.Payload = payload
	l.event.URL = repo.Spec.URL
	l.event.Sender = "incoming"
	return true, repo, nil
}

func (l *listener) processIncoming(targetRepo *v1alpha1.Repository) (provider.Interface, *zap.SugaredLogger, error) {
	// can a git ssh URL be a Repo URL? I don't think this will even ever work
	org, repo, err := formatting.GetRepoOwnerSplitted(targetRepo.Spec.URL)
	if err != nil {
		return nil, nil, err
	}
	l.event.Organization = org
	l.event.Repository = repo

	var provider provider.Interface
	if targetRepo.Spec.GitProvider == nil || targetRepo.Spec.GitProvider.Type == "" {
		provider = github.New()
	} else {
		switch targetRepo.Spec.GitProvider.Type {
		case "github":
			provider = github.New()
		case "gitlab":
			provider = &gitlab.Provider{}
		case "gitea":
			provider = &gitea.Provider{}
		case "bitbucket-cloud":
			provider = &bitbucketcloud.Provider{}
		case "bitbucket-server":
			provider = &bitbucketserver.Provider{}
		default:
			return l.processRes(false, nil, l.logger.With("namespace", targetRepo.Namespace), "", fmt.Errorf("no supported Git provider has been detected"))
		}
	}

	return l.processRes(true, provider, l.logger.With("provider", "incoming", "namespace", targetRepo.Namespace), "", nil)
}
