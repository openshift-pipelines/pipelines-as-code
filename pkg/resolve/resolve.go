package resolve

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	apipac "github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/keys"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/formatting"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/matcher"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider"
	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	tektonv1beta1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"go.uber.org/zap"
	k8scheme "k8s.io/client-go/kubernetes/scheme"
)

type Types struct {
	PipelineRuns []*tektonv1.PipelineRun
	Pipelines    []*tektonv1.Pipeline
	TaskRuns     []*tektonv1.TaskRun
	Tasks        []*tektonv1.Task
}

var yamlDocSeparatorRe = regexp.MustCompile(`(?m)^---\s*$`)

func readTypes(ctx context.Context, log *zap.SugaredLogger, data string) (Types, error) {
	types := Types{}
	decoder := k8scheme.Codecs.UniversalDeserializer()

	for _, doc := range yamlDocSeparatorRe.Split(data, -1) {
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
			c := &tektonv1.Pipeline{}
			// o.SetDefaults(ctx)
			// this would fail on Validate otherwise
			// if o.GetName() == "" {
			// 	o.SetName(o.GetGenerateName())
			// }
			// ctx2 := features.SetFeatureFlag(context.Background())
			// if err := o.Validate(ctx2); err != nil {
			// 	return types, fmt.Errorf("pipeline %s cannot be validated properly: err: %w", o.GetName(), err)
			// }
			if err := o.ConvertTo(ctx, c); err != nil {
				return types, fmt.Errorf("pipeline v1beta1 %s cannot be converted as v1: err: %w", o.GetName(), err)
			}
			types.Pipelines = append(types.Pipelines, c)
		case *tektonv1beta1.PipelineRun:
			c := &tektonv1.PipelineRun{}
			// o.SetDefaults(ctx)
			// this would fail on Validate otherwise
			// if o.GetName() == "" {
			// 	o.SetName(o.GetGenerateName())
			// }
			// ctx2 := features.SetFeatureFlag(context.Background())
			// if err := o.Validate(ctx2); err != nil {
			// 	return types, fmt.Errorf("pipelinerun %s cannot be validated properly: err: %w", o.GetName(), err)
			// }
			if err := o.ConvertTo(ctx, c); err != nil {
				return types, fmt.Errorf("pipelinerun v1beta1 %s cannot be converted as v1: err: %w", o.GetName(), err)
			}
			types.PipelineRuns = append(types.PipelineRuns, c)
		case *tektonv1beta1.Task:
			c := &tektonv1.Task{}
			// o.SetDefaults(ctx)
			// // this would fail on Validate otherwise
			// if o.GetName() == "" {
			// 	o.SetName(o.GetGenerateName())
			// }
			// if err := o.Validate(ctx); err != nil {
			// 	return types, fmt.Errorf("task %s cannot be validated properly: err: %w", o.GetName(), err)
			// }
			if err := o.ConvertTo(ctx, c); err != nil {
				return types, fmt.Errorf("task v1beta1 %s cannot be converted as v1: err: %w", o.GetName(), err)
			}
			types.Tasks = append(types.Tasks, c)
		case *tektonv1.PipelineRun:
			types.PipelineRuns = append(types.PipelineRuns, o)
		case *tektonv1.Pipeline:
			types.Pipelines = append(types.Pipelines, o)
		case *tektonv1.Task:
			types.Tasks = append(types.Tasks, o)
		default:
			log.Info("Skipping document not looking like a tekton resource we can Resolve.")
		}
	}

	return types, nil
}

func getTaskByName(name string, tasks []*tektonv1.Task) (*tektonv1.Task, error) {
	for _, value := range tasks {
		if value.Name == name {
			return value, nil
		}
	}
	return &tektonv1.Task{}, fmt.Errorf("cannot find referenced task %s. if it's a remote task make sure to add it in the annotations", name)
}

func getPipelineByName(name string, tasks []*tektonv1.Pipeline) (*tektonv1.Pipeline, error) {
	for _, value := range tasks {
		if value.Name == name {
			return value, nil
		}
	}
	return &tektonv1.Pipeline{}, fmt.Errorf("cannot find referenced pipeline %s. for a remote pipeline make sure to add it in the annotation", name)
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

func inlineTasks(tasks []tektonv1.PipelineTask, ropt *Opts, types Types) ([]tektonv1.PipelineTask, error) {
	pipelineTasks := []tektonv1.PipelineTask{}
	for _, task := range tasks {
		if task.TaskRef != nil &&
			task.TaskRef.Resolver == "" &&
			isTektonAPIVersion(task.TaskRef.APIVersion) &&
			string(task.TaskRef.Kind) != "ClusterTask" &&
			!skippingTask(task.TaskRef.Name, ropt.SkipInlining) {
			taskResolved, err := getTaskByName(task.TaskRef.Name, types.Tasks)
			if err != nil {
				return nil, err
			}
			task.TaskRef = nil
			task.TaskSpec = &tektonv1.EmbeddedTask{TaskSpec: taskResolved.Spec}
		}
		pipelineTasks = append(pipelineTasks, task)
	}
	return pipelineTasks, nil
}

type Opts struct {
	GenerateName  bool     // whether to GenerateName
	RemoteTasks   bool     // whether to parse annotation to fetch tasks from remote
	SkipInlining  []string // task to skip inlining
	ProviderToken string
}

// Resolve gets a large string which is a yaml multi documents containing
// Pipeline/PipelineRuns/Tasks and resolve them inline as a single PipelineRun
// generateName can be set as True to set the name as a generateName + "-" for
// unique pipelinerun
func Resolve(ctx context.Context, cs *params.Run, logger *zap.SugaredLogger, providerintf provider.Interface, event *info.Event, data string, ropt *Opts) ([]*tektonv1.PipelineRun, error) {
	types, err := readTypes(ctx, logger, data)
	if err != nil {
		return []*tektonv1.PipelineRun{}, err
	}
	if len(types.PipelineRuns) == 0 {
		return []*tektonv1.PipelineRun{}, fmt.Errorf("could not find any PipelineRun in your .tekton/ directory")
	}

	if err := pipelineRunsWithSameName(types.PipelineRuns); err != nil {
		return []*tektonv1.PipelineRun{}, err
	}

	// First resolve Annotations Tasks
	for _, pipelinerun := range types.PipelineRuns {
		if ropt.RemoteTasks && pipelinerun.GetObjectMeta().GetAnnotations() != nil {
			rt := matcher.RemoteTasks{
				Run:               cs,
				Event:             event,
				ProviderInterface: providerintf,
				Logger:            logger,
			}
			remoteTasks, err := rt.GetTaskFromAnnotations(ctx, pipelinerun.GetObjectMeta().GetAnnotations())
			if err != nil {
				return []*tektonv1.PipelineRun{}, err
			}
			// Merge remote tasks with local tasks
			types.Tasks = append(types.Tasks, remoteTasks...)

			remotePipelines, err := rt.GetPipelineFromAnnotations(ctx, pipelinerun.GetObjectMeta().GetAnnotations())
			if err != nil {
				return []*tektonv1.PipelineRun{}, err
			}
			types.Pipelines = append(types.Pipelines, remotePipelines...)
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
		if pipelinerun.Spec.PipelineRef != nil && pipelinerun.Spec.PipelineRef.Resolver == "" {
			pipelineResolved, err := getPipelineByName(pipelinerun.Spec.PipelineRef.Name, types.Pipelines)
			if err != nil {
				return []*tektonv1.PipelineRun{}, err
			}
			pipelinerun.Spec.PipelineRef = nil
			pipelinerun.Spec.PipelineSpec = &pipelineResolved.Spec
		}

		var originPipelinerunName string

		originPipelinerunName = pipelinerun.Name
		// Add a GenerateName based on the pipeline name and a "-"
		// if we already have a GenerateName then just keep it like this
		switch {
		case ropt.GenerateName && pipelinerun.GenerateName == "":
			pipelinerun.GenerateName = pipelinerun.Name + "-"
			pipelinerun.Name = ""
		case pipelinerun.GetGenerateName() == pipelinerun.GetName():
			pipelinerun.SetName("")
		case originPipelinerunName == "" && pipelinerun.GenerateName != "":
			originPipelinerunName = pipelinerun.GenerateName
		}

		// keep the originalPipelineRun in a label
		// because we would need it later on when grouping by cleanups and we
		// can attach that pr file from .tekton directory.

		// Don't overwrite the labels if there is some who already exist set by the user in repo
		if pipelinerun.Labels == nil {
			pipelinerun.Labels = map[string]string{}
		}
		pipelinerun.Labels[apipac.OriginalPRName] = formatting.K8LabelsCleanup(originPipelinerunName)
	}
	return types.PipelineRuns, nil
}

func pipelineRunsWithSameName(prs []*tektonv1.PipelineRun) error {
	prNames := map[string]bool{}
	for _, pr := range prs {
		_, exist := prNames[pr.GetName()]
		if exist {
			return fmt.Errorf("found multiple pipelinerun in .tekton with same name: %v, please update", pr.GetName())
		}
		prNames[pr.GetName()] = true
	}
	return nil
}

//nolint:gochecknoinits
func init() {
	_ = tektonv1.AddToScheme(k8scheme.Scheme)
	_ = tektonv1beta1.AddToScheme(k8scheme.Scheme)
}
