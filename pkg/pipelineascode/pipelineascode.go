package pipelineascode

import (
	"context"
	"fmt"
	"path/filepath"

	apipac "github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/resolve"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/webvcs"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	tektonDir               = ".tekton"
	tektonConfigurationFile = "tekton.yaml"
	maxPipelineRunStatusRun = 5
)

type Options struct {
	Payload     string
	PayloadFile string
	RunInfo     webvcs.RunInfo
}

func getRepoByCR(cs *cli.Clients, url, branch, forceNamespace string) (apipac.Repository, error) {
	var repository apipac.Repository

	repositories, err := cs.PipelineAsCode.PipelinesascodeV1alpha1().Repositories("").List(
		context.Background(), metav1.ListOptions{})
	if err != nil {
		return repository, err
	}
	for _, value := range repositories.Items {
		if value.Spec.URL == url && value.Spec.Branch == branch {
			if forceNamespace != "" && value.Namespace != forceNamespace {
				return repository, fmt.Errorf(
					"repo CR matches but should be installed in %s as configured from tekton.yaml on the main branch",
					forceNamespace)
			}

			// Disallow attempts for hijacks. If the installed CR is not configured on the
			// Namespace the Spec is targeting then disallow it.
			if value.Namespace != value.Spec.Namespace {
				return repository, fmt.Errorf("repo CR %s matches but belongs to %s while it should be in %s",
					value.Name,
					value.Namespace,
					value.Spec.Namespace)
			}
			return value, nil
		}
	}
	return repository, nil
}

func Run(cs *cli.Clients, k8int cli.KubeInteractionIntf, runinfo *webvcs.RunInfo) error {
	var err error
	var maintekton TektonYamlConfig

	checkRun, err := cs.GithubClient.CreateCheckRun("in_progress", runinfo)
	if err != nil {
		return err
	}
	runinfo.CheckRunID = checkRun.ID

	allowed, err := aclCheck(cs, runinfo)
	if err != nil {
		return err
	}

	if !allowed {
		_, _ = cs.GithubClient.CreateStatus(runinfo, "completed", "skipped",
			fmt.Sprintf("User %s is not allowed to run CI on this repo.", runinfo.Sender),
			"https://tenor.com/search/police-cat-gifs")
		cs.Log.Infof("User %s is not allowed to run CI on this repo", runinfo.Sender)
		return nil
	}

	maintektonyaml, _ := cs.GithubClient.GetFileFromDefaultBranch(filepath.Join(tektonDir, tektonConfigurationFile), runinfo)
	if maintektonyaml != "" {
		maintekton, err = processTektonYaml(cs, runinfo, maintektonyaml)
		if err != nil {
			return err
		}
	}
	repo, err := getRepoByCR(cs, runinfo.URL, runinfo.Branch, maintekton.Namespace)
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

	err = k8int.GetNamespace(repo.Spec.Namespace)
	if err != nil {
		return err
	}
	_, err = cs.GithubClient.CreateStatus(runinfo, "in_progress", "",
		fmt.Sprintf("Creating pipelinerun in namespace <b>%s</b>", repo.Spec.Namespace),
		"https://tenor.com/search/sad-cat-gifs")
	if err != nil {
		return err
	}

	yamlConfig := TektonYamlConfig{}
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
	prun, err := resolve.Resolve(cs, allTemplates, true)
	if err != nil {
		return err
	}

	ctx := context.Background()
	pr, err := cs.Tekton.TektonV1beta1().PipelineRuns(repo.Spec.Namespace).Create(ctx, prun[0], metav1.CreateOptions{})
	if err != nil {
		return err
	}

	consoleURL, err := k8int.GetConsoleUI(repo.Spec.Namespace, pr.GetName())
	if err != nil {
		// Don't bomb out if we can't get the console UI
		consoleURL = "https://giphy.com/explore/cat-exercise-wheel"
	}

	_, err = cs.GithubClient.CreateStatus(runinfo, "in_progress", "",
		fmt.Sprintf(`Starting Pipelinerun <b>%s</b> in namespace <b>%s</b><br><br>You can follow the execution on the command line with : <br><br><code>tkn pr logs -f -n %s %s</code>`,
			pr.GetName(), repo.Spec.Namespace, repo.Spec.Namespace, pr.GetName()),
		consoleURL)
	if err != nil {
		return nil
	}

	// Use this as a wait holder until the logs is finished, maybe we would do something with the log output.
	_, err = k8int.TektonCliFollowLogs(repo.Spec.Namespace, pr.GetName())
	if err != nil {
		return err
	}

	newPr, err := postFinalStatus(ctx, cs, k8int, runinfo, pr.Name, repo.Spec.Namespace)
	if err != nil {
		return err
	}

	repoStatus := apipac.RepositoryRunStatus{
		Status:          newPr.Status.Status,
		PipelineRunName: newPr.Name,
		StartTime:       newPr.Status.StartTime,
		CompletionTime:  newPr.Status.CompletionTime,
	}

	// TODO: Get another time the repo in case it was updated, there may be a
	// locking problem we should solve here but we are talking miliseconds race.
	lastrepo, err := cs.PipelineAsCode.PipelinesascodeV1alpha1().Repositories(repo.Spec.Namespace).Get(ctx, repo.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	// TODO: Reversed?
	if len(lastrepo.Status) >= maxPipelineRunStatusRun {
		copy(lastrepo.Status, lastrepo.Status[len(lastrepo.Status)-maxPipelineRunStatusRun+1:])
		lastrepo.Status = lastrepo.Status[:maxPipelineRunStatusRun-1]
	}
	lastrepo.Status = append(lastrepo.Status, repoStatus)
	nrepo, err := cs.PipelineAsCode.PipelinesascodeV1alpha1().Repositories(lastrepo.Namespace).Update(
		ctx, lastrepo, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	cs.Log.Infof("Repository status of %s has been updated", nrepo.Name)
	return err
}
