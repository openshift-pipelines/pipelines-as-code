package pipelineascode

import (
	apipac "github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/formatting"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	tektonv1beta1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
)

func addLabelsAndAnnotations(cs *params.Run, pipelineRun *tektonv1beta1.PipelineRun, repo *apipac.Repository) {
	// Add labels on the soon to be created pipelinerun so UI/CLI can easily
	// query them.
	pipelineRun.Labels = map[string]string{
		"app.kubernetes.io/managed-by":              "pipelines-as-code",
		"pipelinesascode.tekton.dev/url-org":        formatting.K8LabelsCleanup(cs.Info.Event.Owner),
		"pipelinesascode.tekton.dev/url-repository": formatting.K8LabelsCleanup(cs.Info.Event.Repository),
		"pipelinesascode.tekton.dev/sha":            formatting.K8LabelsCleanup(cs.Info.Event.SHA),
		"pipelinesascode.tekton.dev/sender":         formatting.K8LabelsCleanup(cs.Info.Event.Sender),
		"pipelinesascode.tekton.dev/event-type":     formatting.K8LabelsCleanup(cs.Info.Event.EventType),
		"pipelinesascode.tekton.dev/branch":         formatting.K8LabelsCleanup(cs.Info.Event.BaseBranch),
		"pipelinesascode.tekton.dev/repository":     formatting.K8LabelsCleanup(repo.GetName()),
	}

	pipelineRun.Annotations["pipelinesascode.tekton.dev/sha-title"] = cs.Info.Event.SHATitle
	pipelineRun.Annotations["pipelinesascode.tekton.dev/sha-url"] = cs.Info.Event.SHAURL
}
