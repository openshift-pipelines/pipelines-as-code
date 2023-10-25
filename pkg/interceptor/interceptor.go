package interceptor

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
)

type Data struct {
	EventType    string `json:"eventType,omitempty"`
	BaseBranch   string `json:"baseBranch,omitempty"`
	HeadBranch   string `json:"headBranch,omitempty"`
	BaseURL      string `json:"baseURL,omitempty"`
	HeadURL      string `json:"headURL,omitempty"`
	SHA          string `json:"sha,omitempty"`
	Organization string `json:"organization,omitempty"`
	Repository   string `json:"repository,omitempty"`
	URL          string `json:"URL,omitempty"`

	// GitHub
	GithubInstallationID int64 `json:"githubInstallationID,omitempty"`

	// GHE
	GHEURL string `json:"gheURL,omitempty"`

	// Bitbucket Cloud
	BitBucketAccountID string `json:"bitBucketAccountID,omitempty"`

	// Bitbucket Server
	BitBucketCloneURL string `json:"bitBucketCloneURL,omitempty"`

	// Gitlab
	GitlabSourceProjectID int `json:"gitlabSourceProjectID,omitempty"`
	GitlabTargetProjectID int `json:"gitlabTargetProjectID,omitempty"`

	// scm providers
	// this info is needed if auto generation is suported for different scm
	Provider string `json:"provider,omitempty"`
}

type InterceptorRequest struct {
	Data  string `json:"data"`
	Token string `json:"token"`
}

type InterceptorResponse struct {
	PipelineRuns string `json:"pipelineruns"`
}

func GetPipelineRunsFromInterceptorService(ctx context.Context, request *InterceptorRequest, url string) (string, error) {
	marshalledRequest, err := json.Marshal(request)
	if err != nil {
		return "", fmt.Errorf("unable to marshal interceptorRequest struct: %s", err.Error())
	}

	// Create a HTTP post request
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(marshalledRequest))
	if err != nil {
		return "", fmt.Errorf("unable to create http request with url %s : %s", url, err.Error())
	}
	req.Header.Set("Content-Type", "application/json")

	// create http client
	// do not forget to set timeout; otherwise, no timeout!
	client := http.Client{Timeout: 10 * time.Second}
	// send the request
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("unable to make post http request to url %s : %s", url, err.Error())
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("received response %v ", resp.StatusCode)
	}

	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var interceptorResponse InterceptorResponse
	if err := json.Unmarshal(body, &interceptorResponse); err != nil {
		return "", fmt.Errorf("unable to unmarshal the response %s", err.Error())
	}
	return interceptorResponse.PipelineRuns, nil
}

func GetInterceptorRequest(providerType, url string, event *info.Event) (*InterceptorRequest, error) {
	data := getData(providerType, url, event)
	encodedData, err := encodeToBase64(data)
	if err != nil {
		return nil, err
	}
	request := InterceptorRequest{
		Data:  encodedData,
		Token: event.Provider.Token,
	}
	return &request, nil
}

func encodeToBase64(v interface{}) (string, error) {
	var buf bytes.Buffer
	encoder := base64.NewEncoder(base64.StdEncoding, &buf)
	err := json.NewEncoder(encoder).Encode(v)
	if err != nil {
		return "", err
	}
	encoder.Close()
	return buf.String(), nil
}

func getData(providerType, url string, event *info.Event) Data {
	return Data{
		EventType:             event.EventType,
		BaseBranch:            event.BaseBranch,
		HeadBranch:            event.HeadBranch,
		BaseURL:               event.BaseURL,
		HeadURL:               event.HeadURL,
		SHA:                   event.SHA,
		Organization:          event.Organization,
		Repository:            event.Repository,
		GithubInstallationID:  event.InstallationID,
		GHEURL:                event.GHEURL,
		BitBucketAccountID:    event.AccountID,
		BitBucketCloneURL:     event.CloneURL,
		GitlabSourceProjectID: event.SourceProjectID,
		GitlabTargetProjectID: event.TargetProjectID,
		Provider:              providerType,
		URL:                   url,
	}
}
