package kubeinteraction

import (
	"strconv"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/keys"
	apipac "github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/formatting"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/version"
	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
)

const (
	StateStarted   = "started"
	StateQueued    = "queued"
	StateCompleted = "completed"
	StateFailed    = "failed"
)

func AddLabelsAndAnnotations(event *info.Event, pipelineRun *tektonv1.PipelineRun, repo *apipac.Repository, providerinfo *info.ProviderConfig) {
	// Add labels on the soon to be created pipelinerun so UI/CLI can easily
	// query them.
	labels := map[string]string{
		"app.kubernetes.io/managed-by": pipelinesascode.GroupName,
		"app.kubernetes.io/version":    version.Version,
		keys.URLOrg:                    formatting.K8LabelsCleanup(event.Organization),
		keys.URLRepository:             formatting.K8LabelsCleanup(event.Repository),
		keys.SHA:                       formatting.K8LabelsCleanup(event.SHA),
		keys.Sender:                    formatting.K8LabelsCleanup(event.Sender),
		keys.EventType:                 formatting.K8LabelsCleanup(event.EventType),
		keys.Branch:                    formatting.K8LabelsCleanup(event.BaseBranch),
		keys.Repository:                formatting.K8LabelsCleanup(repo.GetName()),
		keys.GitProvider:               providerinfo.Name,
		keys.State:                     StateStarted,
	}

	annotations := map[string]string{
		keys.ShaTitle: event.SHATitle,
		keys.ShaURL:   event.SHAURL,
		keys.RepoURL:  event.URL,
	}

	if event.PullRequestNumber != 0 {
		labels[keys.PullRequest] = strconv.Itoa(event.PullRequestNumber)
	}

	// TODO: move to provider specific function
	if providerinfo.Name == "github" || providerinfo.Name == "github-enterprise" {
		if event.InstallationID != -1 {
			annotations[keys.InstallationID] = strconv.FormatInt(event.InstallationID, 10)
		}
		if event.GHEURL != "" {
			annotations[keys.GHEURL] = event.GHEURL
		}
	}

	// GitLab
	if event.SourceProjectID != 0 {
		annotations[keys.SourceProjectID] = strconv.Itoa(event.SourceProjectID)
	}
	if event.TargetProjectID != 0 {
		annotations[keys.TargetProjectID] = strconv.Itoa(event.TargetProjectID)
	}

	for k, v := range labels {
		pipelineRun.Labels[k] = v
	}
	for k, v := range annotations {
		pipelineRun.Annotations[k] = v
	}
}
