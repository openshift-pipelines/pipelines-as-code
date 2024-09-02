package resolve

import (
	"context"
	"fmt"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/matcher"
	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
)

type NamedItem interface {
	GetName() string
}

func alreadyFetchedResource[T NamedItem](resources map[string]T, resourceName string) bool {
	if _, ok := resources[resourceName]; ok {
		return true
	}
	return false
}

// resolveRemoteResources will get remote tasks or Pipelines from annotations.
//
// It already has some tasks or pipeline coming from the tekton directory stored in [types]
//
// The precedence logic for tasks is in this order:
//
// * Tasks from the PipelineRun annotations
// * Tasks from the Pipeline annotations
// * Tasks from the Tekton directory
//
// The precedence logic for Pipeline is first from PipelineRun annotations and
// then from Tekton directory.
func resolveRemoteResources(ctx context.Context, rt *matcher.RemoteTasks, types TektonTypes, ropt *Opts) ([]*tektonv1.PipelineRun, error) {
	fetchedResourcesForEvent := FetchedResources{}
	pipelineRuns := []*tektonv1.PipelineRun{}
	for _, pipelinerun := range types.PipelineRuns {
		fetchedResourcesForPipelineRun := FetchedResourcesForRun{
			Tasks: map[string]*tektonv1.Task{},
		}
		var pipeline *tektonv1.Pipeline
		var err error
		if ropt.RemoteTasks {
			if pipelinerun.GetObjectMeta().GetAnnotations() == nil {
				continue
			}

			if len(pipelinerun.GetObjectMeta().GetAnnotations()) == 0 {
				continue
			}

			// get first all the pipeline from the pipelinerun annotations
			remotePipeline, err := matcher.GrabPipelineFromAnnotations(pipelinerun.GetObjectMeta().GetAnnotations())
			if err != nil {
				return []*tektonv1.PipelineRun{}, fmt.Errorf("error getting remote task from pipelinerun annotations: %w", err)
			}

			if remotePipeline != "" {
				if alreadyFetchedResource(fetchedResourcesForEvent.Pipelines, remotePipeline) {
					rt.Logger.Debugf("skipping already fetched pipeline %s in annotations on pipelinerun %s", remotePipeline, pipelinerun.GetName())
					pipeline = fetchedResourcesForEvent.Pipelines[remotePipeline]
				} else {
					pipeline, err = rt.GetPipelineFromAnnotationName(ctx, remotePipeline)
					if err != nil {
						return []*tektonv1.PipelineRun{}, fmt.Errorf("error getting remote pipeline from pipelinerun annotations: %w", err)
					}
					fetchedResourcesForEvent.Pipelines[remotePipeline] = pipeline
				}
			}
		}

		remotePipelineTasks := []string{}
		if pipelinerun.Spec.PipelineRef != nil && pipelinerun.Spec.PipelineRef.Resolver == "" {
			if pipeline == nil || pipeline.Name != pipelinerun.Spec.PipelineRef.Name {
				pipeline, err = getPipelineByName(pipelinerun.Spec.PipelineRef.Name, types.Pipelines)
				if err != nil {
					return []*tektonv1.PipelineRun{}, err
				}
			}
			fetchedResourcesForPipelineRun.Pipelines = pipeline
			if pipeline.GetObjectMeta().GetAnnotations() != nil {
				// get all the tasks from the pipeline annotations
				remotePipelineTasks, err = matcher.GrabTasksFromAnnotations(pipeline.GetObjectMeta().GetAnnotations())
				if err != nil {
					return []*tektonv1.PipelineRun{}, fmt.Errorf("error getting remote task from pipeline annotations: %w", err)
				}
			}
		}

		if ropt.RemoteTasks {
			// then get all the tasks from the pipelinerun annotations
			remoteTasks, err := matcher.GrabTasksFromAnnotations(pipelinerun.GetObjectMeta().GetAnnotations())
			if err != nil {
				return []*tektonv1.PipelineRun{}, fmt.Errorf("error getting remote task from pipelinerun annotations: %w", err)
			}

			// now resolve all the tasks from pipelinerun and pipeline annotations, giving preference to pipelinerun annotation tasks
			for _, remoteTask := range append(remoteTasks, remotePipelineTasks...) {
				var task *tektonv1.Task
				if alreadyFetchedResource(fetchedResourcesForEvent.Tasks, remoteTask) {
					rt.Logger.Debugf("skipping already fetched task %s in annotations on pipelinerun %s", remoteTask, pipelinerun.GetName())
					task = fetchedResourcesForEvent.Tasks[remoteTask]
				} else {
					task, err = rt.GetTaskFromAnnotationName(ctx, remoteTask)
					if err != nil {
						return []*tektonv1.PipelineRun{}, fmt.Errorf("error getting remote task from pipelinerun annotations: %w", err)
					}
					fetchedResourcesForEvent.Tasks[remoteTask] = task
				}
				if !alreadyFetchedResource(fetchedResourcesForPipelineRun.Tasks, task.GetName()) {
					fetchedResourcesForPipelineRun.Tasks[task.GetName()] = task
				}
			}
		}

		// now add all the tasks in .tekton directory to Tasks, as we add them by default if not found in annotation
		for _, task := range types.Tasks {
			if !alreadyFetchedResource(fetchedResourcesForPipelineRun.Tasks, task.GetName()) {
				fetchedResourcesForPipelineRun.Tasks[task.GetName()] = task
			}
		}

		// Resolve PipelineRef inside PipelineRef
		if pipelinerun.Spec.PipelineRef != nil && pipelinerun.Spec.PipelineRef.Resolver == "" {
			pipeline := fetchedResourcesForPipelineRun.Pipelines
			turns, err := inlineTasks(pipeline.Spec.Tasks, ropt, fetchedResourcesForPipelineRun)
			if err != nil {
				return nil, err
			}
			pipeline.Spec.Tasks = turns

			fruns, err := inlineTasks(pipeline.Spec.Finally, ropt, fetchedResourcesForPipelineRun)
			if err != nil {
				return nil, err
			}
			pipeline.Spec.Finally = fruns

			pipelinerun.Spec.PipelineRef = nil
			pipelinerun.Spec.PipelineSpec = &pipeline.Spec
		}

		// now resolve the PipelineRun by doing inline spec
		// Resolve {Finally/Task}Ref inside PipelineSpec inside PipelineRun
		if pipelinerun.Spec.PipelineSpec != nil {
			turns, err := inlineTasks(pipelinerun.Spec.PipelineSpec.Tasks, ropt, fetchedResourcesForPipelineRun)
			if err != nil {
				return nil, err
			}
			pipelinerun.Spec.PipelineSpec.Tasks = turns

			fruns, err := inlineTasks(pipelinerun.Spec.PipelineSpec.Finally, ropt, fetchedResourcesForPipelineRun)
			if err != nil {
				return nil, err
			}
			pipelinerun.Spec.PipelineSpec.Finally = fruns
		}

		// Add a GenerateName based on the pipeline name and a "-"
		// if we already have a GenerateName then just keep it like this
		if ropt.GenerateName && pipelinerun.GenerateName == "" {
			pipelinerun.GenerateName = pipelinerun.Name + "-"
			pipelinerun.Name = ""
		}
		pipelineRuns = append(pipelineRuns, pipelinerun)
	}
	return pipelineRuns, nil
}
