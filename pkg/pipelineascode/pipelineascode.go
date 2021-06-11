package pipelineascode

import (
	"context"
	"fmt"
	"strings"
	"time"

	apipac "github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/config"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/resolve"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/webvcs"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	tektonDir               = ".tekton"
	maxPipelineRunStatusRun = 5
)

type Options struct {
	PayloadFile string
	RunInfo     webvcs.RunInfo
}

// The time to wait for a pipelineRun, maybe we should not restrict this?
const pipelineRunTimeout = 2 * time.Hour

func createStatus(ctx context.Context, cs *cli.Clients, runinfo *webvcs.RunInfo, status, conclusion, text, detailsURL string, logit bool) error {
	if logit {
		cs.Log.Infof(text)
	}
	_, err := cs.GithubClient.CreateStatus(ctx, runinfo, status, conclusion, text, detailsURL)
	return err
}

func fmtDuration(d time.Duration) string {
	d = d.Round(time.Minute)
	m := d / time.Minute
	return fmt.Sprintf("%02d", m)
}

// Run over the main loop
func Run(ctx context.Context, cs *cli.Clients, k8int cli.KubeInteractionIntf, runinfo *webvcs.RunInfo) error {
	var err error

	// Create first check run to let know the user we have started the pipeline
	// TODO: Refactor this bit in a function
	checkRun, err := cs.GithubClient.CreateCheckRun(ctx, "in_progress", runinfo)
	if err != nil {
		return err
	}
	// Set the runId on runInfo so if we have an error we can report it on UI (GH checks UI for GH PR)
	runinfo.CheckRunID = checkRun.ID

	// Check if submitted is allowed to run this.
	allowed, err := aclCheck(ctx, cs, runinfo)
	if err != nil {
		return err
	}

	if !allowed {
		msg := fmt.Sprintf("User %s is not allowed to run CI on this repo.", runinfo.Sender)
		err = createStatus(ctx, cs, runinfo, "completed", "skipped", msg, "https://tenor.com/search/police-cat-gifs", true)
		if err != nil {
			return err
		}
		return nil
	}

	// Match the Event to a Repository Resource,
	// TODO: we need to be able to force a Namespace from the configuration as annotation as an extra layer of security for // hijacking
	repo, err := config.GetRepoByCR(ctx, cs, runinfo)
	if err != nil {
		return err
	}
	if repo.Spec.Namespace == "" {
		msg := fmt.Sprintf("Could not find a namespace match for %s/%s on target-branch:%s event-type: %s", runinfo.Owner, runinfo.Repository, runinfo.BaseBranch, runinfo.EventType)
		if runinfo.EventType == "pull_request" || runinfo.TriggerTarget == "issue-recheck" {
			err = createStatus(ctx, cs, runinfo, "completed", "skipped", msg, "https://tenor.com/search/sad-cat-gifs", true)
			if err != nil {
				return err
			}
		} else {
			cs.Log.Infof("Skipping creating status check: %s", msg)
		}
		return nil
	}

	// Get everything in tekton directory
	objects, err := cs.GithubClient.GetTektonDir(ctx, tektonDir, runinfo)
	if len(objects) == 0 || err != nil {
		msg := "😿 Could not find a <b>.tekton/</b> directory for this repository"
		err2 := createStatus(ctx, cs, runinfo, "completed", "skipped",
			msg, "https://tenor.com/search/sad-cat-gifs", true)
		if err2 != nil {
			return err
		}
		return err
	}
	cs.Log.Infow("Loading payload",
		"url", runinfo.URL,
		"branch", runinfo.BaseBranch,
		"sha", runinfo.SHA,
		"event_type", "pull_request")

	// Make sure we have the namespace already created or error it.
	// TODO: this probably can be trashed since repo is only can be created in
	// Namespace
	err = k8int.GetNamespace(ctx, repo.Spec.Namespace)
	if err != nil {
		return err
	}

	// Update status in UI
	err = createStatus(ctx, cs, runinfo, "in_progress", "",
		fmt.Sprintf("Getting pipelinerun configuration in namespace <b>%s</b>", repo.Spec.Namespace),
		"https://tenor.com/search/sad-cat-gifs", true)
	if err != nil {
		return err
	}

	// Concat all yaml files as one multi document yaml string
	allTemplates, err := cs.GithubClient.ConcatAllYamlFiles(ctx, objects, runinfo)
	if err != nil {
		return err
	}

	// Replace those {{var}} placeholders user has in her template to the runinfo variable
	allTemplates = ReplacePlaceHoldersVariables(allTemplates, map[string]string{
		"revision": runinfo.SHA,
		"repo_url": runinfo.URL,
	})

	ropt := &resolve.Opts{
		GenerateName: true,
		RemoteTasks:  true,
	}
	// Merge everything (i.e: tasks/pipeline etc..) as a single pipelinerun
	pipelineRuns, err := resolve.Resolve(ctx, cs, runinfo, allTemplates, ropt)
	if err != nil {
		return err
	}

	// Match the pipelinerun with annotation
	pipelineRun, err := config.MatchPipelinerunByAnnotation(pipelineRuns, cs, runinfo)
	if err != nil {
		return err
	}

	// Add labels on the soon to be created pipelinerun so UI/CLI can easily
	// query them. Since K8s do not like slash in labels value and on push we
	// have the full ref, we replace the "/" by "-". The tools probably need to
	// be aware of it when querying.
	refTomakeK8Happy := strings.ReplaceAll(runinfo.BaseBranch, "/", "-")
	pipelineRun.Labels = map[string]string{
		"tekton.dev/pipeline-ascode-owner":      runinfo.Owner,
		"tekton.dev/pipeline-ascode-repository": runinfo.Repository,
		"tekton.dev/pipeline-ascode-sha":        runinfo.SHA,
		"tekton.dev/pipeline-ascode-sender":     runinfo.Sender,
		"tekton.dev/pipeline-ascode-event-type": runinfo.EventType,
		"tekton.dev/pipeline-ascode-branch":     refTomakeK8Happy,
	}

	// Create the actual pipeline
	pr, err := cs.Tekton.TektonV1beta1().PipelineRuns(repo.Spec.Namespace).Create(ctx, pipelineRun, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	// Get the UI/webconsole URL for this pipeline to watch the log (only openshift console supported atm)
	consoleURL, err := k8int.GetConsoleUI(ctx, repo.Spec.Namespace, pr.GetName())
	if err != nil {
		// Don't bomb out if we can't get the console UI
		consoleURL = "https://giphy.com/explore/cat-exercise-wheel"
	}

	// Create status with the log url
	err = createStatus(ctx, cs, runinfo, "in_progress", "",
		fmt.Sprintf(`Starting Pipelinerun <b>%s</b> in namespace <b>%s</b><br><br>You can follow the execution on the command line with : <br><br><code>tkn pr logs -f -n %s %s</code>`,
			pr.GetName(), repo.Spec.Namespace, repo.Spec.Namespace, pr.GetName()),
		consoleURL, false)
	if err != nil {
		return err
	}

	cs.Log.Infof("Waiting for PipelineRun %s/%s to Succeed in a maximum time of %s minutes", pr.Namespace, pr.Name, fmtDuration(pipelineRunTimeout))
	if err := k8int.WaitForPipelineRunSucceed(ctx, cs.Tekton.TektonV1beta1(), pr, pipelineRunTimeout); err != nil {
		cs.Log.Info("PipelineRun has failed.")
	}

	// Post the final status to GitHub check status with a nice breakdown and
	// tekton cli describe output.
	newPr, err := postFinalStatus(ctx, cs, k8int, runinfo, pr.Name, repo.Spec.Namespace)
	if err != nil {
		return err
	}
	repoStatus := apipac.RepositoryRunStatus{
		Status:          newPr.Status.Status,
		PipelineRunName: newPr.Name,
		StartTime:       newPr.Status.StartTime,
		CompletionTime:  newPr.Status.CompletionTime,
		SHA:             &runinfo.SHA,
		Title:           &runinfo.SHATitle,
	}

	// Get repo again in case it was updated while we were running the CI
	// NOTE: there may be a race issue we should maybe solve here, between the Get and
	// Update but we are talking sub-milliseconds issue here.
	lastrepo, err := cs.PipelineAsCode.PipelinesascodeV1alpha1().Repositories(repo.Spec.Namespace).Get(ctx, repo.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	// Append pipelinerun status files to the repo status
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
