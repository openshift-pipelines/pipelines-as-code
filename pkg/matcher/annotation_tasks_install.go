package matcher

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"regexp"
	"strings"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/hub"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider"
	tektonv1beta1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"go.uber.org/zap"
	k8scheme "k8s.io/client-go/kubernetes/scheme"
)

const (
	taskAnnotationsRegexp = `task(-[0-9]+)?$`
)

type RemoteTasks struct {
	Run *params.Run
}

func (rt RemoteTasks) convertTotask(data string) (*tektonv1beta1.Task, error) {
	decoder := k8scheme.Codecs.UniversalDeserializer()
	obj, _, err := decoder.Decode([]byte(data), nil, nil)
	if err != nil {
		return nil, fmt.Errorf("we have a task that is not looking like a kubernetes resource: task: %s resource: %w", data, err)
	}

	return obj.(*tektonv1beta1.Task), nil
}

func (rt RemoteTasks) getTask(ctx context.Context, providerintf provider.Interface, task string) (*tektonv1beta1.Task, error) {
	var ret *tektonv1beta1.Task

	// TODO: print a log info when getting the task from which location
	switch {
	case strings.HasPrefix(task, "https://"), strings.HasPrefix(task, "http://"):
		// TODO: Add a context
		res, err := rt.Run.Clients.HTTP.Get(task)
		if err != nil {
			return ret, err
		}
		data, _ := ioutil.ReadAll(res.Body)
		defer res.Body.Close()
		return rt.convertTotask(string(data))
	case strings.Contains(task, "/"):
		var data string
		var err error
		if rt.Run.Info.Event.SHA != "" {
			data, err = providerintf.GetFileInsideRepo(ctx, rt.Run.Info.Event, task, "")
			if err != nil {
				return ret, err
			}
		} else {
			data, err = getTaskFromLocalFS(task, rt.Run.Clients.Log)
			if err != nil {
				return nil, err
			}
			if data == "" {
				return nil, nil
			}
		}

		return rt.convertTotask(data)
	default:
		data, err := hub.GetTask(ctx, rt.Run, task)
		if err != nil {
			return nil, err
		}
		return rt.convertTotask(data)
	}
}

// GetTaskFromAnnotations Get task remotely if they are on Annotations
func (rt RemoteTasks) GetTaskFromAnnotations(ctx context.Context, providerintf provider.Interface,
	annotations map[string]string) ([]*tektonv1beta1.Task, error) {
	var ret []*tektonv1beta1.Task
	rtareg := regexp.MustCompile(fmt.Sprintf("%s/%s", pipelinesascode.GroupName, taskAnnotationsRegexp))
	for annotationK, annotationV := range annotations {
		if !rtareg.Match([]byte(annotationK)) {
			continue
		}
		tasks, err := getAnnotationValues(annotationV)
		if err != nil {
			return ret, err
		}
		for _, v := range tasks {
			task, err := rt.getTask(ctx, providerintf, v)
			if err != nil {
				return ret, err
			}
			ret = append(ret, task)
		}
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
		logger.Warnf("could not find remote task %s inside repo", taskName)
		return "", nil
	}

	b, err := ioutil.ReadFile(taskName)
	data = string(b)
	if err != nil {
		return "", err
	}
	return data, nil
}
