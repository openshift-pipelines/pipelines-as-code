package status

import (
	"context"
	"regexp"
	"strings"

	"github.com/google/go-github/v47/github"
	pacv1alpha1 "github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/kubeinteraction"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	sortrepostatus "github.com/openshift-pipelines/pipelines-as-code/pkg/sort"
	tektonv1beta1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var reasonMessageReplacementRegexp = regexp.MustCompile(`\(image: .*`)

// snatched from prow
// https://github.com/kubernetes/test-infra/blob/3c8cbed65c421670a7d37239b8ffceb91e0eb16b/prow/spyglass/lenses/buildlog/lens.go#L95
var (
	ErorrRE                                = regexp.MustCompile(`timed out|ERROR:|(FAIL|Failure \[)\b|panic\b|^E\d{4} \d\d:\d\d:\d\d\.\d\d\d]`)
	numLinesOfLogsInContainersToGrabForErr = int64(10)
)

// CollectTaskInfos collects all tasks information we are interested in.
func CollectTaskInfos(ctx context.Context, cs *params.Run, pr tektonv1beta1.PipelineRun) map[string]pacv1alpha1.TaskInfos {
	failureReasons := map[string]pacv1alpha1.TaskInfos{}
	kinteract, _ := kubeinteraction.NewKubernetesInteraction(cs)
	for _, task := range pr.Status.TaskRuns {
		if task.Status != nil && len(task.Status.Conditions) > 0 && task.Status.Conditions[0].Reason == tektonv1beta1.PipelineRunReasonFailed.String() {
			ti := pacv1alpha1.TaskInfos{
				Reason: reasonMessageReplacementRegexp.ReplaceAllString(task.Status.Conditions[0].Message, ""),
			}

			if kinteract != nil {
				for _, step := range task.Status.Steps {
					if step.Terminated != nil && step.Terminated.ExitCode != 0 {
						log, _ := kinteract.GetPodLogs(ctx, pr.GetNamespace(), task.Status.PodName, step.ContainerName, numLinesOfLogsInContainersToGrabForErr)
						// see if a pattern match from errRe
						ti.LogSnippet = strings.TrimSpace(log)
					}
				}
			}
			failureReasons[task.PipelineTaskName] = ti
		}
	}
	return failureReasons
}

// RepositoryRunStatusRemoveSameSHA remove an existing status with the same
// SHA. This would come from repo pipelinerun_status. We don't want the doublons
// and we rather use the ones from the live PR on cluster.
func RepositoryRunStatusRemoveSameSHA(rs []pacv1alpha1.RepositoryRunStatus, livePrSHA string) []pacv1alpha1.RepositoryRunStatus {
	newRepositoryStatus := []pacv1alpha1.RepositoryRunStatus{}
	for _, value := range rs {
		if value.SHA != nil && *value.SHA == livePrSHA {
			continue
		}
		newRepositoryStatus = append(newRepositoryStatus, value)
	}
	return newRepositoryStatus
}

func convertPrStatusToRepositoryStatus(ctx context.Context, cs *params.Run, pr tektonv1beta1.PipelineRun, logurl string) pacv1alpha1.RepositoryRunStatus {
	failurereasons := CollectTaskInfos(ctx, cs, pr)
	prSHA := pr.GetLabels()["pipelinesascode.tekton.dev/sha"]
	return pacv1alpha1.RepositoryRunStatus{
		Status:             pr.Status.Status,
		LogURL:             &logurl,
		PipelineRunName:    pr.GetName(),
		CollectedTaskInfos: &failurereasons,
		StartTime:          pr.Status.StartTime,
		SHA:                github.String(prSHA),
		SHAURL:             github.String(pr.GetAnnotations()["pipelinesascode.tekton.dev/sha-url"]),
		Title:              github.String(pr.GetAnnotations()["pipelinesascode.tekton.dev/sha-title"]),
		TargetBranch:       github.String(pr.GetLabels()["pipelinesascode.tekton.dev/branch"]),
		EventType:          github.String(pr.GetLabels()["pipelinesascode.tekton.dev/event-type"]),
	}
}

func MixLivePRandRepoStatus(ctx context.Context, cs *params.Run, repository pacv1alpha1.Repository) []pacv1alpha1.RepositoryRunStatus {
	repositorystatus := repository.Status
	label := "pipelinesascode.tekton.dev/repository=" + repository.Name
	prs, err := cs.Clients.Tekton.TektonV1beta1().PipelineRuns(repository.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: label,
	})
	if err != nil {
		return sortrepostatus.RepositorySortRunStatus(repositorystatus)
	}

	for _, pr := range prs.Items {
		repositorystatus = RepositoryRunStatusRemoveSameSHA(repositorystatus, pr.GetLabels()["pipelinesascode.tekton.dev/sha"])
		logurl := cs.Clients.ConsoleUI.DetailURL(pr.GetNamespace(), pr.GetName())
		repositorystatus = append(repositorystatus, convertPrStatusToRepositoryStatus(ctx, cs, pr, logurl))
	}
	return sortrepostatus.RepositorySortRunStatus(repositorystatus)
}
