package config

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"regexp"
	"strings"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/webvcs"
	"github.com/tektoncd/hub/api/pkg/cli/hub"
	tektonv1beta1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	k8scheme "k8s.io/client-go/kubernetes/scheme"
)

const (
	tektonCatalogHubName  = "tekton"
	taskAnnotationsRegexp = `task(-[0-9]+)?$`
)

type RemoteTasks struct {
	Clients *cli.Clients
	Runinfo *webvcs.RunInfo
}

func (rt RemoteTasks) convertTotask(data string) (*tektonv1beta1.Task, error) {
	decoder := k8scheme.Codecs.UniversalDeserializer()
	obj, _, err := decoder.Decode([]byte(data), nil, nil)
	if err != nil {
		// TODO: Better debug information
		return nil, errors.New("we have a task that is not looking like a kubernetes resource")
	}

	return obj.(*tektonv1beta1.Task), nil
}

func (rt RemoteTasks) getTask(ctx context.Context, task string) (*tektonv1beta1.Task, error) {
	var ret *tektonv1beta1.Task
	switch {
	case strings.HasPrefix(task, "https://"), strings.HasPrefix(task, "http://"):
		res, err := rt.Clients.HTTPClient.Get(task)
		if err != nil {
			return ret, err
		}
		data, _ := ioutil.ReadAll(res.Body)
		defer res.Body.Close()
		return rt.convertTotask(string(data))
	case strings.Contains(task, "/"):
		data, err := rt.Clients.GithubClient.GetFileInsideRepo(ctx, task, false, rt.Runinfo)
		if err != nil {
			return ret, err
		}
		return rt.convertTotask(string(data))
	default:
		var version string
		taskName := task
		if strings.Contains(task, ":") {
			split := strings.Split(task, ":")
			version = split[len(split)-1]
			taskName = split[0]
		}
		resource := rt.Clients.Hub.GetResource(hub.ResourceOption{
			Name:    taskName,
			Catalog: tektonCatalogHubName,
			Kind:    "task",
			Version: version,
		})
		data, err := resource.Manifest()
		if err != nil {
			return ret, err
		}
		return rt.convertTotask(string(data))
	}
}

// GetTaskFromAnnotations Get task remotely if they are on Annotations
func (rt RemoteTasks) GetTaskFromAnnotations(ctx context.Context, annotations map[string]string) ([]*tektonv1beta1.Task, error) {
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
			task, err := rt.getTask(ctx, v)
			if err != nil {
				return ret, err
			}
			ret = append(ret, task)
		}
	}
	return ret, nil
}
