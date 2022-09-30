package github

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	jwt "github.com/golang-jwt/jwt/v4"
	"github.com/google/go-github/v47/github"
)

const acceptHeader = "application/vnd.github.v3+json"

type InstallationRepositories struct {
	Repositories []*github.Repository
}

// can't use ghinstallation because
// https://github.com/bradleyfalzon/ghinstallation/issues/39 and seems simpler
// than redoing our client again
func makeHTTPRequestWithBearerToken(ctx context.Context, url, method, bearerToken string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+bearerToken)
	req.Header.Set("Accept", acceptHeader)
	return http.DefaultClient.Do(req)
}

func (v *Provider) getFileContentViaAPI(ctx context.Context, url, token string) (*github.RepositoryContent, error) {
	resp, err := makeHTTPRequestWithBearerToken(ctx, url, "GET", token)
	if err != nil {
		return nil, err
	}
	ret, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("error getting content from github: %d: %s", resp.StatusCode, ret)
	}
	repoContent := &github.RepositoryContent{}
	if err := json.Unmarshal(ret, repoContent); err != nil {
		return nil, err
	}
	return repoContent, nil
}

func (v *Provider) matchTaskRepoInstallURL(ctx context.Context, installationID int64, uri string) (bool, string, error) {
	jwToken, err := v.generateJWTToken()
	if err != nil {
		return false, "", err
	}
	token, err := v.getAccessTokenForInstallation(ctx, &installationID, jwToken)
	if err != nil {
		return false, "", err
	}
	spOrg, spRepo, spPath, spRef, err := v.splitGithubURL(uri)
	if err != nil {
		return false, "", err
	}
	url := fmt.Sprintf("%srepos/%s/%s/contents/%s?ref=%s", *v.APIURL, spOrg, spRepo, spPath, spRef)
	content, err := v.getFileContentViaAPI(ctx, url, *token.Token)
	if err != nil {
		return false, "", err
	}
	if content.GetType() != "file" {
		return false, "", fmt.Errorf("URL %s does not seem to be a proper proper file", uri)
	}
	decoded, err := base64.StdEncoding.DecodeString(*content.Content)
	if err != nil {
		return false, "", err
	}
	return true, string(decoded), nil
}

func (v *Provider) generateJWTToken() (string, error) {
	key, err := jwt.ParseRSAPrivateKeyFromPEM(v.GithubAppPrivateKey)
	if err != nil {
		return "", fmt.Errorf("could not parse private key: %w", err)
	}
	iss := &jwt.NumericDate{Time: time.Now().Add(-30 * time.Second).Truncate(time.Second)}
	exp := &jwt.NumericDate{Time: iss.Add(2 * time.Minute)}
	claims := &jwt.RegisteredClaims{
		IssuedAt:  iss,
		ExpiresAt: exp,
		Issuer:    strconv.FormatInt(v.GithubAppAppID, 10),
	}
	bearer := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	return bearer.SignedString(key)
}

func (v *Provider) getAllInstallationOfApp(ctx context.Context) error {
	jwtToken, err := v.generateJWTToken()
	if err != nil {
		return err
	}
	resp, err := makeHTTPRequestWithBearerToken(ctx, fmt.Sprintf("%sapp/installations", *v.APIURL), "GET", jwtToken)
	if err != nil {
		return err
	}
	ret, err := io.ReadAll(resp.Body)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("error getting installation token for the github app: %d, %s", resp.StatusCode, ret)
	}

	var installations []*github.Installation
	if err := json.Unmarshal(ret, &installations); err != nil {
		return err
	}
	v.GithubAppInstallations = map[int64][]*github.Repository{}
	for _, installation := range installations {
		repos, err := v.getReposOfAnInstallation(ctx, installation.ID)
		if err != nil {
			return err
		}
		v.GithubAppInstallations[*installation.ID] = repos.Repositories
	}
	return err
}

func (v *Provider) getAccessTokenForInstallation(ctx context.Context, installationID *int64, jwtToken string) (*github.InstallationToken, error) {
	resp, err := makeHTTPRequestWithBearerToken(ctx,
		fmt.Sprintf("%sapp/installations/%d/access_tokens",
			*v.APIURL, *installationID), "POST", jwtToken)
	if err != nil {
		return nil, err
	}
	ret, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("error getting installation token for the github app %d: %d, %s", *installationID, resp.StatusCode, ret)
	}
	var installToken *github.InstallationToken
	if err := json.Unmarshal(ret, &installToken); err != nil {
		return nil, err
	}
	return installToken, nil
}

func (v *Provider) getReposOfAnInstallation(ctx context.Context, installationID *int64) (*InstallationRepositories, error) {
	jwtToken, err := v.generateJWTToken()
	if err != nil {
		return nil, err
	}
	token, err := v.getAccessTokenForInstallation(ctx, installationID, jwtToken)
	if err != nil {
		return nil, err
	}
	resp, err := makeHTTPRequestWithBearerToken(ctx,
		fmt.Sprintf("%sinstallation/repositories", *v.APIURL), "GET", *token.Token)
	if err != nil {
		return nil, err
	}

	ret, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var installationRepositories *InstallationRepositories
	if err := json.Unmarshal(ret, &installationRepositories); err != nil {
		return nil, err
	}
	return installationRepositories, nil
}
