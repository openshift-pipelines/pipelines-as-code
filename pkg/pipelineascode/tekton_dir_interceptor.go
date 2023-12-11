package pipelineascode

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
	"github.com/openshift-pipelines/pipelines-as-code/pkg/resolve"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"go.uber.org/zap"
)

type Data struct {
	// Payload object
	Payload []byte `json:"payload,omitempty"`

	EventType  string `json:"eventType,omitempty"`
	BaseBranch string `json:"baseBranch,omitempty"`
	HeadBranch string `json:"headBranch,omitempty"`
	BaseURL    string `json:"baseURL,omitempty"`
	HeadURL    string `json:"headURL,omitempty"`
	SHA        string `json:"sha,omitempty"`

	// Github
	GithubOrganization   string `json:"githubOrganization,omitempty"`
	GithubRepository     string `json:"githubRepository,omitempty"`
	GithubInstallationID int64  `json:"githubInstallationID,omitempty"`

	// GHE
	GHEURL string `json:"gheURL,omitempty"`

	// Bitbucket Cloud
	BitBucketAccountID string `json:"bitBucketAccountID,omitempty"`

	// Bitbucket Server
	BitBucketCloneURL string `json:"bitBucketCloneURL,omitempty"`

	// Gitlab
	GitlabSourceProjectID int `json:"gitlabSourceProjectID,omitempty"`
	GitlabTargetProjectID int `json:"gitlabTargetProjectID,omitempty"`
}

type InterceptorRequest struct {
	Data  string `json:"data"`
	Token string `json:"token"`
}

type InterceptorResponse struct {
	Tasks        []*v1.Task        `json:"tasks,omitempty"`
	Pipelines    []*v1.Pipeline    `json:"pipelines,omitempty"`
	PipelineRuns []*v1.PipelineRun `json:"pipelineruns"`
}

func (p *PacRun) getPipelineRunsFromService(ctx context.Context, request *InterceptorRequest) (*resolve.TektonTypes, error) {
	url := p.run.Info.Pac.TektonDirInterceptorURL

	// marshall data to json (like json_encode)
	marshalledRequest, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("unable to marshal interceptorRequest struct: %s", err.Error())
	}

	// Create a HTTP post request
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(marshalledRequest))
	if err != nil {
		return nil, fmt.Errorf("unable to create http request with url %s : %s", url, err.Error())
	}
	req.Header.Set("Content-Type", "application/json")

	// create http client
	// do not forget to set timeout; otherwise, no timeout!
	client := http.Client{Timeout: 10 * time.Second}
	// send the request
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("unable to make post http request to url %s : %s", url, err.Error())
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("received response %v : %s", resp.StatusCode, err.Error())
	}

	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var interceptorResponse InterceptorResponse
	if err := json.Unmarshal(body, &interceptorResponse); err != nil {
		return nil, fmt.Errorf("unable to unmarshal the response %s", err.Error())
	}

	if len(interceptorResponse.PipelineRuns) == 0 {
		msg := fmt.Sprintf("no pipelineruns received for this repository from %s interceptor service", url)
		p.eventEmitter.EmitMessage(nil, zap.InfoLevel, "RepositoryCannotLocatePipelineRun", msg)
		return nil, nil
	}

	return &resolve.TektonTypes{
		Tasks:        interceptorResponse.Tasks,
		Pipelines:    interceptorResponse.Pipelines,
		PipelineRuns: interceptorResponse.PipelineRuns,
	}, nil
}

func (p *PacRun) getInterceptorRequest() (*InterceptorRequest, error) {
	data := getData(p.event, p.payload)
	encodedData, err := encodeToBase64(data)
	if err != nil {
		return nil, err
	}
	request := InterceptorRequest{
		Data:  encodedData,
		Token: p.vcx.GetToken(),
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

func getData(event *info.Event, payload []byte) Data {
	return Data{
		Payload:               payload,
		EventType:             event.EventType,
		BaseBranch:            event.BaseBranch,
		HeadBranch:            event.HeadBranch,
		BaseURL:               event.BaseURL,
		HeadURL:               event.HeadURL,
		SHA:                   event.SHA,
		GithubOrganization:    event.Organization,
		GithubRepository:      event.Repository,
		GithubInstallationID:  event.InstallationID,
		GHEURL:                event.GHEURL,
		BitBucketAccountID:    event.AccountID,
		BitBucketCloneURL:     event.CloneURL,
		GitlabSourceProjectID: event.SourceProjectID,
		GitlabTargetProjectID: event.TargetProjectID,
	}
}
