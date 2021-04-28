package resolve

import (
	"errors"
	"fmt"
	"strings"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	tektonv1beta1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"k8s.io/client-go/kubernetes/scheme"
	k8scheme "k8s.io/client-go/kubernetes/scheme"
)

type Types struct {
	PipelineRuns []*v1beta1.PipelineRun
	Pipelines    []*v1beta1.Pipeline
	TaskRuns     []*v1beta1.TaskRun
	Tasks        []*v1beta1.Task
}

func readTypes(cs *cli.Clients, data []byte) (Types, error) {
	var types = Types{}
	decoder := scheme.Codecs.UniversalDeserializer()

	for _, doc := range strings.Split(strings.Trim(string(data), "-"), "---") {
		if strings.TrimSpace(doc) == "" {
			continue
		}

		obj, _, err := decoder.Decode([]byte(doc), nil, nil)
		if err != nil {
			cs.Log.Info("Skipping document not looking like a kubernetes resources")
			continue
		}
		switch o := obj.(type) {
		case *v1beta1.Pipeline:
			types.Pipelines = append(types.Pipelines, o)
		case *v1beta1.PipelineRun:
			types.PipelineRuns = append(types.PipelineRuns, o)
		case *v1beta1.Task:
			types.Tasks = append(types.Tasks, o)
		default:
			cs.Log.Info("Skipping document not looking like a tekton resource we can Resolve.")
		}
	}

	return types, nil
}

func getTaskByName(name string, tasks []*v1beta1.Task) (*v1beta1.Task, error) {
	for _, value := range tasks {
		if value.Name == name {
			return value, nil
		}
	}
	return &v1beta1.Task{}, fmt.Errorf("Cannot find task %s in input", name)
}

func getPipelineByName(name string, tasks []*v1beta1.Pipeline) (*v1beta1.Pipeline, error) {
	for _, value := range tasks {
		if value.Name == name {
			return value, nil
		}
	}
	return &v1beta1.Pipeline{}, fmt.Errorf("Cannot find pipeline %s in input", name)
}

func resolve(cs *cli.Clients, data []byte, generateName bool) ([]*v1beta1.PipelineRun, error) {
	types, err := readTypes(cs, data)
	if err != nil {
		return []*v1beta1.PipelineRun{}, err
	}
	if len(types.PipelineRuns) == 0 {
		return []*v1beta1.PipelineRun{}, errors.New("We need at least one pipelinerun to start with")
	}

	// Resolve TaskRef inside Pipeline
	for _, pipeline := range types.Pipelines {
		pipelineTasks := []v1beta1.PipelineTask{}
		for _, task := range pipeline.Spec.Tasks {
			if task.TaskRef != nil {
				taskResolved, err := getTaskByName(task.TaskRef.Name, types.Tasks)
				if err != nil {
					return []*v1beta1.PipelineRun{}, err
				}
				task.TaskRef = nil
				task.TaskSpec = &v1beta1.EmbeddedTask{TaskSpec: taskResolved.Spec}
			}
			pipelineTasks = append(pipelineTasks, task)
		}
		pipeline.Spec.Tasks = pipelineTasks
	}

	for _, pipelinerun := range types.PipelineRuns {
		// Resolve taskRef inside PipelineSpec inside PipelineRun
		if pipelinerun.Spec.PipelineSpec != nil {
			pipelineTasksResolve := []v1beta1.PipelineTask{}
			for _, task := range pipelinerun.Spec.PipelineSpec.Tasks {
				if task.TaskRef != nil {
					taskResolved, err := getTaskByName(task.TaskRef.Name, types.Tasks)
					if err != nil {
						return []*v1beta1.PipelineRun{}, err
					}
					task.TaskRef = nil
					task.TaskSpec = &v1beta1.EmbeddedTask{TaskSpec: taskResolved.Spec}
				}
				pipelineTasksResolve = append(pipelineTasksResolve, task)
			}
			pipelinerun.Spec.PipelineSpec.Tasks = pipelineTasksResolve

		}
		// Resolve PipelineRef inside PipelineRef
		if pipelinerun.Spec.PipelineRef != nil {
			pipelineResolved, err := getPipelineByName(pipelinerun.Spec.PipelineRef.Name, types.Pipelines)
			if err != nil {
				return []*v1beta1.PipelineRun{}, err
			}
			pipelinerun.Spec.PipelineRef = nil
			pipelinerun.Spec.PipelineSpec = &pipelineResolved.Spec

		}
		// Add a generateName based on name if we want it
		if generateName && pipelinerun.ObjectMeta.GenerateName == "" {
			pipelinerun.ObjectMeta.GenerateName = pipelinerun.ObjectMeta.Name + "-"
			pipelinerun.ObjectMeta.Name = ""
		}
	}
	return types.PipelineRuns, nil
}

// Resolve gets a large string which is a yaml multi documents containing
// Pipeline/PipelineRuns/Tasks and resolve them inline as a single PipelineRun
// generateName can be set as True to set the name as a generateName + "-" for
// unique pipelinerun
func Resolve(cs *cli.Clients, allTemplates string, generateName bool) ([]*v1beta1.PipelineRun, error) {
	s := k8scheme.Scheme
	if err := tektonv1beta1.AddToScheme(s); err != nil {
		return []*v1beta1.PipelineRun{}, err
	}
	return resolve(cs, []byte(allTemplates), generateName)
}
