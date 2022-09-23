package kubeinteraction

import (
	"path/filepath"
	"strconv"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode"
	apipac "github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/formatting"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/version"
	tektonv1beta1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
)

const (
	StateWait      = "wait"
	StateStarted   = "started"
	StateQueued    = "queued"
	StateCompleted = "completed"
)

func AddLabelsAndAnnotations(event *info.Event, pipelineRun *tektonv1beta1.PipelineRun, repo *apipac.Repository, providerinfo *info.ProviderConfig) {
	// Add labels on the soon to be created pipelinerun so UI/CLI can easily
	// query them.
	labels := map[string]string{
		"app.kubernetes.io/managed-by":                             pipelinesascode.GroupName,
		"app.kubernetes.io/version":                                version.Version,
		filepath.Join(pipelinesascode.GroupName, "url-org"):        formatting.K8LabelsCleanup(event.Organization),
		filepath.Join(pipelinesascode.GroupName, "url-repository"): formatting.K8LabelsCleanup(event.Repository),
		filepath.Join(pipelinesascode.GroupName, "sha"):            formatting.K8LabelsCleanup(event.SHA),
		filepath.Join(pipelinesascode.GroupName, "sender"):         formatting.K8LabelsCleanup(event.Sender),
		filepath.Join(pipelinesascode.GroupName, "event-type"):     formatting.K8LabelsCleanup(event.EventType),
		filepath.Join(pipelinesascode.GroupName, "branch"):         formatting.K8LabelsCleanup(event.BaseBranch),
		filepath.Join(pipelinesascode.GroupName, "repository"):     formatting.K8LabelsCleanup(repo.GetName()),
		filepath.Join(pipelinesascode.GroupName, "git-provider"):   providerinfo.Name,
		filepath.Join(pipelinesascode.GroupName, "state"):          StateStarted,
	}

	annotations := map[string]string{
		filepath.Join(pipelinesascode.GroupName, "sha-title"): event.SHATitle,
		filepath.Join(pipelinesascode.GroupName, "sha-url"):   event.SHAURL,
		filepath.Join(pipelinesascode.GroupName, "repo-url"):  event.URL,
	}

	if event.PullRequestNumber != 0 {
		annotations[filepath.Join(pipelinesascode.GroupName, "pull-request")] = strconv.Itoa(event.PullRequestNumber)
	}

	// TODO: move to provider specific function
	if providerinfo.Name == "github" || providerinfo.Name == "github-enterprise" {
		if event.InstallationID != -1 {
			annotations[filepath.Join(pipelinesascode.GroupName, "installation-id")] = strconv.FormatInt(event.InstallationID, 10)
		}
		if event.GHEURL != "" {
			annotations[filepath.Join(pipelinesascode.GroupName, "ghe-url")] = event.GHEURL
		}
	}

	// GitLab
	if event.SourceProjectID != 0 {
		annotations[filepath.Join(pipelinesascode.GroupName, "source-project-id")] = strconv.Itoa(event.SourceProjectID)
	}
	if event.TargetProjectID != 0 {
		annotations[filepath.Join(pipelinesascode.GroupName, "target-project-id")] = strconv.Itoa(event.TargetProjectID)
	}

	for k, v := range labels {
		pipelineRun.Labels[k] = v
	}
	for k, v := range annotations {
		pipelineRun.Annotations[k] = v
	}
}
