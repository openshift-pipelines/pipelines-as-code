package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

type RepositorySpec struct {
	Namespace string `json:"namespace"`
	URL       string `json:"url"`
	EventType string `json:"event_type"`
	Branch    string `json:"branch"`
}

type Repository struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec RepositorySpec `json:"spec"`
}

type RepositoryList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []Repository `json:"items"`
}
