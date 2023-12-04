package consoleui

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/keys"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/settings"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/templates"
	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"k8s.io/client-go/dynamic"
)

type CustomConsole struct {
	Info   *info.Info
	params map[string]string
}

func (o *CustomConsole) SetParams(mt map[string]string) {
	o.params = mt
}

func (o *CustomConsole) GetName() string {
	if o.Info.Pac.CustomConsoleName == "" {
		return "Not configured"
	}
	return o.Info.Pac.CustomConsoleName
}

func (o *CustomConsole) URL() string {
	if o.Info.Pac.CustomConsoleURL == "" {
		return fmt.Sprintf("https://url.setting.%s.is.not.configured", settings.CustomConsoleURLKey)
	}
	return o.Info.Pac.CustomConsoleURL
}

// generateURL will generate a URL from a template, trim some of the spaces and
// \n we get from yaml
// return the default URL if there it's not become a proper url or that it has
// some of the templates like {{}} left.
func (o *CustomConsole) generateURL(urlTmpl string, dict map[string]string) string {
	newurl := templates.ReplacePlaceHoldersVariables(urlTmpl, dict, nil, nil, map[string]interface{}{})
	// trim new line because yaml parser adds new line at the end of the string
	newurl = strings.TrimSpace(strings.TrimSuffix(newurl, "\n"))
	if _, err := url.ParseRequestURI(newurl); err != nil {
		return o.URL()
	}
	// detect if there is still some {{}} in the url
	if keys.ParamsRe.MatchString(newurl) {
		return o.URL()
	}
	return newurl
}

func (o *CustomConsole) DetailURL(pr *tektonv1.PipelineRun) string {
	if o.Info.Pac.CustomConsolePRdetail == "" {
		return fmt.Sprintf("https://detailurl.setting.%s.is.not.configured", settings.CustomConsolePRDetailKey)
	}
	nm := o.params
	// make sure the map is not nil before setting this up
	// there is a case where SetParams is not called before DetailURL and this would crash the container
	if nm == nil {
		nm = make(map[string]string)
	}
	nm["namespace"] = pr.GetNamespace()
	nm["pr"] = pr.GetName()
	return o.generateURL(o.Info.Pac.CustomConsolePRdetail, nm)
}

func (o *CustomConsole) NamespaceURL(pr *tektonv1.PipelineRun) string {
	if o.Info.Pac.CustomConsoleNamespaceURL == "" {
		return fmt.Sprintf("https://detailurl.setting.%s.is.not.configured", settings.CustomConsoleNamespaceURLKey)
	}
	nm := o.params
	// make sure the map is not nil before setting this up
	// there is a case where SetParams is not called before DetailURL and this would crash the container
	if nm == nil {
		nm = make(map[string]string)
	}
	nm["namespace"] = pr.GetNamespace()
	return o.generateURL(o.Info.Pac.CustomConsoleNamespaceURL, nm)
}

func (o *CustomConsole) TaskLogURL(pr *tektonv1.PipelineRun, taskRunStatus *tektonv1.PipelineRunTaskRunStatus) string {
	if o.Info.Pac.CustomConsolePRTaskLog == "" {
		return fmt.Sprintf("https://tasklogurl.setting.%s.is.not.configured", settings.CustomConsolePRTaskLogKey)
	}
	firstFailedStep := ""
	// search for the first failed steps in taskrunstatus
	for _, step := range taskRunStatus.Status.Steps {
		if step.Terminated != nil && step.Terminated.ExitCode != 0 {
			firstFailedStep = step.Name
			break
		}
	}

	nm := o.params
	// make sure the map is not nil before setting this up
	// there is a case where SetParams is not called before DetailURL and this would crash the container
	if nm == nil {
		nm = make(map[string]string)
	}
	nm["namespace"] = pr.GetNamespace()
	nm["pr"] = pr.GetName()
	nm["task"] = taskRunStatus.PipelineTaskName
	nm["pod"] = taskRunStatus.Status.PodName
	nm["firstFailedStep"] = firstFailedStep
	return o.generateURL(o.Info.Pac.CustomConsolePRTaskLog, nm)
}

func (o *CustomConsole) UI(_ context.Context, _ dynamic.Interface) error {
	return nil
}
