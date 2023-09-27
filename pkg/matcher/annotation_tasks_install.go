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
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/settings"
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
func (rt RemoteTasks) convertToPipeline(ctx context.Context, uri, data string) (*tektonv1.Pipeline, error) {
	decoder := k8scheme.Codecs.UniversalDeserializer()
	obj, _, err := decoder.Decode([]byte(data), nil, nil)
	if err != nil {
		return nil, fmt.Errorf("remote pipeline from uri: %s cannot be parsed as a kubernetes resource: %w", uri, err)
	}

	var pipeline *tektonv1.Pipeline
	switch o := obj.(type) {
	case *tektonv1.Pipeline:
		pipeline = o
	case *tektonv1beta1.Pipeline: //nolint: staticcheck
		c := &tektonv1.Pipeline{}
		// TODO: figure ou the issue we have with setdefault setting defaults SA
		// and then don't let pipeline do its job to automatically set a
		// pipeline on configuration
		// o.SetDefaults(ctx)
		// ctx2 := features.SetFeatureFlag(context.Background())
		// if err := o.Validate(ctx2); err != nil {
		// return nil, fmt.Errorf("remote pipeline from uri: %s with name %s cannot be validated: %w", uri, o.GetName(), err)
		// }
		if err := o.ConvertTo(ctx, c); err != nil {
			return nil, fmt.Errorf("remote pipeline from uri: %s with name %s cannot be converted to v1beta1: %w", uri, o.GetName(), err)
		}
		pipeline = c
	default:
		return nil, fmt.Errorf("remote pipeline from uri: %s has not been recognized as a tekton pipeline: %v", uri, o)
	}

	return pipeline, nil
}

// nolint: dupl
// golint has decided that it is a duplication with convertToPipeline but i swear it isn't does two are different function
// and not even sure this is possible to do this with generic crazyness
func (rt RemoteTasks) convertTotask(ctx context.Context, uri, data string) (*tektonv1.Task, error) {
	decoder := k8scheme.Codecs.UniversalDeserializer()
	obj, _, err := decoder.Decode([]byte(data), nil, nil)
	if err != nil {
		return nil, fmt.Errorf("remote task from uri: %s cannot be parsed as a kubernetes resource: %w", uri, err)
	}

	var task *tektonv1.Task
	switch o := obj.(type) {
	case *tektonv1.Task:
		task = o
	case *tektonv1beta1.Task: //nolint: staticcheck // we need to support v1beta1
		c := &tektonv1.Task{}
		// o.SetDefaults(ctx)
		// if err := o.Validate(ctx); err != nil {
		// 	return nil, fmt.Errorf("remote task %s cannot be validated properly: err: %w", o.GetName(), err)
		// return nil, fmt.Errorf("remote task from uri: %s with name %s cannot be validated: %w", uri, o.GetName(), err)
		// }
		if err := o.ConvertTo(ctx, c); err != nil {
			return nil, fmt.Errorf("remote task from uri: %s with name %s cannot be converted to v1beta1: %w", uri, o.GetName(), err)
		}
		task = c
	default:
		return nil, fmt.Errorf("remote task from uri: %s has not been recognized as a tekton task: %v", uri, o)
	}

	return task, nil
}

func (rt RemoteTasks) getRemote(ctx context.Context, uri string, fromHub bool) (string, error) {
	if fetchedFromURIFromProvider, task, err := rt.ProviderInterface.GetTaskURI(ctx, rt.Run, rt.Event, uri); fetchedFromURIFromProvider {
		return task, err
	}

	switch {
	case strings.HasPrefix(uri, "https://"), strings.HasPrefix(uri, "http://"): // if it starts with http(s)://, it is a remote resource
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, uri, nil)
		res, err := rt.Run.Clients.HTTP.Do(req)
		if err != nil {
			return "", err
		}
		if res.StatusCode != http.StatusOK {
			return "", fmt.Errorf("cannot get remote resource: %s: %s", uri, res.Status)
		}
		data, _ := io.ReadAll(res.Body)
		defer res.Body.Close()
		rt.Logger.Infof("successfully fetched %s from remote https url", uri)
		return string(data), nil
	case fromHub && strings.Contains(uri, "://"): // if it contains ://, it is a remote custom catalog
		split := strings.Split(uri, "://")
		catalogID := split[0]
		value, _ := rt.Run.Info.Pac.HubCatalogs.Load(catalogID)
		if _, ok := rt.Run.Info.Pac.HubCatalogs.Load(catalogID); !ok {
			rt.Logger.Infof("custom catalog %s is not found, skipping", catalogID)
			return "", nil
		}
		uri = strings.TrimPrefix(uri, fmt.Sprintf("%s://", catalogID))
		data, err := hub.GetTask(ctx, rt.Run, catalogID, uri)
		if err != nil {
			return "", err
		}
		catalogValue, ok := value.(settings.HubCatalog)
		if !ok {
			return "", fmt.Errorf("could not get details for catalog name: %s", catalogID)
		}
		rt.Logger.Infof("successfully fetched task %s from custom catalog HUB %s on URL %s", uri, catalogID, catalogValue.URL)
		return data, nil
	case strings.Contains(uri, "/"): // if it contains a slash, it is a file inside a repository
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

		rt.Logger.Infof("successfully fetched %s inside repository", uri)
		return data, nil
	case fromHub: // finally a simple word will fetch from the default catalog (if enabled)
		data, err := hub.GetTask(ctx, rt.Run, "default", uri)
		if err != nil {
			return "", err
		}
		value, _ := rt.Run.Info.Pac.HubCatalogs.Load("default")
		catalogValue, ok := value.(settings.HubCatalog)
		if !ok {
			return "", fmt.Errorf("could not get details for catalog name: %s", "default")
		}
		rt.Logger.Infof("successfully fetched %s from default configured catalog HUB on URL: %s", uri, catalogValue.URL)
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

		task, err := rt.convertTotask(ctx, v, data)
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
		pipeline, err := rt.convertToPipeline(ctx, v, data)
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
