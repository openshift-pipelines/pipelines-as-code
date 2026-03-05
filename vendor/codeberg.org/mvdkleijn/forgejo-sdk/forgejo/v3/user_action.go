// Copyright 2024 The Forgejo Authors. All rights reserved.
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

// CreateUserActionSecret creates a secret for the authenticated user
func (c *Client) CreateUserActionSecret(opt CreateSecretOption) (*Response, error) {
	if err := (&opt).Validate(); err != nil {
		return nil, err
	}

	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, err
	}

	secretName := opt.Name
	if err := escapeValidatePathSegments(&secretName); err != nil {
		return nil, err
	}

	status, resp, err := c.getStatusCode("PUT", fmt.Sprintf("/user/actions/secrets/%s", secretName), jsonHeader, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	switch status {
	case http.StatusCreated, http.StatusNoContent:
		return resp, nil
	case http.StatusBadRequest:
		return resp, fmt.Errorf("bad request")
	default:
		return resp, fmt.Errorf("unexpected Status: %d", status)
	}
}

// DeleteUserActionSecret deletes a secret for the authenticated user
func (c *Client) DeleteUserActionSecret(secretName string) (*Response, error) {
	if err := escapeValidatePathSegments(&secretName); err != nil {
		return nil, err
	}

	status, resp, err := c.getStatusCode("DELETE", fmt.Sprintf("/user/actions/secrets/%s", secretName), jsonHeader, nil)
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

// ListUserActionJobs searches for user action jobs
func (c *Client) ListUserActionJobs(opt ListActionJobsOption) ([]*ActionRunJob, *Response, error) {
	link, _ := url.Parse("/user/actions/runners/jobs")
	if opt.Labels != "" {
		query := link.Query()
		query.Add("labels", opt.Labels)
		link.RawQuery = query.Encode()
	}

	jobs := make([]*ActionRunJob, 0)
	resp, err := c.getParsedResponse("GET", link.String(), jsonHeader, nil, &jobs)
	return jobs, resp, err
}

// ListUserActionVariables lists action variables for the authenticated user
func (c *Client) ListUserActionVariables(opt ListOptions) ([]*ActionVariable, *Response, error) {
	opt.setDefaults()

	link, _ := url.Parse("/user/actions/variables")
	link.RawQuery = opt.getURLQuery().Encode()

	variables := make([]*ActionVariable, 0, opt.PageSize)
	resp, err := c.getParsedResponse("GET", link.String(), jsonHeader, nil, &variables)
	return variables, resp, err
}

// GetUserActionVariable gets a specific user action variable
func (c *Client) GetUserActionVariable(variableName string) (*ActionVariable, *Response, error) {
	if err := escapeValidatePathSegments(&variableName); err != nil {
		return nil, nil, err
	}

	variable := new(ActionVariable)
	resp, err := c.getParsedResponse("GET", fmt.Sprintf("/user/actions/variables/%s", variableName), jsonHeader, nil, variable)
	return variable, resp, err
}

// CreateUserActionVariable creates an action variable for the authenticated user
func (c *Client) CreateUserActionVariable(opt CreateVariableOption) (*Response, error) {
	if err := opt.Validate(); err != nil {
		return nil, err
	}

	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, err
	}

	variableName := opt.Name
	if err := escapeValidatePathSegments(&variableName); err != nil {
		return nil, err
	}

	status, resp, err := c.getStatusCode("POST", fmt.Sprintf("/user/actions/variables/%s", variableName), jsonHeader, bytes.NewReader(body))
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

// UpdateUserActionVariable updates an action variable for the authenticated user
func (c *Client) UpdateUserActionVariable(variableName string, opt CreateVariableOption) (*Response, error) {
	if err := escapeValidatePathSegments(&variableName); err != nil {
		return nil, err
	}
	if err := opt.Validate(); err != nil {
		return nil, err
	}

	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, err
	}

	status, resp, err := c.getStatusCode("PUT", fmt.Sprintf("/user/actions/variables/%s", variableName), jsonHeader, bytes.NewReader(body))
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

// DeleteUserActionVariable deletes an action variable for the authenticated user
func (c *Client) DeleteUserActionVariable(variableName string) (*Response, error) {
	if err := escapeValidatePathSegments(&variableName); err != nil {
		return nil, err
	}

	status, resp, err := c.getStatusCode("DELETE", fmt.Sprintf("/user/actions/variables/%s", variableName), jsonHeader, nil)
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

// GetUserActionRunnerRegistrationToken gets a runner registration token for the authenticated user
func (c *Client) GetUserActionRunnerRegistrationToken() (*RunnerRegistrationToken, *Response, error) {
	token := new(RunnerRegistrationToken)
	resp, err := c.getParsedResponse("GET", "/user/actions/runners/registration-token", jsonHeader, nil, token)
	return token, resp, err
}
