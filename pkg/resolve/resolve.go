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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8scheme "k8s.io/client-go/kubernetes/scheme"
	yaml "sigs.k8s.io/yaml/goyaml.v2"
)

type TektonTypes struct {
	PipelineRuns     []*tektonv1.PipelineRun
	Pipelines        []*tektonv1.Pipeline
	TaskRuns         []*tektonv1.TaskRun
	Tasks            []*tektonv1.Task
	ValidationErrors map[string]string
}

func NewTektonTypes() TektonTypes {
	return TektonTypes{
		ValidationErrors: map[string]string{},
	}
}

var yamlDocSeparatorRe = regexp.MustCompile(`(?m)^---\s*$`)

// detectAtleastNameOrGenerateNameFromPipelineRun detects the name or
// generateName of a yaml files even if there is an error decoding it as tekton types.
func detectAtleastNameOrGenerateNameFromPipelineRun(data string) string {
	var metadataName struct {
		Metadata metav1.ObjectMeta
	}
	err := yaml.Unmarshal([]byte(data), &metadataName)
	if err != nil {
		return ""
	}
	if metadataName.Metadata.Name != "" {
		return metadataName.Metadata.Name
	}

	// TODO: yaml Unmarshal don't want to parse generatename and i have no idea why
	if metadataName.Metadata.GenerateName != "" {
		return metadataName.Metadata.GenerateName
	}
	return "unknown"
}

// getTaskRunByName returns the taskrun with the given name the first one found
// will be matched. It does not handle conflicts so user has fetched multiple
// taskruns with the same name it will just pick up the first one.
// if the taskrun is not found it returns an error.
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

func pipelineRunsWithSameName(prs []*tektonv1.PipelineRun) error {
	prNames := map[string]bool{}
	for _, pr := range prs {
		name := pr.GetName()
		generateName := pr.GetGenerateName()

		if name != "" {
			if _, exist := prNames[name]; exist {
				return fmt.Errorf("found multiple pipelinerun in .tekton with the same name: %v, please update", name)
			}
			prNames[name] = true
		}

		if generateName != "" {
			if _, exist := prNames[generateName]; exist {
				return fmt.Errorf("found multiple pipelinerun in .tekton with the same generateName: %v, please update", generateName)
			}
			prNames[generateName] = true
		}
	}
	return nil
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

func inlineTasks(tasks []tektonv1.PipelineTask, ropt *Opts, types TektonTypes) ([]tektonv1.PipelineTask, error) {
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
			tmd := tektonv1.PipelineTaskMetadata{
				Annotations: taskResolved.GetObjectMeta().GetAnnotations(),
				Labels:      taskResolved.GetObjectMeta().GetLabels(),
			}
			task.TaskRef = nil
			task.TaskSpec = &tektonv1.EmbeddedTask{
				TaskSpec: taskResolved.Spec,
				Metadata: tmd,
			}
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

func ReadTektonTypes(ctx context.Context, log *zap.SugaredLogger, data string) (TektonTypes, error) {
	types := NewTektonTypes()
	decoder := k8scheme.Codecs.UniversalDeserializer()

	for _, doc := range yamlDocSeparatorRe.Split(data, -1) {
		if strings.TrimSpace(doc) == "" {
			continue
		}

		obj, _, err := decoder.Decode([]byte(doc), nil, nil)
		if err != nil {
			types.ValidationErrors[detectAtleastNameOrGenerateNameFromPipelineRun(doc)] = err.Error()
			continue
		}
		switch o := obj.(type) {
		case *tektonv1beta1.Pipeline: //nolint: staticcheck // we need to support v1beta1
			c := &tektonv1.Pipeline{}
			if err := o.ConvertTo(ctx, c); err != nil {
				return types, fmt.Errorf("pipeline v1beta1 %s cannot be converted as v1: err: %w", o.GetName(), err)
			}
			types.Pipelines = append(types.Pipelines, c)
		case *tektonv1beta1.PipelineRun: //nolint: staticcheck // we need to support v1beta1
			c := &tektonv1.PipelineRun{}
			if err := o.ConvertTo(ctx, c); err != nil {
				return types, fmt.Errorf("pipelinerun v1beta1 %s cannot be converted as v1: err: %w", o.GetName(), err)
			}
			types.PipelineRuns = append(types.PipelineRuns, c)
		case *tektonv1beta1.Task: //nolint: staticcheck // we need to support v1beta1
			c := &tektonv1.Task{}
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
			log.Info("skipping yaml document not looking like a tekton resource we can Resolve.")
		}
	}

	return types, nil
}

// Resolve gets a large string which is a yaml multi documents containing
// Pipeline/PipelineRuns/Tasks and resolve them inline as a single PipelineRun
// generateName can be set as True to set the name as a generateName + "-" for
// unique pipelinerun.
func Resolve(ctx context.Context, cs *params.Run, logger *zap.SugaredLogger, providerintf provider.Interface, types TektonTypes, event *info.Event, ropt *Opts) ([]*tektonv1.PipelineRun, error) {
	if len(types.PipelineRuns) == 0 {
		return []*tektonv1.PipelineRun{}, fmt.Errorf("could not find any PipelineRun in your .tekton/ directory")
	}

	if _, err := MetadataResolve(types.PipelineRuns); err != nil {
		return []*tektonv1.PipelineRun{}, err
	}

	// Resolve remote annotations on remote task or remote pipeline or tasks
	// inside remote pipeline
	if ropt.RemoteTasks {
		rt := &matcher.RemoteTasks{
			Run:               cs,
			Event:             event,
			ProviderInterface: providerintf,
			Logger:            logger,
		}
		var err error
		if types, err = getRemotes(ctx, rt, types); err != nil {
			return []*tektonv1.PipelineRun{}, err
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
			turns, err := inlineTasks(pipelinerun.Spec.PipelineSpec.Tasks, ropt, types)
			if err != nil {
				return nil, err
			}
			pipelinerun.Spec.PipelineSpec.Tasks = turns

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

		// Add a GenerateName based on the pipeline name and a "-"
		// if we already have a GenerateName then just keep it like this
		if ropt.GenerateName && pipelinerun.GenerateName == "" {
			pipelinerun.GenerateName = pipelinerun.Name + "-"
			pipelinerun.Name = ""
		}
	}
	return types.PipelineRuns, nil
}

func MetadataResolve(prs []*tektonv1.PipelineRun) ([]*tektonv1.PipelineRun, error) {
	if err := pipelineRunsWithSameName(prs); err != nil {
		return []*tektonv1.PipelineRun{}, err
	}

	for _, prun := range prs {
		originPipelineRunName := prun.GetName()
		if originPipelineRunName == "" && prun.GenerateName != "" {
			originPipelineRunName = prun.GetGenerateName()
		}

		// keep the originalPipelineRun in a label
		// because we would need it later on when grouping by cleanups and we
		// can attach that pr file from .tekton directory.

		// Don't overwrite the labels if there is some who already exist set by the user in repo
		if prun.GetLabels() == nil {
			prun.Labels = map[string]string{}
		}
		// Don't overwrite the annotation if there is some who already exist set by the user in repo
		if prun.GetAnnotations() == nil {
			prun.Annotations = map[string]string{}
		}
		prun.GetLabels()[apipac.OriginalPRName] = formatting.CleanValueKubernetes(originPipelineRunName)
		prun.GetAnnotations()[apipac.OriginalPRName] = originPipelineRunName
	}
	return prs, nil
}

//nolint:gochecknoinits
func init() {
	_ = tektonv1.AddToScheme(k8scheme.Scheme)
	_ = tektonv1beta1.AddToScheme(k8scheme.Scheme)
}
