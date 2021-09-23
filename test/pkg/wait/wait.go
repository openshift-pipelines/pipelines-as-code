package wait

import (
	"context"
	"fmt"
	"time"

	pacversioned "github.com/openshift-pipelines/pipelines-as-code/pkg/generated/clientset/versioned"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/kubeinteraction"
	tektonv1beta1client "github.com/tektoncd/pipeline/pkg/client/clientset/versioned/typed/pipeline/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Opts struct {
	RepoName        string
	Namespace       string
	MinNumberStatus int
	PollTimeout     time.Duration
	AdminNS         string
	TargetSHA       string
}

func UntilRepositoryUpdated(ctx context.Context, pacintf pacversioned.Interface, tknintf tektonv1beta1client.TektonV1beta1Interface, opts Opts) error {
	ctx, cancel := context.WithTimeout(ctx, opts.PollTimeout)
	defer cancel()
	return kubeinteraction.PollImmediateWithContext(ctx, func() (bool, error) {
		pacintf.PipelinesascodeV1alpha1().Repositories(opts.Namespace)
		r, err := pacintf.PipelinesascodeV1alpha1().Repositories(opts.Namespace).Get(ctx, opts.RepoName,
			metav1.GetOptions{})
		if err != nil {
			return true, err
		}

		prs, err := tknintf.PipelineRuns(opts.Namespace).List(ctx, metav1.ListOptions{
			LabelSelector: fmt.Sprintf("pipelinesascode.tekton.dev/sha=%s", opts.TargetSHA),
		})
		if err != nil {
			return true, err
		}
		if len(prs.Items) > 0 {
			prConditions := prs.Items[0].Status.Conditions
			if len(prConditions) != 0 && prConditions[0].Status == corev1.ConditionFalse {
				return true, fmt.Errorf("pipelinerun has failed")
			}
		}

		return len(r.Status) > opts.MinNumberStatus, nil
	})
}
