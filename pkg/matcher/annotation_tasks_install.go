package matcher

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/hub"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider"
	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	tektonv1beta1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"go.uber.org/zap"
	k8scheme "k8s.io/client-go/kubernetes/scheme"
)

const (
	taskAnnotationsRegexp     = `task(-[0-9]+)?$`
	pipelineAnnotationsRegexp = `pipeline(-[0-9]+)?$`
)

type RemoteTasks struct {
	Run               *params.Run
	ProviderInterface provider.Interface
	Event             *info.Event
	Logger            *zap.SugaredLogger
}

// nolint: dupl
func (rt RemoteTasks) convertToPipeline(ctx context.Context, data string) (*tektonv1.Pipeline, error) {
	decoder := k8scheme.Codecs.UniversalDeserializer()
	obj, _, err := decoder.Decode([]byte(data), nil, nil)
	if err != nil {
		return nil, fmt.Errorf("we have a pipeline that is not looking like a kubernetes resource: pipeline: %s resource: %w", data, err)
	}

	var pipeline *tektonv1.Pipeline
	switch o := obj.(type) {
	case *tektonv1.Pipeline:
		pipeline = o
	case *tektonv1beta1.Pipeline:
		c := &tektonv1.Pipeline{}
		// TODO: figure ou the issue we have with setdefault setting defaults SA
		// and then don't let pipeline do its job to automatically set a
		// pipeline on configuration
		// o.SetDefaults(ctx)
		// ctx2 := features.SetFeatureFlag(context.Background())
		// if err := o.Validate(ctx2); err != nil {
		// 	return nil, fmt.Errorf("remote pipeline %s cannot be validated properly: err: %w", o.GetName(), err)
		// }
		if err := o.ConvertTo(ctx, c); err != nil {
			return nil, fmt.Errorf("remote pipeline v1beta1 %s cannot be converted as v1: err: %w", o.GetName(), err)
		}
		pipeline = c
	default:
		return nil, fmt.Errorf("this doesn't seem to be a proper pipeline: %v", o)
	}

	return pipeline, nil
}

// nolint: dupl
// golint has decided that it is a duplication with convertToPipeline but i swear it isnt does two are different function
// and not even sure this is possible to do this with generic crazyness
func (rt RemoteTasks) convertTotask(ctx context.Context, data string) (*tektonv1.Task, error) {
	decoder := k8scheme.Codecs.UniversalDeserializer()
	obj, _, err := decoder.Decode([]byte(data), nil, nil)
	if err != nil {
		return nil, fmt.Errorf("we have a task that is not looking like a kubernetes resource: task: %s resource: %w", data, err)
	}

	var task *tektonv1.Task
	switch o := obj.(type) {
	case *tektonv1.Task:
		task = o
	case *tektonv1beta1.Task:
		c := &tektonv1.Task{}
		// o.SetDefaults(ctx)
		// if err := o.Validate(ctx); err != nil {
		// 	return nil, fmt.Errorf("remote task %s cannot be validated properly: err: %w", o.GetName(), err)
		// }
		if err := o.ConvertTo(ctx, c); err != nil {
			return nil, fmt.Errorf("remote task v1beta1 %s cannot be converted as v1: err: %w", o.GetName(), err)
		}
		task = c
	default:
		return nil, fmt.Errorf("this doesn't seem to be a proper task: %v", o)
	}

	return task, nil
}

func (rt RemoteTasks) getRemote(ctx context.Context, uri string, fromHub bool) (string, error) {
	if fetchedFromURIFromProvider, task, err := rt.ProviderInterface.GetTaskURI(ctx, rt.Run, rt.Event, uri); fetchedFromURIFromProvider {
		return task, err
	}

	switch {
	case strings.HasPrefix(uri, "https://"), strings.HasPrefix(uri, "http://"):
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, uri, nil)
		res, err := rt.Run.Clients.HTTP.Do(req)
		if err != nil {
			return "", err
		}
		if res.StatusCode != http.StatusOK {
			return "", fmt.Errorf("cannot get remote resource: \"%s\": %s", uri, res.Status)
		}
		data, _ := io.ReadAll(res.Body)
		defer res.Body.Close()
		rt.Logger.Infof("successfully fetched \"%s\" from remote https url", uri)
		return string(data), nil
	case strings.Contains(uri, "/"):
		var data string
		var err error
		if rt.Event.SHA != "" {
			data, err = rt.ProviderInterface.GetFileInsideRepo(ctx, rt.Event, uri, "")
			if err != nil {
				return "", err
			}
		} else {
			data, err = getTaskFromLocalFS(uri, rt.Logger)
			if err != nil {
				return "", err
			}
			if data == "" {
				return "", nil
			}
		}

		rt.Logger.Infof("successfully fetched \"%s\" inside repository", uri)
		return data, nil
	case fromHub:
		data, err := hub.GetTask(ctx, rt.Run, uri)
		if err != nil {
			return "", err
		}
		rt.Logger.Infof("successfully fetched \"%s\" from hub URL: %s", uri, rt.Run.Info.Pac.HubURL)
		return data, nil
	}
	return "", fmt.Errorf(`cannot find "%s" anywhere`, uri)
}

func grabValuesFromAnnotations(annotations map[string]string, annotationReg string) ([]string, error) {
	rtareg := regexp.MustCompile(fmt.Sprintf("%s/%s", pipelinesascode.GroupName, annotationReg))
	var ret []string
	for annotationK, annotationV := range annotations {
		if !rtareg.Match([]byte(annotationK)) {
			continue
		}
		items, err := getAnnotationValues(annotationV)
		if err != nil {
			return ret, err
		}
		ret = append(items, ret...)
	}
	return ret, nil
}

// GetTaskFromAnnotations Get task remotely if they are on Annotations
func (rt RemoteTasks) GetTaskFromAnnotations(ctx context.Context, annotations map[string]string) ([]*tektonv1.Task, error) {
	ret := []*tektonv1.Task{}
	tasks, err := grabValuesFromAnnotations(annotations, taskAnnotationsRegexp)
	if err != nil {
		return nil, err
	}
	for _, v := range tasks {
		data, err := rt.getRemote(ctx, v, true)
		if err != nil {
			return nil, fmt.Errorf("error getting remote task \"%s\": %w", v, err)
		}
		if data == "" {
			return nil, fmt.Errorf("error getting remote task \"%s\": returning empty", v)
		}

		task, err := rt.convertTotask(ctx, data)
		if err != nil {
			return nil, err
		}
		ret = append(ret, task)
	}
	return ret, nil
}

// GetPipelineFromAnnotations Get pipeline remotely if they are on Annotations
// TODO: merge in a generic between the two
func (rt RemoteTasks) GetPipelineFromAnnotations(ctx context.Context, annotations map[string]string) ([]*tektonv1.Pipeline, error) {
	ret := []*tektonv1.Pipeline{}
	pipelinesAnnotation, err := grabValuesFromAnnotations(annotations, pipelineAnnotationsRegexp)
	if err != nil {
		return nil, err
	}
	if len(pipelinesAnnotation) > 1 {
		return nil, fmt.Errorf("only one pipeline is allowed on remote resolution, we have received multiple of them: %+v", pipelinesAnnotation)
	}
	for _, v := range pipelinesAnnotation {
		data, err := rt.getRemote(ctx, v, false)
		if err != nil {
			return nil, fmt.Errorf("error getting remote pipeline %s: %w", v, err)
		}
		if data == "" {
			return nil, fmt.Errorf("could not get pipeline \"%s\": returning empty", v)
		}
		pipeline, err := rt.convertToPipeline(ctx, data)
		if err != nil {
			return nil, err
		}
		ret = append(ret, pipeline)
	}
	return ret, nil
}

// getTaskFromLocalFS get task locally if file exist
// TODO: may want to try chroot to the git root dir first as well if we are able so.
func getTaskFromLocalFS(taskName string, logger *zap.SugaredLogger) (string, error) {
	var data string
	// We are most probably running with tkn pac resolve -f here, so
	// let's try by any chance to check locally if the task is here on
	// the filesystem
	if _, err := os.Stat(taskName); errors.Is(err, os.ErrNotExist) {
		logger.Warnf("could not find remote task %s inside Repo", taskName)
		return "", nil
	}

	b, err := os.ReadFile(taskName)
	data = string(b)
	if err != nil {
		return "", err
	}
	return data, nil
}
