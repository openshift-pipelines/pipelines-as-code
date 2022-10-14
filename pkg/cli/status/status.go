package status

import (
	"context"

	"github.com/google/go-github/v45/github"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	sortrepostatus "github.com/openshift-pipelines/pipelines-as-code/pkg/sort"
	tektonv1beta1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func convertPrStatusToRepositoryStatus(repositorystatus []v1alpha1.RepositoryRunStatus, pr tektonv1beta1.PipelineRun, logurl string) []v1alpha1.RepositoryRunStatus {
	return append(repositorystatus, v1alpha1.RepositoryRunStatus{
		Status:          pr.Status.Status,
		LogURL:          &logurl,
		PipelineRunName: pr.GetName(),
		StartTime:       pr.Status.StartTime,
		SHA:             github.String(pr.GetLabels()["pipelinesascode.tekton.dev/sha"]),
		SHAURL:          github.String(pr.GetAnnotations()["pipelinesascode.tekton.dev/sha-url"]),
		Title:           github.String(pr.GetAnnotations()["pipelinesascode.tekton.dev/sha-title"]),
		TargetBranch:    github.String(pr.GetLabels()["pipelinesascode.tekton.dev/branch"]),
		EventType:       github.String(pr.GetLabels()["pipelinesascode.tekton.dev/event-type"]),
	})
}

func GetLivePRAndRepostatus(ctx context.Context, cs *params.Run, repository *v1alpha1.Repository) []v1alpha1.RepositoryRunStatus {
	repositorystatus := repository.Status
	label := "pipelinesascode.tekton.dev/repository=" + repository.Name
	prs, err := cs.Clients.Tekton.TektonV1beta1().PipelineRuns(repository.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: label,
	})
	if err != nil {
		return sortrepostatus.RepositorySortRunStatus(repositorystatus)
	}

	for _, pr := range prs.Items {
		logurl := cs.Clients.ConsoleUI.DetailURL(pr.GetNamespace(), pr.GetName())
		if pr.Status.Conditions == nil || len(pr.Status.Conditions) == 0 {
			repositorystatus = convertPrStatusToRepositoryStatus(repositorystatus, pr, logurl)
		} else if pr.Status.Conditions[0].Reason == tektonv1beta1.PipelineRunReasonRunning.String() {
			repositorystatus = convertPrStatusToRepositoryStatus(repositorystatus, pr, logurl)
		}
	}

	return sortrepostatus.RepositorySortRunStatus(repositorystatus)
}
