package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	duckv1beta1 "knative.dev/pkg/apis/duck/v1beta1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Repository is the representation of a repo
type Repository struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RepositorySpec        `json:"spec"`
	Status []RepositoryRunStatus `json:"pipelinerun_status,omitempty"`
}

type RepositoryRunStatus struct {
	duckv1beta1.Status `json:",inline"`

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
}

// RepositorySpec is the spec of a repo
type RepositorySpec struct {
	URL          string            `json:"url"`
	EventType    string            `json:"event_type"`
	Branch       string            `json:"branch"`
	WebvcsAPIURL string            `json:"webvcs_api_url"`
	WebvcsSecret *WebvcsSecretSpec `json:"webvcs_secret,omitempty"`
}

type WebvcsSecretSpec struct {
	Name string `json:"name"`
	Key  string `json:"key"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RepositoryList is the list of Repositories
type RepositoryList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []Repository `json:"items"`
}
