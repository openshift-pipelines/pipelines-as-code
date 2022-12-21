package adapter

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/golang-jwt/jwt/v4"
	gt "github.com/google/go-github/v48/github"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/keys"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/formatting"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/matcher"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider/bitbucketcloud"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider/bitbucketserver"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider/gitea"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider/github"
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

	var isProviderTypeNotSet bool
	if repo.Spec.GitProvider == nil {
		isProviderTypeNotSet = true
	} else if repo.Spec.GitProvider.Type == "" {
		isProviderTypeNotSet = true
	}
	if isProviderTypeNotSet {
		if err = l.getAndUpdateInstallationID(ctx, req, l.run, repo); err != nil {
			return false, nil, err
		}
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

func (l *listener) getAndUpdateInstallationID(ctx context.Context, req *http.Request, run *params.Run, repo *v1alpha1.Repository) error {
	jwtToken, err := GenerateJWT(ctx, run)
	if err != nil {
		return err
	}

	installationURL := keys.APIURL + keys.InstallationURL
	enterpriseURL := req.Header.Get("X-GitHub-Enterprise-Host")
	if enterpriseURL != "" {
		installationURL = enterpriseURL + keys.InstallationURL
	}

	res, err := getResponse(ctx, http.MethodGet, installationURL, jwtToken, run)
	if err != nil {
		return err
	}

	defer res.Body.Close()
	data, err := io.ReadAll(res.Body)
	if err != nil {
		return err
	}

	installationData := []gt.Installation{}
	if err = json.Unmarshal(data, &installationData); err != nil {
		return err
	}

	/* each installationID can have list of repository
	ref: https://docs.github.com/en/developers/apps/building-github-apps/authenticating-with-github-apps#authenticating-as-an-installation ,
	     https://docs.github.com/en/rest/apps/installations?apiVersion=2022-11-28#list-repositories-accessible-to-the-app-installation */
	for i := range installationData {
		l.event.Provider.URL = enterpriseURL
		gh := github.New()
		if *installationData[i].ID != 0 {
			l.event.Provider.Token, err = gh.GetAppToken(ctx, run.Clients.Kube, l.event.Provider.URL, *installationData[i].ID)
			if err != nil {
				return err
			}
		}
		repoList, err := gh.ListRepos(ctx)
		if err != nil {
			return err
		}
		for i := range repoList {
			// If URL matches with repo spec url then we can break for loop
			if repoList[i] == repo.Spec.URL {
				l.event.InstallationID = *installationData[i].ID
				break
			}
		}
	}
	return nil
}

func getResponse(ctx context.Context, method, urlData, jwtToken string, run *params.Run) (*http.Response, error) {
	rawurl, err := url.Parse(urlData)
	if err != nil {
		return nil, err
	}

	newreq, err := http.NewRequestWithContext(ctx, method, rawurl.String(), nil)
	if err != nil {
		return nil, err
	}
	newreq.Header = map[string][]string{
		"Accept":        {"application/vnd.github+json"},
		"Authorization": {fmt.Sprintf("Bearer %s", jwtToken)},
	}
	res, err := run.Clients.HTTP.Do(newreq)
	return res, err
}

func (l *listener) processIncoming(targetRepo *v1alpha1.Repository) (provider.Interface, *zap.SugaredLogger, error) {
	// can a git ssh URL be a Repo URL? I don't think this will even ever work
	org, repo, err := formatting.GetRepoOwnerSplitted(targetRepo.Spec.URL)
	if err != nil {
		return nil, nil, err
	}
	l.event.Organization = org
	l.event.Repository = repo

	var isProviderTypeNotSet bool
	if targetRepo.Spec.GitProvider == nil {
		isProviderTypeNotSet = true
	} else if targetRepo.Spec.GitProvider.Type == "" {
		isProviderTypeNotSet = true
	}

	var provider provider.Interface
	if isProviderTypeNotSet {
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
			return l.processRes(false, nil, l.logger, "", fmt.Errorf("no supported Git provider has been detected"))
		}
	}

	return l.processRes(true, provider, l.logger.With("provider", "incoming"), "", nil)
}

type JWTClaim struct {
	Issuer int64 `json:"iss"`
	jwt.RegisteredClaims
}

func GenerateJWT(ctx context.Context, run *params.Run) (string, error) {
	applicationID, privateKey, err := github.GetAppIDAndPrivateKey(ctx, run.Clients.Kube)
	if err != nil {
		return "", err
	}

	expirationTime := time.Now().Add(5 * time.Minute)
	claims := &JWTClaim{
		Issuer: applicationID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)

	parsedPK, err := jwt.ParseRSAPrivateKeyFromPEM(privateKey)
	if err != nil {
		return "", fmt.Errorf("failed to parse private key: %w", err)
	}

	tokenString, err := token.SignedString(parsedPK)
	if err != nil {
		return "", fmt.Errorf("failed to sign private key: %w", err)
	}
	return tokenString, nil
}
