package resolve

import (
	"errors"
	"fmt"
	"strings"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"k8s.io/client-go/kubernetes/scheme"
)

type Types struct {
	PipelineRuns []*v1beta1.PipelineRun
	Pipelines    []*v1beta1.Pipeline
	TaskRuns     []*v1beta1.TaskRun
	Tasks        []*v1beta1.Task
}

func readTypes(data []byte) (Types, error) {
	var types = Types{}
	decoder := scheme.Codecs.UniversalDeserializer()

	for _, doc := range strings.Split(strings.Trim(string(data), "-"), "---") {
		if strings.TrimSpace(doc) == "" {
			continue
		}

		obj, _, err := decoder.Decode([]byte(doc), nil, nil)
		if err != nil {
			return types, err
		}
		switch o := obj.(type) {
		case *v1beta1.Pipeline:
			types.Pipelines = append(types.Pipelines, o)
		case *v1beta1.PipelineRun:
			types.PipelineRuns = append(types.PipelineRuns, o)
		case *v1beta1.Task:
			types.Tasks = append(types.Tasks, o)
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

// Resolve gets a set of bytes which contains Pipeline/PipelineRuns/Tasks and
// resolve them inline as a single PipelineRun
func Resolve(data []byte) ([]*v1beta1.PipelineRun, error) {
	types, err := readTypes(data)
	if err != nil {
		return []*v1beta1.PipelineRun{}, err
	}
	if len(types.PipelineRuns) == 0 {
		return []*v1beta1.PipelineRun{}, errors.New("We need at least one pipelinerun to start with")
	}

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
		if pipelinerun.Spec.PipelineRef != nil {
			pipelineResolved, err := getPipelineByName(pipelinerun.Spec.PipelineRef.Name, types.Pipelines)
			if err != nil {
				return []*v1beta1.PipelineRun{}, err
			}
			pipelinerun.Spec.PipelineRef = nil
			pipelinerun.Spec.PipelineSpec = &pipelineResolved.Spec

		}
	}
	return types.PipelineRuns, nil
}
