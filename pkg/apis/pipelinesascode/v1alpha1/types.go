package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Repository is the representation of a repo.
type Repository struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RepositorySpec        `json:"spec"`
	Status []RepositoryRunStatus `json:"pipelinerun_status,omitempty"`
}

type RepositoryRunStatus struct {
	duckv1.Status `json:",inline"`

	// PipelineRunName is the name of the PipelineRun
	// +optional
	PipelineRunName string `json:"pipelineRunName,omitempty"`

	// StartTime is the time the PipelineRun is actually started.
	// +optional
	StartTime *metav1.Time `json:"startTime,omitempty"`

	// CompletionTime is the time the PipelineRun completed.
	// +optional
	CompletionTime *metav1.Time `json:"completionTime,omitempty"`

	// SHA is the name of the SHA that has been tested
	// +optional
	SHA *string `json:"sha,omitempty"`

	// SHA the URL of the SHA to view it
	// +optional
	SHAURL *string `json:"sha_url,omitempty"`

	// Title is the title of the commit SHA that has been tested
	// +optional
	Title *string `json:"title,omitempty"`

	// LogURL is the full url to this run long
	// +optional
	LogURL *string `json:"logurl,omitempty"`

	// TargetBranch is the target branch of that run
	// +optional
	TargetBranch *string `json:"target_branch,omitempty"`

	// EventType is the event type of that run
	// +optional
	EventType *string `json:"event_type,omitempty"`

	// CollectedTaskInfos is the information about tasks
	CollectedTaskInfos *map[string]TaskInfos `json:"failure_reason,omitempty"`
}

type TaskInfos struct {
	Name           string
	Message        string
	LogSnippet     string
	Reason         string
	DisplayName    string
	CompletionTime *metav1.Time
}

// RepositorySpec is the spec of a repo.
type RepositorySpec struct {
	ConcurrencyLimit *int         `json:"concurrency_limit,omitempty"` // move it to settings in further version of the spec
	URL              string       `json:"url"`
	GitProvider      *GitProvider `json:"git_provider,omitempty"`
	Incomings        *[]Incoming  `json:"incoming,omitempty"`
	Params           *[]Params    `json:"params,omitempty"`
	Settings         *Settings    `json:"settings,omitempty"`
}

func (r *RepositorySpec) Merge(newRepo RepositorySpec) {
	if newRepo.ConcurrencyLimit != nil && r.ConcurrencyLimit == nil {
		r.ConcurrencyLimit = newRepo.ConcurrencyLimit
	}
	if newRepo.Settings != nil {
		r.Settings.Merge(newRepo.Settings)
	}
	if r.GitProvider != nil && newRepo.GitProvider != nil {
		r.GitProvider.Merge(newRepo.GitProvider)
	}

	// TODO(chmouel): maybe let it merges those between the user Repo Incomings and Params with the global ones?
	// we need to gather feedback first with users to know what they want.
	if newRepo.Incomings != nil && r.Incomings == nil {
		r.Incomings = newRepo.Incomings
	}
	if newRepo.Params != nil && r.Params == nil {
		r.Params = newRepo.Params
	}
}

type Settings struct {
	GithubAppTokenScopeRepos []string `json:"github_app_token_scope_repos,omitempty"`
	PipelineRunProvenance    string   `json:"pipelinerun_provenance,omitempty"`
	Policy                   *Policy  `json:"policy,omitempty"`
}

func (s *Settings) Merge(newSettings *Settings) {
	if newSettings.PipelineRunProvenance != "" && s.PipelineRunProvenance == "" {
		s.PipelineRunProvenance = newSettings.PipelineRunProvenance
	}
	if newSettings.Policy != nil && s.Policy == nil {
		s.Policy = newSettings.Policy
	}
	if newSettings.GithubAppTokenScopeRepos != nil && s.GithubAppTokenScopeRepos == nil {
		s.GithubAppTokenScopeRepos = newSettings.GithubAppTokenScopeRepos
	}
}

type Policy struct {
	OkToTest    []string `json:"ok_to_test,omitempty"`
	PullRequest []string `json:"pull_request,omitempty"`
}

type Params struct {
	Name      string  `json:"name"`
	Value     string  `json:"value,omitempty"`
	SecretRef *Secret `json:"secret_ref,omitempty"`
	Filter    string  `json:"filter,omitempty"`
}

type Incoming struct {
	Type    string   `json:"type"`
	Secret  Secret   `json:"secret"`
	Params  []string `json:"params,omitempty"`
	Targets []string `json:"targets,omitempty"`
}

type GitProvider struct {
	URL           string  `json:"url,omitempty"`
	User          string  `json:"user,omitempty"`
	Secret        *Secret `json:"secret,omitempty"`
	WebhookSecret *Secret `json:"webhook_secret,omitempty"`
	Type          string  `json:"type,omitempty"`
}

func (g *GitProvider) Merge(newGitProvider *GitProvider) {
	// only merge of the same type
	if newGitProvider.Type != "" && g.Type != "" && g.Type != newGitProvider.Type {
		return
	}
	if newGitProvider.URL != "" && g.URL == "" {
		g.URL = newGitProvider.URL
	}
	if newGitProvider.User != "" && g.User == "" {
		g.User = newGitProvider.User
	}
	if newGitProvider.Type != "" && g.Type == "" {
		g.Type = newGitProvider.Type
	}
	if newGitProvider.Secret != nil && g.Secret == nil {
		g.Secret = newGitProvider.Secret
	}
	if newGitProvider.WebhookSecret != nil && g.WebhookSecret == nil {
		g.WebhookSecret = newGitProvider.WebhookSecret
	}
}

type Secret struct {
	Name string `json:"name"`
	Key  string `json:"key"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RepositoryList is the list of Repositories.
type RepositoryList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []Repository `json:"items"`
}
