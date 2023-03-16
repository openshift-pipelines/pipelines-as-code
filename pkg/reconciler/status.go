package reconciler

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/go-github/v49/github"
	apipac "github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/keys"
	pacv1a1 "github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/formatting"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/kubeinteraction"
	kstatus "github.com/openshift-pipelines/pipelines-as-code/pkg/kubeinteraction/status"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/secrets"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/sort"
	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
)

const (
	maxPipelineRunStatusRun = 5
	logSnippetNumLines      = 10
	failureReasonText       = "%s<br><h4>Failure reason</h4><br>%s"
)

var backoffSchedule = []time.Duration{
	1 * time.Second,
	3 * time.Second,
	5 * time.Second,
}

func (r *Reconciler) updateRepoRunStatus(ctx context.Context, logger *zap.SugaredLogger, pr *tektonv1.PipelineRun, repo *pacv1a1.Repository, event *info.Event) error {
	refsanitized := formatting.SanitizeBranch(event.BaseBranch)
	repoStatus := pacv1a1.RepositoryRunStatus{
		Status:          pr.Status.Status,
		PipelineRunName: pr.Name,
		StartTime:       pr.Status.StartTime,
		CompletionTime:  pr.Status.CompletionTime,
		SHA:             &event.SHA,
		SHAURL:          &event.SHAURL,
		Title:           &event.SHATitle,
		LogURL:          github.String(r.run.Clients.ConsoleUI.DetailURL(pr)),
		EventType:       &event.EventType,
		TargetBranch:    &refsanitized,
	}

	// Get repository again in case it was updated while we were running the CI
	// we try multiple time until we get right in case of conflicts.
	// that's what the error message tell us anyway, so i guess we listen.
	maxRun := 10
	for i := 0; i < maxRun; i++ {
		lastrepo, err := r.run.Clients.PipelineAsCode.PipelinesascodeV1alpha1().Repositories(
			pr.GetNamespace()).Get(ctx, repo.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}

		// Append PipelineRun status files to the repo status
		if len(lastrepo.Status) >= maxPipelineRunStatusRun {
			copy(lastrepo.Status, lastrepo.Status[len(lastrepo.Status)-maxPipelineRunStatusRun+1:])
			lastrepo.Status = lastrepo.Status[:maxPipelineRunStatusRun-1]
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

func (r *Reconciler) getFailureSnippet(ctx context.Context, pr *tektonv1.PipelineRun) string {
	intf, err := kubeinteraction.NewKubernetesInteraction(r.run)
	if err != nil {
		return ""
	}
	taskinfos := kstatus.CollectFailedTasksLogSnippet(ctx, r.run, intf, pr, logSnippetNumLines)
	if len(taskinfos) == 0 {
		return ""
	}
	sortedTaskInfos := sort.TaskInfos(taskinfos)
	text := strings.TrimSpace(sortedTaskInfos[0].LogSnippet)
	if text == "" {
		text = sortedTaskInfos[0].Message
	}

	if r.copilot != nil {
		cpt, err := r.copilot.GetResponse(ctx, fmt.Sprintf("analyze this CI Error: %s", text))
		if err != nil {
			r.run.Clients.Log.Errorw("Error while getting copilot response", zap.Error(err))
		}
		return fmt.Sprintf("task <b>%s</b> has the status <b>\"%s\"</b>:\n<pre>%s</pre>ChatGPT explains this error like this\n<pre>%s</pre>", sortedTaskInfos[0].Name, sortedTaskInfos[0].Reason, text, cpt)
	}

	return fmt.Sprintf("task <b>%s</b> has the status <b>\"%s\"</b>:\n<pre>%s</pre>", sortedTaskInfos[0].Name, sortedTaskInfos[0].Reason, text)
}

func (r *Reconciler) postFinalStatus(ctx context.Context, logger *zap.SugaredLogger, vcx provider.Interface, event *info.Event, createdPR *tektonv1.PipelineRun) (*tektonv1.PipelineRun, error) {
	pr, err := r.run.Clients.Tekton.TektonV1().PipelineRuns(createdPR.GetNamespace()).Get(
		ctx, createdPR.GetName(), metav1.GetOptions{},
	)
	if err != nil {
		return pr, err
	}

	trStatus := kstatus.GetStatusFromTaskStatusOrFromAsking(ctx, pr, r.run)
	var taskStatusText string
	if len(trStatus) > 0 {
		var err error
		taskStatusText, err = sort.TaskStatusTmpl(pr, trStatus, r.run, vcx.GetConfig())
		if err != nil {
			return pr, err
		}
	} else {
		taskStatusText = pr.Status.GetCondition(apis.ConditionSucceeded).Message
	}

	if r.run.Info.Pac.ErrorLogSnippet {
		failures := r.getFailureSnippet(ctx, pr)
		if failures != "" {
			secretValues := secrets.GetSecretsAttachedToPipelineRun(ctx, r.kinteract, pr)
			failures = secrets.ReplaceSecretsInText(failures, secretValues)
			taskStatusText = fmt.Sprintf(failureReasonText, taskStatusText, failures)
		}
	}

	status := provider.StatusOpts{
		Status:                  "completed",
		PipelineRun:             pr,
		Conclusion:              formatting.PipelineRunStatus(pr),
		Text:                    taskStatusText,
		PipelineRunName:         pr.Name,
		DetailsURL:              r.run.Clients.ConsoleUI.DetailURL(pr),
		OriginalPipelineRunName: pr.GetLabels()[apipac.OriginalPRName],
	}

	err = createStatusWithRetry(ctx, logger, r.run.Clients.Tekton, vcx, event, r.run.Info.Pac, status)
	logger.Infof("pipelinerun %s has a status of '%s'", pr.Name, status.Conclusion)
	return pr, err
}

func createStatusWithRetry(ctx context.Context, logger *zap.SugaredLogger, tekton versioned.Interface, vcx provider.Interface, event *info.Event, opts *info.PacOpts, status provider.StatusOpts) error {
	var finalError error
	for _, backoff := range backoffSchedule {
		err := vcx.CreateStatus(ctx, tekton, event, opts, status)
		if err == nil {
			return nil
		}
		logger.Infof("failed to create status, error: %v, retrying in %v", err, backoff)
		time.Sleep(backoff)
		finalError = err
	}
	return fmt.Errorf("failed to report status: %w", finalError)
}
