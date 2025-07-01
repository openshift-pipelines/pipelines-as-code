package app

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider/github"
	"knative.dev/pkg/logging"
)

type Install struct {
	request   *http.Request
	run       *params.Run
	repo      *v1alpha1.Repository
	ghClient  *github.Provider
	namespace string

	repoList []string
}

func NewInstallation(req *http.Request, run *params.Run, repo *v1alpha1.Repository, gh *github.Provider, namespace string) *Install {
	if req == nil {
		req = &http.Request{}
	}
	return &Install{
		request:   req,
		run:       run,
		repo:      repo,
		ghClient:  gh,
		namespace: namespace,
	}
}

// GetAndUpdateInstallationID retrieves and updates the installation ID for the GitHub App.
// It generates a JWT token, and directly fetches the installation for the
// repository.
func (ip *Install) GetAndUpdateInstallationID(ctx context.Context) (string, string, int64, error) {
	logger := logging.FromContext(ctx)

	// Generate a JWT token for authentication
	jwtToken, err := ip.GenerateJWT(ctx)
	if err != nil {
		return "", "", 0, err
	}

	// Get owner and repo from the repository URL
	repoURL, err := url.Parse(ip.repo.Spec.URL)
	if err != nil {
		return "", "", 0, fmt.Errorf("failed to parse repository URL: %w", err)
	}
	pathParts := strings.Split(strings.Trim(repoURL.Path, "/"), "/")
	if len(pathParts) < 2 {
		return "", "", 0, fmt.Errorf("invalid repository URL path: %s", repoURL.Path)
	}
	owner := pathParts[0]
	repoName := pathParts[1]

	apiURL := *ip.ghClient.APIURL
	enterpriseHost := ip.request.Header.Get("X-GitHub-Enterprise-Host")
	if enterpriseHost != "" {
		apiURL = "https://" + enterpriseHost + "/api/v3"
	}

	client, _, _ := github.MakeClient(ctx, apiURL, jwtToken)
	// Directly get the installation for the repository
	installation, _, err := client.Apps.FindRepositoryInstallation(ctx, owner, repoName)
	if err != nil {
		// Fallback to finding organization installation if repository installation is not found
		installation, _, err = client.Apps.FindOrganizationInstallation(ctx, owner)
		if err != nil {
			return "", "", 0, fmt.Errorf("could not find repository or organization installation for %s/%s: %w", owner, repoName, err)
		}
	}

	if installation.ID == nil {
		return "", "", 0, fmt.Errorf("installation ID is nil")
	}

	installationID := *installation.ID
	token, err := ip.ghClient.GetAppToken(ctx, ip.run.Clients.Kube, enterpriseHost, installationID, ip.namespace)
	if err != nil {
		logger.Warnf("Could not get a token for installation ID %d: %v", installationID, err)
		// Return with the installation ID even if token generation fails,
		// as some operations might only need the ID.
		return enterpriseHost, "", installationID, nil
	}

	return enterpriseHost, token, installationID, nil
}

// matchRepos matches GitHub repositories to their installation IDs.
// It lists all repositories accessible to the app installation and checks if
// any match the repository URL in the spec.
func (ip *Install) matchRepos(ctx context.Context) (bool, error) {
	installationRepoList, err := github.ListRepos(ctx, ip.ghClient)
	if err != nil {
		return false, err
	}
	ip.repoList = append(ip.repoList, installationRepoList...)
	if slices.Contains(installationRepoList, ip.repo.Spec.URL) {
		return true, nil
	}
	return false, nil
}

// JWTClaim represents the JWT claims for the GitHub App.
type JWTClaim struct {
	Issuer int64 `json:"iss"`
	jwt.RegisteredClaims
}

// GenerateJWT generates a JWT token for the GitHub App.
// It retrieves the application ID and private key, sets the claims, and signs the token.
func (ip *Install) GenerateJWT(ctx context.Context) (string, error) {
	// TODO: move this out of here
	gh := github.New()
	gh.Run = ip.run
	applicationID, privateKey, err := gh.GetAppIDAndPrivateKey(ctx, ip.namespace, ip.run.Clients.Kube)
	if err != nil {
		return "", err
	}

	// The expirationTime claim identifies the expiration time on or after which the JWT MUST NOT be accepted for processing.
	// Value cannot be longer duration.
	// See https://datatracker.ietf.org/doc/html/rfc7519#section-4.1.4
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
