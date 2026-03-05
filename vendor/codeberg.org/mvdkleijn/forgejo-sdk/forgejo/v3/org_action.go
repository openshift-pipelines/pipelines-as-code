// Copyright 2024 The Forgejo Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

// Copyright 2023 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package forgejo

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

// ListOrgActionSecretOption list OrgActionSecret options
type ListOrgActionSecretOption struct {
	ListOptions
}

// ListOrgActionSecret list an organization's secrets
func (c *Client) ListOrgActionSecret(org string, opt ListOrgActionSecretOption) ([]*Secret, *Response, error) {
	if err := escapeValidatePathSegments(&org); err != nil {
		return nil, nil, err
	}
	opt.setDefaults()
	secrets := make([]*Secret, 0, opt.PageSize)

	link, _ := url.Parse(fmt.Sprintf("/orgs/%s/actions/secrets", org))
	link.RawQuery = opt.getURLQuery().Encode()
	resp, err := c.getParsedResponse("GET", link.String(), jsonHeader, nil, &secrets)
	return secrets, resp, err
}

// CreateSecretOption represents the options for creating a secret.
type CreateSecretOption struct {
	Name string `json:"name"` // Name is the name of the secret.
	Data string `json:"data"` // Data is the data of the secret.
}

// Validate checks if the CreateSecretOption is valid.
// It returns an error if any of the validation checks fail.
// Validation rules:
// - Name is required and must not exceed 255 characters
// - Name must contain only alphanumeric characters and underscores (case-insensitive)
// - Name must not start with GITEA_ or GITHUB_ (reserved prefixes)
// - Data is required
func (opt *CreateSecretOption) Validate() error {
	if len(opt.Name) == 0 {
		return fmt.Errorf("name required")
	}
	if len(opt.Name) > 255 {
		return fmt.Errorf("name too long (maximum 255 characters)")
	}

	// Validate name format: alphanumeric and underscores only
	validNamePattern := regexp.MustCompile(`^[a-zA-Z0-9_]+$`)
	if !validNamePattern.MatchString(opt.Name) {
		return fmt.Errorf("name must contain only alphanumeric characters and underscores")
	}

	// Check for reserved prefixes (case-insensitive)
	nameUpper := strings.ToUpper(opt.Name)
	if strings.HasPrefix(nameUpper, "GITEA_") || strings.HasPrefix(nameUpper, "GITHUB_") {
		return fmt.Errorf("name cannot start with GITEA_ or GITHUB_ (reserved prefixes)")
	}

	if len(opt.Data) == 0 {
		return fmt.Errorf("data required")
	}
	return nil
}

// CreateOrgActionSecret creates a secret for the specified organization in the Gitea Actions.
// It takes the organization name and the secret options as parameters.
// The function returns the HTTP response and an error, if any.
func (c *Client) CreateOrgActionSecret(org string, opt CreateSecretOption) (*Response, error) {
	if err := escapeValidatePathSegments(&org); err != nil {
		return nil, err
	}
	if err := (&opt).Validate(); err != nil {
		return nil, err
	}
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, err
	}

	status, resp, err := c.getStatusCode("PUT", fmt.Sprintf("/orgs/%s/actions/secrets/%s", org, opt.Name), jsonHeader, bytes.NewReader(body))
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

// DeleteOrgActionSecret deletes a secret in an organization
func (c *Client) DeleteOrgActionSecret(org, secretName string) (*Response, error) {
	if err := escapeValidatePathSegments(&org, &secretName); err != nil {
		return nil, err
	}

	status, resp, err := c.getStatusCode("DELETE", fmt.Sprintf("/orgs/%s/actions/secrets/%s", org, secretName), jsonHeader, nil)
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

// ListOrgActionJobs searches for organization action jobs
func (c *Client) ListOrgActionJobs(org string, opt ListActionJobsOption) ([]*ActionRunJob, *Response, error) {
	if err := escapeValidatePathSegments(&org); err != nil {
		return nil, nil, err
	}

	link, _ := url.Parse(fmt.Sprintf("/orgs/%s/actions/runners/jobs", org))
	if opt.Labels != "" {
		query := link.Query()
		query.Add("labels", opt.Labels)
		link.RawQuery = query.Encode()
	}

	jobs := make([]*ActionRunJob, 0)
	resp, err := c.getParsedResponse("GET", link.String(), jsonHeader, nil, &jobs)
	return jobs, resp, err
}

// ListOrgActionVariables lists an organization's action variables
func (c *Client) ListOrgActionVariables(org string, opt ListOptions) ([]*ActionVariable, *Response, error) {
	if err := escapeValidatePathSegments(&org); err != nil {
		return nil, nil, err
	}
	opt.setDefaults()

	link, _ := url.Parse(fmt.Sprintf("/orgs/%s/actions/variables", org))
	link.RawQuery = opt.getURLQuery().Encode()

	variables := make([]*ActionVariable, 0, opt.PageSize)
	resp, err := c.getParsedResponse("GET", link.String(), jsonHeader, nil, &variables)
	return variables, resp, err
}

// GetOrgActionVariable gets a specific organization action variable
func (c *Client) GetOrgActionVariable(org, variableName string) (*ActionVariable, *Response, error) {
	if err := escapeValidatePathSegments(&org, &variableName); err != nil {
		return nil, nil, err
	}

	variable := new(ActionVariable)
	resp, err := c.getParsedResponse("GET", fmt.Sprintf("/orgs/%s/actions/variables/%s", org, variableName), jsonHeader, nil, variable)
	return variable, resp, err
}

// CreateOrgActionVariable creates an action variable for an organization
func (c *Client) CreateOrgActionVariable(org string, opt CreateVariableOption) (*Response, error) {
	if err := escapeValidatePathSegments(&org); err != nil {
		return nil, err
	}
	if err := opt.Validate(); err != nil {
		return nil, err
	}

	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, err
	}

	status, resp, err := c.getStatusCode("POST", fmt.Sprintf("/orgs/%s/actions/variables/%s", org, opt.Name), jsonHeader, bytes.NewReader(body))
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

// UpdateOrgActionVariable updates an action variable for an organization
func (c *Client) UpdateOrgActionVariable(org, variableName string, opt CreateVariableOption) (*Response, error) {
	if err := escapeValidatePathSegments(&org, &variableName); err != nil {
		return nil, err
	}
	if err := opt.Validate(); err != nil {
		return nil, err
	}

	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, err
	}

	status, resp, err := c.getStatusCode("PUT", fmt.Sprintf("/orgs/%s/actions/variables/%s", org, variableName), jsonHeader, bytes.NewReader(body))
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

// DeleteOrgActionVariable deletes an action variable from an organization
func (c *Client) DeleteOrgActionVariable(org, variableName string) (*Response, error) {
	if err := escapeValidatePathSegments(&org, &variableName); err != nil {
		return nil, err
	}

	status, resp, err := c.getStatusCode("DELETE", fmt.Sprintf("/orgs/%s/actions/variables/%s", org, variableName), jsonHeader, nil)
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

// GetOrgActionRunnerRegistrationToken gets a runner registration token for an organization
func (c *Client) GetOrgActionRunnerRegistrationToken(org string) (*RunnerRegistrationToken, *Response, error) {
	if err := escapeValidatePathSegments(&org); err != nil {
		return nil, nil, err
	}

	token := new(RunnerRegistrationToken)
	resp, err := c.getParsedResponse("GET", fmt.Sprintf("/orgs/%s/actions/runners/registration-token", org), jsonHeader, nil, token)
	return token, resp, err
}
