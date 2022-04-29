package pipelineascode

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/google/go-github/v43/github"
	apipac "github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode"
	pacv1a1 "github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/formatting"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/sort"
	tektonv1beta1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (p *PacRun) updateRepoRunStatus(ctx context.Context, pr *tektonv1beta1.PipelineRun, repo *pacv1a1.Repository) error {
	refsanitized := formatting.SanitizeBranch(p.event.BaseBranch)
	repoStatus := pacv1a1.RepositoryRunStatus{
		Status:          pr.Status.Status,
		PipelineRunName: pr.Name,
		StartTime:       pr.Status.StartTime,
		CompletionTime:  pr.Status.CompletionTime,
		SHA:             &p.event.SHA,
		SHAURL:          &p.event.SHAURL,
		Title:           &p.event.SHATitle,
		LogURL:          github.String(p.run.Clients.ConsoleUI.DetailURL(pr.GetNamespace(), pr.GetName())),
		EventType:       &p.event.EventType,
		TargetBranch:    &refsanitized,
	}

	// Get repo again in case it was updated while we were running the CI
	// we try multiple time until we get right in case of conflicts.
	// that's what the error message tell us anyway, so i guess we listen.
	maxRun := 10
	for i := 0; i < maxRun; i++ {
		lastrepo, err := p.run.Clients.PipelineAsCode.PipelinesascodeV1alpha1().Repositories(
			pr.GetNamespace()).Get(ctx, repo.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}

		// Append pipelinerun status files to the repo status
		if len(lastrepo.Status) >= maxPipelineRunStatusRun {
			copy(lastrepo.Status, lastrepo.Status[len(lastrepo.Status)-maxPipelineRunStatusRun+1:])
			lastrepo.Status = lastrepo.Status[:maxPipelineRunStatusRun-1]
		}

		lastrepo.Status = append(lastrepo.Status, repoStatus)
		nrepo, err := p.run.Clients.PipelineAsCode.PipelinesascodeV1alpha1().Repositories(lastrepo.Namespace).Update(
			ctx, lastrepo, metav1.UpdateOptions{})
		if err != nil {
			p.logger.Infof("Could not update repo %s, retrying %d/%d: %s", lastrepo.Namespace, i, maxRun, err.Error())
			continue
		}
		p.logger.Infof("Repository status of %s has been updated", nrepo.Name)
		return nil
	}

	return fmt.Errorf("cannot update %s", repo.Name)
}

func (p *PacRun) postFinalStatus(ctx context.Context, createdPR *tektonv1beta1.PipelineRun) (*tektonv1beta1.PipelineRun, error) {
	pr, err := p.run.Clients.Tekton.TektonV1beta1().PipelineRuns(createdPR.GetNamespace()).Get(
		ctx, createdPR.GetName(), metav1.GetOptions{},
	)
	if err != nil {
		return pr, err
	}

	taskStatus, err := sort.TaskStatusTmpl(pr, p.run.Clients.ConsoleUI, p.vcx.GetConfig().TaskStatusTMPL)
	if err != nil {
		return pr, err
	}

	status := provider.StatusOpts{
		Status:                  "completed",
		Conclusion:              formatting.PipelineRunStatus(pr),
		Text:                    taskStatus,
		PipelineRunName:         pr.Name,
		DetailsURL:              p.run.Clients.ConsoleUI.DetailURL(pr.GetNamespace(), pr.GetName()),
		OriginalPipelineRunName: pr.GetLabels()[filepath.Join(apipac.GroupName, "original-prname")],
	}

	err = p.vcx.CreateStatus(ctx, p.event, p.run.Info.Pac, status)
	p.logger.Infof("pipelinerun %s has %s", pr.Name, status.Conclusion)
	return pr, err
}
