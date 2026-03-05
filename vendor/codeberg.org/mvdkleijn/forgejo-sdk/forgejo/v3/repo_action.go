// Copyright 2024 The Forgejo Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

// Copyright 2024 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package forgejo

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

// ListRepoActionSecretOption list RepoActionSecret options
type ListRepoActionSecretOption struct {
	ListOptions
}

// ListRepoActionSecret list a repository's secrets
func (c *Client) ListRepoActionSecret(user, repo string, opt ListRepoActionSecretOption) ([]*Secret, *Response, error) {
	if err := escapeValidatePathSegments(&user, &repo); err != nil {
		return nil, nil, err
	}
	opt.setDefaults()
	secrets := make([]*Secret, 0, opt.PageSize)

	link, _ := url.Parse(fmt.Sprintf("/repos/%s/%s/actions/secrets", user, repo))
	link.RawQuery = opt.getURLQuery().Encode()
	resp, err := c.getParsedResponse("GET", link.String(), jsonHeader, nil, &secrets)
	return secrets, resp, err
}

// CreateRepoActionSecret creates a secret for the specified repository in the Gitea Actions.
// It takes the organization name and the secret options as parameters.
// The function returns the HTTP response and an error, if any.
func (c *Client) CreateRepoActionSecret(user, repo string, opt CreateSecretOption) (*Response, error) {
	if err := escapeValidatePathSegments(&user, &repo); err != nil {
		return nil, err
	}
	if err := (&opt).Validate(); err != nil {
		return nil, err
	}
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, err
	}

	status, resp, err := c.getStatusCode("PUT", fmt.Sprintf("/repos/%s/%s/actions/secrets/%s", user, repo, opt.Name), jsonHeader, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	switch status {
	case http.StatusCreated:
		return resp, nil
	case http.StatusNoContent:
		return resp, nil
	case http.StatusNotFound:
		return resp, fmt.Errorf("forbidden")
	case http.StatusBadRequest:
		return resp, fmt.Errorf("bad request")
	default:
		return resp, fmt.Errorf("unexpected Status: %d", status)
	}
}

// DeleteRepoActionSecret deletes a secret in a repository
func (c *Client) DeleteRepoActionSecret(owner, repo, secretName string) (*Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo, &secretName); err != nil {
		return nil, err
	}

	status, resp, err := c.getStatusCode("DELETE", fmt.Sprintf("/repos/%s/%s/actions/secrets/%s", owner, repo, secretName), jsonHeader, nil)
	if err != nil {
		return nil, err
	}

	switch status {
	case http.StatusNoContent:
		return resp, nil
	case http.StatusNotFound:
		return resp, fmt.Errorf("secret not found")
	default:
		return resp, fmt.Errorf("unexpected Status: %d", status)
	}
}

// ListRepoActionRuns lists a repository's action runs
func (c *Client) ListRepoActionRuns(owner, repo string, opt ListActionRunsOption) (*ListActionRunsResponse, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
	opt.setDefaults()

	link, _ := url.Parse(fmt.Sprintf("/repos/%s/%s/actions/runs", owner, repo))
	link.RawQuery = opt.QueryEncode()

	runs := new(ListActionRunsResponse)
	resp, err := c.getParsedResponse("GET", link.String(), jsonHeader, nil, runs)
	return runs, resp, err
}

// GetRepoActionRun gets a specific action run
func (c *Client) GetRepoActionRun(owner, repo string, runID int64) (*ActionRun, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}

	run := new(ActionRun)
	resp, err := c.getParsedResponse("GET", fmt.Sprintf("/repos/%s/%s/actions/runs/%d", owner, repo, runID), jsonHeader, nil, run)
	return run, resp, err
}

// DispatchRepoWorkflow triggers a workflow dispatch event
func (c *Client) DispatchRepoWorkflow(owner, repo, workflow string, opt DispatchWorkflowOption) (*DispatchWorkflowResponse, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo, &workflow); err != nil {
		return nil, nil, err
	}

	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, nil, err
	}

	// If ReturnRunInfo is true, we expect a response body
	if opt.ReturnRunInfo {
		dispatchResp := new(DispatchWorkflowResponse)
		resp, err := c.getParsedResponse("POST", fmt.Sprintf("/repos/%s/%s/actions/workflows/%s/dispatches", owner, repo, workflow), jsonHeader, bytes.NewReader(body), dispatchResp)
		return dispatchResp, resp, err
	}

	// Otherwise, expect 204 No Content
	status, resp, err := c.getStatusCode("POST", fmt.Sprintf("/repos/%s/%s/actions/workflows/%s/dispatches", owner, repo, workflow), jsonHeader, bytes.NewReader(body))
	if err != nil {
		return nil, nil, err
	}

	switch status {
	case http.StatusNoContent, http.StatusOK:
		return nil, resp, nil
	case http.StatusNotFound:
		return nil, resp, fmt.Errorf("workflow not found")
	default:
		return nil, resp, fmt.Errorf("unexpected Status: %d", status)
	}
}

// ListRepoActionTasks lists a repository's action tasks
func (c *Client) ListRepoActionTasks(owner, repo string, opt ListActionTasksOption) (*ListActionTasksResponse, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
	opt.setDefaults()

	link, _ := url.Parse(fmt.Sprintf("/repos/%s/%s/actions/tasks", owner, repo))
	link.RawQuery = opt.getURLQuery().Encode()

	tasks := new(ListActionTasksResponse)
	resp, err := c.getParsedResponse("GET", link.String(), jsonHeader, nil, tasks)
	return tasks, resp, err
}

// ListRepoActionJobs searches for repository action jobs
func (c *Client) ListRepoActionJobs(owner, repo string, opt ListActionJobsOption) ([]*ActionRunJob, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}

	link, _ := url.Parse(fmt.Sprintf("/repos/%s/%s/actions/runners/jobs", owner, repo))
	if opt.Labels != "" {
		query := link.Query()
		query.Add("labels", opt.Labels)
		link.RawQuery = query.Encode()
	}

	jobs := make([]*ActionRunJob, 0)
	resp, err := c.getParsedResponse("GET", link.String(), jsonHeader, nil, &jobs)
	return jobs, resp, err
}

// ListRepoActionVariables lists a repository's action variables
func (c *Client) ListRepoActionVariables(owner, repo string, opt ListOptions) ([]*ActionVariable, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
	opt.setDefaults()

	link, _ := url.Parse(fmt.Sprintf("/repos/%s/%s/actions/variables", owner, repo))
	link.RawQuery = opt.getURLQuery().Encode()

	variables := make([]*ActionVariable, 0, opt.PageSize)
	resp, err := c.getParsedResponse("GET", link.String(), jsonHeader, nil, &variables)
	return variables, resp, err
}

// GetRepoActionVariable gets a specific action variable
func (c *Client) GetRepoActionVariable(owner, repo, variableName string) (*ActionVariable, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo, &variableName); err != nil {
		return nil, nil, err
	}

	variable := new(ActionVariable)
	resp, err := c.getParsedResponse("GET", fmt.Sprintf("/repos/%s/%s/actions/variables/%s", owner, repo, variableName), jsonHeader, nil, variable)
	return variable, resp, err
}

// CreateRepoActionVariable creates an action variable for a repository
func (c *Client) CreateRepoActionVariable(owner, repo string, opt CreateVariableOption) (*Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, err
	}
	if err := opt.Validate(); err != nil {
		return nil, err
	}

	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, err
	}

	status, resp, err := c.getStatusCode("POST", fmt.Sprintf("/repos/%s/%s/actions/variables/%s", owner, repo, opt.Name), jsonHeader, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	switch status {
	case http.StatusCreated, http.StatusNoContent:
		return resp, nil
	case http.StatusConflict:
		return resp, fmt.Errorf("variable already exists")
	default:
		return resp, fmt.Errorf("unexpected Status: %d", status)
	}
}

// UpdateRepoActionVariable updates an action variable for a repository
func (c *Client) UpdateRepoActionVariable(owner, repo, variableName string, opt CreateVariableOption) (*Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo, &variableName); err != nil {
		return nil, err
	}
	if err := opt.Validate(); err != nil {
		return nil, err
	}

	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, err
	}

	status, resp, err := c.getStatusCode("PUT", fmt.Sprintf("/repos/%s/%s/actions/variables/%s", owner, repo, variableName), jsonHeader, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	switch status {
	case http.StatusOK, http.StatusNoContent:
		return resp, nil
	case http.StatusNotFound:
		return resp, fmt.Errorf("variable not found")
	default:
		return resp, fmt.Errorf("unexpected Status: %d", status)
	}
}

// DeleteRepoActionVariable deletes an action variable from a repository
func (c *Client) DeleteRepoActionVariable(owner, repo, variableName string) (*Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo, &variableName); err != nil {
		return nil, err
	}

	status, resp, err := c.getStatusCode("DELETE", fmt.Sprintf("/repos/%s/%s/actions/variables/%s", owner, repo, variableName), jsonHeader, nil)
	if err != nil {
		return nil, err
	}

	switch status {
	case http.StatusNoContent, http.StatusOK:
		return resp, nil
	case http.StatusNotFound:
		return resp, fmt.Errorf("variable not found")
	default:
		return resp, fmt.Errorf("unexpected Status: %d", status)
	}
}

// GetRepoActionRunnerRegistrationToken gets a runner registration token for a repository
func (c *Client) GetRepoActionRunnerRegistrationToken(owner, repo string) (*RunnerRegistrationToken, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}

	token := new(RunnerRegistrationToken)
	resp, err := c.getParsedResponse("GET", fmt.Sprintf("/repos/%s/%s/actions/runners/registration-token", owner, repo), jsonHeader, nil, token)
	return token, resp, err
}
