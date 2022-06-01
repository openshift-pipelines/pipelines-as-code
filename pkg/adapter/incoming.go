package adapter

import (
	"context"
	"crypto/subtle"
	"fmt"
	"net/http"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/formatting"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/kubeinteraction"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/matcher"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider/bitbucketcloud"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider/bitbucketserver"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider/github"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider/gitlab"
	"go.uber.org/zap"
)

func compareSecret(incomingSecret string, secretValue string) bool {
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
	if repository == "" || querySecret == "" || branch == "" {
		return false, nil, fmt.Errorf("missing query URL argument: branch, repository, secret: %+v",
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

	secretOpts := kubeinteraction.GetSecretOpt{
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
		return false, nil, fmt.Errorf("secret passed to the webhook does not match incoming webhook secret in %s", hook.Secret.Name)
	}

	if repo.Spec.GitProvider.Type == "" {
		return false, nil, fmt.Errorf("repo %s has no git provider type, you need to specify one, ie: github", repository)
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
	switch targetRepo.Spec.GitProvider.Type {
	case "github":
		provider = &github.Provider{}
	case "gitlab":
		provider = &gitlab.Provider{}
	case "bitbucket-cloud":
		provider = &bitbucketcloud.Provider{}
	case "bitbucket-server":
		provider = &bitbucketserver.Provider{}
	default:
		return l.processRes(false, nil, l.logger, "", fmt.Errorf("no supported Git provider has been detected"))
	}
	return l.processRes(true, provider, l.logger.With("provider", "incoming"), "", nil)
}
