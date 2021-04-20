package pipelineascode

import (
	"context"

	apipac "github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli"
	k8pac "github.com/openshift-pipelines/pipelines-as-code/pkg/kubernetes"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/resolve"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/tektoncli"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/webvcs"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	tektonDir               = ".tekton"
	tektonConfigurationFile = "tekton.yaml"
)

type Options struct {
	GithubPayLoad string
}

func getRepoByCRD(cs *cli.Clients, url, branch, eventType string) (apipac.Repository, error) {
	var repository apipac.Repository

	repositories, err := cs.PipelineAsCode.PipelinesascodeV1alpha1().Repositories("").List(
		context.Background(), metav1.ListOptions{})
	if err != nil {
		return repository, err
	}
	for _, value := range repositories.Items {
		if value.Spec.URL == url && value.Spec.Branch == branch && value.Spec.EventType == eventType {
			return value, nil
		}
	}
	return repository, nil
}

func Run(p cli.Params, cs *cli.Clients, runinfo *webvcs.RunInfo) error {
	var err error
	var ctx = context.Background()
	checkRun, err := cs.GithubClient.CreateCheckRun("in_progress", runinfo)
	if err != nil {
		return err
	}
	runinfo.CheckRunID = checkRun.ID

	repo, err := getRepoByCRD(cs, runinfo.URL, runinfo.Branch, "pull_request")
	if err != nil {
		return err
	}

	if repo.Spec.Namespace == "" {
		_, _ = cs.GithubClient.CreateStatus(runinfo, "completed", "skipped",
			"Could not find a configuration for this repository", "https://tenor.com/search/sad-cat-gifs")
		cs.Log.Infof("Could not find a namespace match for %s/%s on %s", runinfo.Owner, runinfo.Repository, runinfo.Branch)
		return nil
	}

	objects, err := cs.GithubClient.GetTektonDir(tektonDir, runinfo)
	if err != nil {
		_, _ = cs.GithubClient.CreateStatus(runinfo, "completed", "skipped",
			"😿 Could not find a <b>.tekton/</b> directory for this repository", "https://tenor.com/search/sad-cat-gifs")
		return err
	}

	cs.Log.Infow("Loading payload",
		"url", runinfo.URL,
		"branch", runinfo.Branch,
		"sha", runinfo.SHA,
		"event_type", "pull_request")

	err = k8pac.CreateNamespace(cs, repo.Spec.Namespace)
	if err != nil {
		return err
	}

	var yamlConfig = TektonYamlConfig{}
	for _, file := range objects {
		if file.GetName() == tektonConfigurationFile {
			data, err := cs.GithubClient.GetObject(file.GetSHA(), runinfo)
			if err != nil {
				return err
			}

			yamlConfig, err = processTektonYaml(cs, runinfo, string(data))
			if err != nil {
				return err
			}
			break
		}
	}

	allTemplates, err := cs.GithubClient.GetTektonDirTemplate(objects, runinfo)
	if err != nil {
		return err
	}

	allTemplates = ReplacePlaceHoldersVariables(allTemplates, map[string]string{
		"revision": runinfo.SHA,
		"repo_url": runinfo.URL,
	})

	// Do not do place holders replacement on remote tasks, who knows maybe not good!
	if yamlConfig.RemoteTasks != "" {
		allTemplates += yamlConfig.RemoteTasks
	}
	prun, err := resolve.Resolve(allTemplates, true)
	if err != nil {
		return err
	}

	pr, err := cs.Tekton.TektonV1beta1().PipelineRuns(repo.Spec.Namespace).Create(ctx, prun[0], v1.CreateOptions{})
	if err != nil {
		return err
	}

	log, err := tektoncli.FollowLogs(cs, pr.Name, repo.Spec.Namespace)
	if err != nil {
		return err
	}

	return nil

	describe, err := tektoncli.PipelineRunDescribe(pr.Name, repo.Spec.Namespace)
	if err != nil {
		return err
	}
	pr, err = cs.Tekton.TektonV1beta1().PipelineRuns(repo.Spec.Namespace).Get(ctx, pr.Name, v1.GetOptions{})
	if err != nil {
		return err
	}

	_, err = cs.GithubClient.CreateStatus(runinfo, "completed", pipelineRunStatus(pr),
		"<h2>Describe output:</h2><pre>"+describe+"</pre><h2>Log output:</h2><hr><code>"+log+"</code>", "")

	return err
}
