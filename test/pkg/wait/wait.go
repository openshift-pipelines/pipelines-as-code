package wait

import (
	"context"
	"fmt"
	"time"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/keys"
	pacv1alpha1 "github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/kubeinteraction"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/clients"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Opts struct {
	RepoName            string
	Namespace           string
	MinNumberStatus     int
	PollTimeout         time.Duration
	AdminNS             string
	TargetSHA           string
	FailOnRepoCondition corev1.ConditionStatus
}

func UntilMinPRAppeared(ctx context.Context, clients clients.Clients, opts Opts, minNumber int) error {
	ctx, cancel := context.WithTimeout(ctx, opts.PollTimeout)
	defer cancel()
	return kubeinteraction.PollImmediateWithContext(ctx, opts.PollTimeout, func() (bool, error) {
		prs, err := clients.Tekton.TektonV1().PipelineRuns(opts.Namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return false, err
		}
		if len(prs.Items) >= minNumber {
			return true, nil
		}
		return false, nil
	})
}

func UntilRepositoryUpdated(ctx context.Context, clients clients.Clients, opts Opts) (*pacv1alpha1.Repository, error) {
	ctx, cancel := context.WithTimeout(ctx, opts.PollTimeout)
	defer cancel()
	var repo *pacv1alpha1.Repository
	return repo, kubeinteraction.PollImmediateWithContext(ctx, opts.PollTimeout, func() (bool, error) {
		clients.PipelineAsCode.PipelinesascodeV1alpha1().Repositories(opts.Namespace)
		var err error
		if repo, err = clients.PipelineAsCode.PipelinesascodeV1alpha1().Repositories(opts.Namespace).Get(ctx, opts.RepoName, metav1.GetOptions{}); err != nil {
			return true, err
		}

		prs, err := clients.Tekton.TektonV1().PipelineRuns(opts.Namespace).List(ctx, metav1.ListOptions{
			LabelSelector: fmt.Sprintf("%s=%s", keys.SHA, opts.TargetSHA),
		})
		if err != nil {
			return true, err
		}
		if len(prs.Items) > 0 {
			prConditions := prs.Items[0].Status.Conditions
			if opts.FailOnRepoCondition == "" {
				opts.FailOnRepoCondition = corev1.ConditionFalse
			}

			if len(prConditions) != 0 && prConditions[0].Status == opts.FailOnRepoCondition {
				return true, fmt.Errorf("pipelinerun has failed")
			}
		}

		clients.Log.Infof("Still waiting for repository status to be updated: %d/%d", len(repo.Status), opts.MinNumberStatus)
		time.Sleep(2 * time.Second)
		return len(repo.Status) >= opts.MinNumberStatus, nil
	})
}

func UntilPipelineRunCreated(ctx context.Context, clients clients.Clients, opts Opts) error {
	ctx, cancel := context.WithTimeout(ctx, opts.PollTimeout)
	defer cancel()
	return kubeinteraction.PollImmediateWithContext(ctx, opts.PollTimeout, func() (bool, error) {
		clients.PipelineAsCode.PipelinesascodeV1alpha1().Repositories(opts.Namespace)
		prs, err := clients.Tekton.TektonV1().PipelineRuns(opts.Namespace).List(ctx, metav1.ListOptions{
			LabelSelector: fmt.Sprintf("%s=%s", keys.SHA, opts.TargetSHA),
		})
		if err != nil {
			return true, err
		}

		clients.Log.Info("still waiting for pipelinerun to be created")
		return len(prs.Items) == opts.MinNumberStatus, nil
	})
}
