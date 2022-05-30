package webhook

import (
	"context"
	"fmt"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/generated/clientset/versioned"
	v1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"knative.dev/pkg/webhook"
)

var universalDeserializer = serializer.NewCodecFactory(runtime.NewScheme()).UniversalDeserializer()

// Path implements AdmissionController
func (ac *reconciler) Path() string {
	return ac.path
}

// Admit implements AdmissionController
func (ac *reconciler) Admit(ctx context.Context, request *v1.AdmissionRequest) *v1.AdmissionResponse {
	raw := request.Object.Raw
	repo := v1alpha1.Repository{}
	if _, _, err := universalDeserializer.Decode(raw, nil, &repo); err != nil {
		return webhook.MakeErrorStatus("validation failed: %v", err)
	}

	exist, err := checkIfRepoExist(ctx, ac.pacClient, &repo, "")
	if err != nil {
		return webhook.MakeErrorStatus("validation failed: %v", err)
	}

	if exist {
		return webhook.MakeErrorStatus(fmt.Sprintf("repository already exist with url: %s", repo.Spec.URL))
	}

	return &v1.AdmissionResponse{Allowed: true}
}

func checkIfRepoExist(ctx context.Context, pac versioned.Interface, repo *v1alpha1.Repository, ns string) (bool, error) {
	repositories, err := pac.PipelinesascodeV1alpha1().Repositories(ns).List(
		ctx, metav1.ListOptions{})
	if err != nil {
		return false, err
	}
	for i := len(repositories.Items) - 1; i >= 0; i-- {
		repoFromCluster := repositories.Items[i]
		if repoFromCluster.Spec.URL == repo.Spec.URL &&
			(repoFromCluster.Name != repo.Name || repoFromCluster.Namespace != repo.Namespace) {
			return true, nil
		}
	}
	return false, nil
}
