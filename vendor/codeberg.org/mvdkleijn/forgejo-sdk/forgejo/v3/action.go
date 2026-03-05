// Copyright 2024 The Forgejo Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package forgejo

import (
	"fmt"
	"time"
)

// ActionRun represents a workflow run
type ActionRun struct {
	ID                int64       `json:"id"`
	RunNumber         int64       `json:"index_in_repo"`
	WorkflowID        string      `json:"workflow_id"`
	Title             string      `json:"title"`
	Status            string      `json:"status"`
	Event             string      `json:"event"`
	CommitSHA         string      `json:"commit_sha"`
	PrettyRef         string      `json:"prettyref"`
	HTMLURL           string      `json:"html_url"`
	TriggerUser       *User       `json:"trigger_user"`
	Repository        *Repository `json:"repository"`
	Created           time.Time   `json:"created"`
	Started           time.Time   `json:"started"`
	Stopped           time.Time   `json:"stopped"`
	Updated           time.Time   `json:"updated"`
	NeedApproval      bool        `json:"need_approval"`
	ApprovedBy        int64       `json:"approved_by"`
	IsForkPullRequest bool        `json:"is_fork_pull_request"`
	IsRefDeleted      bool        `json:"is_ref_deleted"`
}

// ActionTask represents a task within a workflow run
type ActionTask struct {
	ID           int64     `json:"id"`
	Name         string    `json:"name"`
	WorkflowID   string    `json:"workflow_id"`
	Status       string    `json:"status"`
	Event        string    `json:"event"`
	DisplayTitle string    `json:"display_title"`
	HeadBranch   string    `json:"head_branch"`
	HeadSHA      string    `json:"head_sha"`
	RunNumber    int64     `json:"run_number"`
	URL          string    `json:"url"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	RunStartedAt time.Time `json:"run_started_at"`
}

// ActionVariable represents an action variable
type ActionVariable struct {
	Name    string `json:"name"`
	Data    string `json:"data"`
	OwnerID int64  `json:"owner_id"`
	RepoID  int64  `json:"repo_id"`
}

// ActionRunJob represents a job within a workflow run
type ActionRunJob struct {
	ID      int64    `json:"id"`
	Name    string   `json:"name"`
	Status  string   `json:"status"`
	TaskID  int64    `json:"task_id"`
	OwnerID int64    `json:"owner_id"`
	RepoID  int64    `json:"repo_id"`
	RunsOn  []string `json:"runs_on"`
	Needs   []string `json:"needs"`
}

// RunnerRegistrationToken represents a runner registration token
type RunnerRegistrationToken struct {
	Token string `json:"token"`
}

// ListActionRunsOption options for listing action runs
type ListActionRunsOption struct {
	ListOptions
	Event     string `json:"event"`
	Status    string `json:"status"`
	RunNumber int64  `json:"run_number"`
	HeadSHA   string `json:"head_sha"`
}

// QueryEncode encodes options to query parameters
func (opt *ListActionRunsOption) QueryEncode() string {
	query := opt.getURLQuery()
	if opt.Event != "" {
		query.Add("event", opt.Event)
	}
	if opt.Status != "" {
		query.Add("status", opt.Status)
	}
	if opt.RunNumber > 0 {
		query.Add("run_number", fmt.Sprintf("%d", opt.RunNumber))
	}
	if opt.HeadSHA != "" {
		query.Add("head_sha", opt.HeadSHA)
	}
	return query.Encode()
}

// ListActionRunsResponse paginated list of action runs
type ListActionRunsResponse struct {
	TotalCount   int64        `json:"total_count"`
	WorkflowRuns []*ActionRun `json:"workflow_runs"`
}

// ListActionTasksOption options for listing action tasks
type ListActionTasksOption struct {
	ListOptions
}

// ListActionTasksResponse paginated list of action tasks
type ListActionTasksResponse struct {
	TotalCount   int64         `json:"total_count"`
	WorkflowRuns []*ActionTask `json:"workflow_runs"`
}

// ListActionJobsOption options for listing/searching action jobs
type ListActionJobsOption struct {
	Labels string `json:"labels"`
}

// DispatchWorkflowOption options for triggering a workflow
type DispatchWorkflowOption struct {
	Ref           string            `json:"ref"`
	Inputs        map[string]string `json:"inputs,omitempty"`
	ReturnRunInfo bool              `json:"return_run_info,omitempty"`
}

// DispatchWorkflowResponse response from dispatching a workflow
type DispatchWorkflowResponse struct {
	ID        int64           `json:"id"`
	RunNumber int64           `json:"run_number"`
	Jobs      []*ActionRunJob `json:"jobs"`
}

// CreateVariableOption options for creating/updating a variable
type CreateVariableOption struct {
	Name string `json:"name"`
	Data string `json:"value"`
}

// Validate checks if the CreateVariableOption is valid.
func (opt *CreateVariableOption) Validate() error {
	if len(opt.Name) == 0 {
		return fmt.Errorf("name required")
	}
	if len(opt.Data) == 0 {
		return fmt.Errorf("data required")
	}
	return nil
}
