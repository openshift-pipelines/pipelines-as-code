package reconciler

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/google/go-github/v45/github"
	apipac "github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode"
	pacv1a1 "github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/formatting"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/sort"
	tektonv1beta1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	maxPipelineRunStatusRun = 5
)

func (r *Reconciler) updateRepoRunStatus(ctx context.Context, logger *zap.SugaredLogger, pr *tektonv1beta1.PipelineRun, repo *pacv1a1.Repository, event *info.Event) error {
	refsanitized := formatting.SanitizeBranch(event.BaseBranch)
	repoStatus := pacv1a1.RepositoryRunStatus{
		Status:          pr.Status.Status,
		PipelineRunName: pr.Name,
		StartTime:       pr.Status.StartTime,
		CompletionTime:  pr.Status.CompletionTime,
		SHA:             &event.SHA,
		SHAURL:          &event.SHAURL,
		Title:           &event.SHATitle,
		LogURL:          github.String(r.run.Clients.ConsoleUI.DetailURL(pr.GetNamespace(), pr.GetName())),
		EventType:       &event.EventType,
		TargetBranch:    &refsanitized,
	}

	// Get repo again in case it was updated while we were running the CI
	// we try multiple time until we get right in case of conflicts.
	// that's what the error message tell us anyway, so i guess we listen.
	maxRun := 10
	for i := 0; i < maxRun; i++ {
		lastrepo, err := r.run.Clients.PipelineAsCode.PipelinesascodeV1alpha1().Repositories(
			pr.GetNamespace()).Get(ctx, repo.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}

		// Append pipelinerun status files to the repo status
		if len(lastrepo.Status) >= maxPipelineRunStatusRun {
			copy(lastrepo.Status, lastrepo.Status[len(lastrepo.Status)-maxPipelineRunStatusRun+1:])
			lastrepo.Status = lastrepo.Status[:maxPipelineRunStatusRun-1]
		}
		if r.isPipelinerunNameAlreadyExistInRepoStatus(lastrepo.Status, repoStatus.PipelineRunName) {
			lastrepo.Status = removePipelinerunNameFromLastRepo(lastrepo.Status, repoStatus.PipelineRunName)
		}

		lastrepo.Status = append(lastrepo.Status, repoStatus)

		nrepo, err := r.run.Clients.PipelineAsCode.PipelinesascodeV1alpha1().Repositories(lastrepo.Namespace).Update(
			ctx, lastrepo, metav1.UpdateOptions{})
		if err != nil {
			logger.Infof("Could not update repo %s, retrying %d/%d: %s", lastrepo.Namespace, i, maxRun, err.Error())
			continue
		}
		logger.Infof("Repository status of %s has been updated", nrepo.Name)
		return nil
	}

	return fmt.Errorf("cannot update %s", repo.Name)
}

// isPipelinerunNameAlreadyExistInStatus checks if a pipelinerun is present in a last repository status
func (r *Reconciler) isPipelinerunNameAlreadyExistInRepoStatus(s []pacv1a1.RepositoryRunStatus, str string) bool {
	for _, v := range s {
		if v.PipelineRunName == str {
			return true
		}
	}
	return false
}

// removePipelinerunNameFromLastRepo will remove duplicate pipelinerun from repo status
func removePipelinerunNameFromLastRepo(s []pacv1a1.RepositoryRunStatus, str string) []pacv1a1.RepositoryRunStatus {
	for j, v := range s {
		if v.PipelineRunName == str {
			s = append(s[:j], s[j+1:]...)
		}
	}
	return s
}

func (r *Reconciler) postFinalStatus(ctx context.Context, logger *zap.SugaredLogger, vcx provider.Interface, event *info.Event, createdPR *tektonv1beta1.PipelineRun) (*tektonv1beta1.PipelineRun, error) {
	pr, err := r.run.Clients.Tekton.TektonV1beta1().PipelineRuns(createdPR.GetNamespace()).Get(
		ctx, createdPR.GetName(), metav1.GetOptions{},
	)
	if err != nil {
		return pr, err
	}

	taskStatus, err := sort.TaskStatusTmpl(pr, r.run.Clients.ConsoleUI, vcx.GetConfig().TaskStatusTMPL)
	if err != nil {
		return pr, err
	}

	status := provider.StatusOpts{
		Status:                  "completed",
		PipelineRun:             pr,
		Conclusion:              formatting.PipelineRunStatus(pr),
		Text:                    taskStatus,
		PipelineRunName:         pr.Name,
		DetailsURL:              r.run.Clients.ConsoleUI.DetailURL(pr.GetNamespace(), pr.GetName()),
		OriginalPipelineRunName: pr.GetLabels()[filepath.Join(apipac.GroupName, "original-prname")],
	}

	err = vcx.CreateStatus(ctx, r.run.Clients.Tekton, event, r.run.Info.Pac, status)
	logger.Infof("pipelinerun %s has a status of '%s'", pr.Name, status.Conclusion)
	return pr, err
}
