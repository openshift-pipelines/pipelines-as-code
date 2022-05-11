package resolve

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	apipac "github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/matcher"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider"
	tektonv1beta1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"go.uber.org/zap"
	k8scheme "k8s.io/client-go/kubernetes/scheme"
)

type Types struct {
	PipelineRuns []*tektonv1beta1.PipelineRun
	Pipelines    []*tektonv1beta1.Pipeline
	TaskRuns     []*tektonv1beta1.TaskRun
	Tasks        []*tektonv1beta1.Task
}

func readTypes(log *zap.SugaredLogger, data string) Types {
	types := Types{}
	decoder := k8scheme.Codecs.UniversalDeserializer()

	for _, doc := range strings.Split(strings.Trim(data, "-"), "---") {
		if strings.TrimSpace(doc) == "" {
			continue
		}

		obj, _, err := decoder.Decode([]byte(doc), nil, nil)
		if err != nil {
			log.Infof("Skipping document not looking like a kubernetes resources: %v", err)
			continue
		}
		switch o := obj.(type) {
		case *tektonv1beta1.Pipeline:
			types.Pipelines = append(types.Pipelines, o)
		case *tektonv1beta1.PipelineRun:
			types.PipelineRuns = append(types.PipelineRuns, o)
		case *tektonv1beta1.Task:
			types.Tasks = append(types.Tasks, o)
		default:
			log.Info("Skipping document not looking like a tekton resource we can Resolve.")
		}
	}

	return types
}

func getTaskByName(name string, tasks []*tektonv1beta1.Task) (*tektonv1beta1.Task, error) {
	for _, value := range tasks {
		if value.Name == name {
			return value, nil
		}
	}
	return &tektonv1beta1.Task{}, fmt.Errorf("cannot find task %s in input", name)
}

func getPipelineByName(name string, tasks []*tektonv1beta1.Pipeline) (*tektonv1beta1.Pipeline, error) {
	for _, value := range tasks {
		if value.Name == name {
			return value, nil
		}
	}
	return &tektonv1beta1.Pipeline{}, fmt.Errorf("cannot find pipeline %s in input", name)
}

func skippingTask(taskName string, skippedTasks []string) bool {
	for _, value := range skippedTasks {
		if value == taskName {
			return true
		}
	}
	return false
}

func isTektonAPIVersion(apiVersion string) bool {
	return strings.HasPrefix(apiVersion, "tekton.dev/") || apiVersion == ""
}

func inlineTasks(tasks []tektonv1beta1.PipelineTask, ropt *Opts, types Types) ([]tektonv1beta1.PipelineTask, error) {
	pipelineTasks := []tektonv1beta1.PipelineTask{}
	for _, task := range tasks {
		if task.TaskRef != nil && task.TaskRef.Bundle == "" &&
			isTektonAPIVersion(task.TaskRef.APIVersion) &&
			string(task.TaskRef.Kind) != "ClusterTask" &&
			!skippingTask(task.TaskRef.Name, ropt.SkipInlining) {
			taskResolved, err := getTaskByName(task.TaskRef.Name, types.Tasks)
			if err != nil {
				return nil, err
			}
			task.TaskRef = nil
			task.TaskSpec = &tektonv1beta1.EmbeddedTask{TaskSpec: taskResolved.Spec}
		}
		pipelineTasks = append(pipelineTasks, task)
	}
	return pipelineTasks, nil
}

type Opts struct {
	GenerateName bool     // whether to GenerateName
	RemoteTasks  bool     // whether to parse annotation to fetch tasks from remote
	SkipInlining []string // task to skip inlining
}

// Resolve gets a large string which is a yaml multi documents containing
// Pipeline/PipelineRuns/Tasks and resolve them inline as a single PipelineRun
// generateName can be set as True to set the name as a generateName + "-" for
// unique pipelinerun
func Resolve(ctx context.Context, cs *params.Run, logger *zap.SugaredLogger, providerintf provider.Interface, event *info.Event, data string, ropt *Opts) ([]*tektonv1beta1.PipelineRun, error) {
	types := readTypes(logger, data)
	if len(types.PipelineRuns) == 0 {
		return []*tektonv1beta1.PipelineRun{}, errors.New("we need at least one pipelinerun to start with")
	}

	// First resolve Annotations Tasks
	for _, pipelinerun := range types.PipelineRuns {
		if ropt.RemoteTasks && pipelinerun.GetObjectMeta().GetAnnotations() != nil {
			rt := matcher.RemoteTasks{
				Run: cs,
			}
			remoteTasks, err := rt.GetTaskFromAnnotations(ctx, logger, providerintf, event, pipelinerun.GetObjectMeta().GetAnnotations())
			if err != nil {
				return []*tektonv1beta1.PipelineRun{}, err
			}
			// Merge remote tasks with local tasks
			types.Tasks = append(types.Tasks, remoteTasks...)
		}
	}

	// Resolve {Finally/Task}Ref inside Pipeline
	for _, pipeline := range types.Pipelines {
		pipelineTasks, err := inlineTasks(pipeline.Spec.Tasks, ropt, types)
		if err != nil {
			return nil, err
		}
		pipeline.Spec.Tasks = pipelineTasks

		finallyTasks, err := inlineTasks(pipeline.Spec.Finally, ropt, types)
		if err != nil {
			return nil, err
		}
		pipeline.Spec.Finally = finallyTasks
	}

	for _, pipelinerun := range types.PipelineRuns {
		// Resolve {Finally/Task}Ref inside PipelineSpec inside PipelineRun
		if pipelinerun.Spec.PipelineSpec != nil {
			truns, err := inlineTasks(pipelinerun.Spec.PipelineSpec.Tasks, ropt, types)
			if err != nil {
				return nil, err
			}
			pipelinerun.Spec.PipelineSpec.Tasks = truns

			fruns, err := inlineTasks(pipelinerun.Spec.PipelineSpec.Finally, ropt, types)
			if err != nil {
				return nil, err
			}
			pipelinerun.Spec.PipelineSpec.Finally = fruns
		}

		// Resolve PipelineRef inside PipelineRef
		if pipelinerun.Spec.PipelineRef != nil && pipelinerun.Spec.PipelineRef.Bundle == "" {
			pipelineResolved, err := getPipelineByName(pipelinerun.Spec.PipelineRef.Name, types.Pipelines)
			if err != nil {
				return []*tektonv1beta1.PipelineRun{}, err
			}
			pipelinerun.Spec.PipelineRef = nil
			pipelinerun.Spec.PipelineSpec = &pipelineResolved.Spec
		}

		var originPipelinerunName string

		originPipelinerunName = pipelinerun.ObjectMeta.Name
		// Add a GenerateName based on the pipeline name and a "-"
		// if we already have a GenerateName then just keep it like this
		if ropt.GenerateName && pipelinerun.ObjectMeta.GenerateName == "" {
			pipelinerun.ObjectMeta.GenerateName = pipelinerun.ObjectMeta.Name + "-"
			pipelinerun.ObjectMeta.Name = ""
		} else if originPipelinerunName == "" && pipelinerun.ObjectMeta.GenerateName != "" {
			originPipelinerunName = pipelinerun.ObjectMeta.GenerateName
		}

		// make sure we keep the originalPipelineRun in a label
		// because we would need it later on when grouping by cleanups and we
		// can attach that pr file from .tekton directory.

		// Don't overwrite the labels if there is some who already exist set by the user in repo
		if pipelinerun.ObjectMeta.Labels == nil {
			pipelinerun.ObjectMeta.Labels = map[string]string{}
		}
		pipelinerun.ObjectMeta.Labels[filepath.Join(apipac.GroupName, "original-prname")] = originPipelinerunName
	}
	return types.PipelineRuns, nil
}

// nolint:gochecknoinits
func init() {
	_ = tektonv1beta1.AddToScheme(k8scheme.Scheme)
}
