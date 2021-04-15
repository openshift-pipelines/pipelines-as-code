package pac

import (
	"context"

	apipac "github.com/openshift-pipelines/pipelines-as-code/pkg/api/types/v1apha1"
	pacclient "github.com/openshift-pipelines/pipelines-as-code/pkg/clientset/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Pac struct {
	Client pacclient.RepositoryV1Alpha1Interface
}

func (p Pac) FilterBy(url, branch, event_type string) (apipac.Repository, error) {
	var repository apipac.Repository
	repositories, err := p.Client.Repositories("").List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return repository, err
	}
	for _, value := range repositories.Items {
		if value.Spec.URL == url && value.Spec.Branch == branch && value.Spec.EventType == event_type {
			return value, nil
		}
	}
	return repository, nil
}
