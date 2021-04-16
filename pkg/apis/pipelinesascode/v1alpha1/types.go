package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Repository is the representation of a repo
type Repository struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec RepositorySpec `json:"spec"`
}

// RepositorySpec is the spec of a repo
type RepositorySpec struct {
	Namespace string `json:"namespace"`
	URL       string `json:"url"`
	EventType string `json:"event_type"`
	Branch    string `json:"branch"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RepositoryList is the list of Repositories
type RepositoryList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []Repository `json:"items"`
}
