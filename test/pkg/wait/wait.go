package wait

import (
	"context"
	"time"

	pacversioned "github.com/openshift-pipelines/pipelines-as-code/pkg/generated/clientset/versioned"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/kubeinteraction"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func UntilRepositoryUpdated(ctx context.Context,
	pacintf pacversioned.Interface, repo, ns string,
	minNumberStatus int, polltimeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, polltimeout)
	defer cancel()
	return kubeinteraction.PollImmediateWithContext(ctx, func() (bool, error) {
		pacintf.PipelinesascodeV1alpha1().Repositories(ns)
		r, err := pacintf.PipelinesascodeV1alpha1().Repositories(ns).Get(ctx, repo, metav1.GetOptions{})
		if err != nil {
			return true, err
		}
		return len(r.Status) > minNumberStatus, nil
	})
}
