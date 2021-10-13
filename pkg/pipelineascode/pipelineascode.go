package pipelineascode

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	apipac "github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/kubeinteraction"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/matcher"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/resolve"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/webvcs"
	tektonv1beta1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	tektonDir               = ".tekton"
	maxPipelineRunStatusRun = 5
	startingPipelineRunText = `Starting Pipelinerun <b>%s</b> in namespace
  <b>%s</b><br><br>You can follow the execution on the [OpenShift console](%s) pipelinerun viewer or via
  the command line with :
	<br><code>tkn pr logs -f -n %s %s</code>`
)

// The time to wait for a pipelineRun, maybe we should not restrict this?
const pipelineRunTimeout = 2 * time.Hour

func Run(ctx context.Context, cs *params.Run, vcsintf webvcs.Interface, k8int kubeinteraction.Interface) error {
	var err error

	// Match the Event to a Repository Resource,
	// We are going to match on targetNamespace annotation later on in
	// `MatchPipelinerunByAnnotation`
	repo, err := matcher.GetRepoByCR(ctx, cs, "")
	if err != nil {
		return err
	}

	if repo == nil || repo.Spec.URL == "" {
		msg := fmt.Sprintf("Could not find a namespace match for %s/%s on target-branch:%s event-type: %s", cs.Info.Event.Owner, cs.Info.Event.Repository, cs.Info.Event.BaseBranch, cs.Info.Event.EventType)

		if cs.Info.Pac.VCSToken == "" {
			cs.Clients.Log.Error(msg)
			cs.Clients.Log.Error("cannot set status since not token has been set")
			return nil
		}

		err = createStatus(ctx, vcsintf, cs, webvcs.StatusOpts{
			Status:     "completed",
			Conclusion: "skipped",
			Text:       msg,
			DetailsURL: "https://tenor.com/search/sad-cat-gifs",
		}, true)

		if err != nil {
			return err
		}
		return nil
	}

	if repo.Spec.WebvcsAPISecret != nil {
		err := secretFromRepository(ctx, cs, k8int, vcsintf.GetConfig(), repo)
		if err != nil {
			return err
		}
		// We already SetClient before ParseWebhook in case if we already set
		// the token (ie: github apps) and not coming from the repository
		err = vcsintf.SetClient(ctx, cs.Info.Pac)
		if err != nil {
			return err
		}
	}

	// Get the SHA commit info, we want to get the URL and commit title
	err = vcsintf.GetCommitInfo(ctx, cs.Info.Event)
	if err != nil {
		return err
	}

	// Check if the submitter is allowed to run this.
	allowed, err := vcsintf.IsAllowed(ctx, cs.Info.Event)
	if err != nil {
		return err
	}

	if !allowed {
		msg := fmt.Sprintf("User %s is not allowed to run CI on this repo.", cs.Info.Event.Sender)
		if cs.Info.Event.AccountID != "" {
			msg = fmt.Sprintf("User: %s AccountID: %s is not allowed to run CI on this repo.", cs.Info.Event.Sender,
				cs.Info.Event.AccountID)
		}
		err = createStatus(ctx, vcsintf, cs, webvcs.StatusOpts{
			Status:     "completed",
			Conclusion: "skipped",
			Text:       msg,
			DetailsURL: "https://tenor.com/search/police-cat-gifs",
		}, true)
		if err != nil {
			return err
		}
		return nil
	}

	// Get everything in tekton directory
	allTemplates, err := vcsintf.GetTektonDir(ctx, cs.Info.Event, tektonDir)
	if allTemplates == "" || err != nil {
		msg := fmt.Sprintf("%s - Could not find a **.tekton/** directory for this repository", cs.Info.Pac.ApplicationName)

		err = createStatus(ctx, vcsintf, cs, webvcs.StatusOpts{
			Status:     "completed",
			Conclusion: "skipped",
			Text:       msg,
			DetailsURL: "https://tenor.com/search/sad-cat-gifs",
		}, true)

		if err != nil {
			return err
		}
		return err
	}
	cs.Clients.Log.Infow("Loading payload",
		"url", cs.Info.Event.URL,
		"branch", cs.Info.Event.BaseBranch,
		"sha", cs.Info.Event.SHA,
		"event_type", "pull_request")

	// Replace those {{var}} placeholders user has in her template to the cs.Info variable
	allTemplates = ReplacePlaceHoldersVariables(allTemplates, map[string]string{
		"revision":   cs.Info.Event.SHA,
		"repo_url":   cs.Info.Event.URL,
		"repo_owner": cs.Info.Event.Owner,
		"repo_name":  cs.Info.Event.Repository,
	})

	ropt := &resolve.Opts{
		GenerateName: true,
		RemoteTasks:  true, // TODO: add an option to disable remote tasking,
	}
	// Merge everything (i.e: tasks/pipeline etc..) as a single pipelinerun
	pipelineRuns, err := resolve.Resolve(ctx, cs, vcsintf, allTemplates, ropt)
	if err != nil {
		return err
	}

	// Match the pipelinerun with annotation
	pipelineRun, annotationRepo, config, err := matcher.MatchPipelinerunByAnnotation(ctx, pipelineRuns, cs)
	if err != nil {
		return err
	}

	if annotationRepo.Spec.URL != "" {
		repo = annotationRepo
	}

	// Automatically create a secret with the token to be reused by git-clone task
	if cs.Info.Pac.SecretAutoCreation {
		err = k8int.CreateBasicAuthSecret(ctx, cs.Info.Event, cs.Info.Pac, repo.GetNamespace())
		if err != nil {
			return err
		}
	}

	// Add labels and annotations to pipelinerun
	addLabelsAndAnnotations(cs, pipelineRun, repo)

	// Create the actual pipeline
	pr, err := cs.Clients.Tekton.TektonV1beta1().PipelineRuns(repo.GetNamespace()).Create(ctx, pipelineRun, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	// Get the UI/webconsole URL for this pipeline to watch the log (only openshift console supported atm)
	consoleURL, err := k8int.GetConsoleUI(ctx, repo.GetNamespace(), pr.GetName())
	if err != nil {
		// Don't bomb out if we can't get the console UI
		consoleURL = "https://giphy.com/explore/cat-exercise-wheel"
	}

	// Create status with the log url
	msg := fmt.Sprintf(startingPipelineRunText, pr.GetName(),
		repo.GetNamespace(), consoleURL, repo.GetNamespace(), pr.GetName())
	err = createStatus(ctx, vcsintf, cs, webvcs.StatusOpts{
		Status:     "in_progress",
		Conclusion: "pending",
		Text:       msg,
		DetailsURL: consoleURL,
	}, false)
	if err != nil {
		return err
	}

	cs.Clients.Log.Infof("Waiting for PipelineRun %s/%s to Succeed in a maximum time of %s minutes", pr.Namespace, pr.Name, fmtDuration(pipelineRunTimeout))
	if err := k8int.WaitForPipelineRunSucceed(ctx, cs.Clients.Tekton.TektonV1beta1(), pr, pipelineRunTimeout); err != nil {
		cs.Clients.Log.Info("PipelineRun has failed.")
	}

	// Do cleanups
	if keepMaxPipeline, ok := config["max-keep-runs"]; ok {
		max, err := strconv.Atoi(keepMaxPipeline)
		if err != nil {
			return err
		}

		err = k8int.CleanupPipelines(ctx, repo.GetNamespace(), repo.Name, max)
		if err != nil {
			return err
		}
	}

	// Post the final status to GitHub check status with a nice breakdown and
	// tekton cli describe output.
	newPr, err := postFinalStatus(ctx, cs, k8int, vcsintf, pr.Name, repo.GetNamespace())
	if err != nil {
		return err
	}
	repoStatus := apipac.RepositoryRunStatus{
		Status:          newPr.Status.Status,
		PipelineRunName: newPr.Name,
		StartTime:       newPr.Status.StartTime,
		CompletionTime:  newPr.Status.CompletionTime,
		SHA:             &cs.Info.Event.SHA,
		SHAURL:          &cs.Info.Event.SHAURL,
		Title:           &cs.Info.Event.SHATitle,
		LogURL:          &consoleURL,
	}

	// Get repo again in case it was updated while we were running the CI
	// NOTE: there may be a race issue we should maybe solve here, between the Get and
	// Update but we are talking sub-milliseconds issue here.
	lastrepo, err := cs.Clients.PipelineAsCode.PipelinesascodeV1alpha1().Repositories(repo.GetNamespace()).Get(ctx, repo.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	// Append pipelinerun status files to the repo status
	if len(lastrepo.Status) >= maxPipelineRunStatusRun {
		copy(lastrepo.Status, lastrepo.Status[len(lastrepo.Status)-maxPipelineRunStatusRun+1:])
		lastrepo.Status = lastrepo.Status[:maxPipelineRunStatusRun-1]
	}

	lastrepo.Status = append(lastrepo.Status, repoStatus)
	nrepo, err := cs.Clients.PipelineAsCode.PipelinesascodeV1alpha1().Repositories(lastrepo.Namespace).Update(
		ctx, lastrepo, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	cs.Clients.Log.Infof("Repository status of %s has been updated", nrepo.Name)
	return err
}

// cleanupLabel k8s do not like slash in labels value and on push we have the
// full ref, we replace the "/" by "-". The tools probably need to be aware of
// it when querying.
func cleanupLabel(s string) string {
	replasoeur := strings.NewReplacer("/", "-", " ", "_")
	return replasoeur.Replace(s)
}

func addLabelsAndAnnotations(cs *params.Run, pipelineRun *tektonv1beta1.PipelineRun, repo *apipac.Repository) {
	// Add labels on the soon to be created pipelinerun so UI/CLI can easily
	// query them.
	pipelineRun.Labels = map[string]string{
		"app.kubernetes.io/managed-by":              "pipelines-as-code",
		"pipelinesascode.tekton.dev/url-org":        cleanupLabel(cs.Info.Event.Owner),
		"pipelinesascode.tekton.dev/url-repository": cleanupLabel(cs.Info.Event.Repository),
		"pipelinesascode.tekton.dev/sha":            cleanupLabel(cs.Info.Event.SHA),
		"pipelinesascode.tekton.dev/sender":         cleanupLabel(cs.Info.Event.Sender),
		"pipelinesascode.tekton.dev/event-type":     cleanupLabel(cs.Info.Event.EventType),
		"pipelinesascode.tekton.dev/branch":         cleanupLabel(cs.Info.Event.BaseBranch),
		"pipelinesascode.tekton.dev/repository":     cleanupLabel(repo.GetName()),
	}

	pipelineRun.Annotations["pipelinesascode.tekton.dev/sha-title"] = cs.Info.Event.SHATitle
	pipelineRun.Annotations["pipelinesascode.tekton.dev/sha-url"] = cs.Info.Event.SHAURL
}

func fmtDuration(d time.Duration) string {
	d = d.Round(time.Minute)
	m := d / time.Minute
	return fmt.Sprintf("%02d", m)
}

func secretFromRepository(ctx context.Context, cs *params.Run, k8int kubeinteraction.Interface, config *info.VCSConfig, repo *apipac.Repository) error {
	var err error

	if repo.Spec.WebvcsAPIURL == "" {
		repo.Spec.WebvcsAPIURL = config.APIURL
	}

	cs.Info.Pac.VCSUser = repo.Spec.WebvcsAPIUser
	cs.Info.Pac.VCSToken, err = k8int.GetSecret(
		ctx,
		kubeinteraction.GetSecretOpt{
			Namespace: repo.GetNamespace(),
			Name:      repo.Spec.WebvcsAPISecret.Name,
			Key:       repo.Spec.WebvcsAPISecret.Key,
		},
	)

	if err != nil {
		return err
	}
	cs.Info.Pac.VCSInfoFromRepo = true

	if repo.Spec.WebvcsAPIUser != "" {
		cs.Clients.Log.Infof("Using vcs-user %s", repo.Spec.WebvcsAPIUser)
	}
	cs.Clients.Log.Infof("Using vcs-token from secret %s in key %s", repo.Spec.WebvcsAPISecret.Name, repo.Spec.WebvcsAPISecret.Key)
	return nil
}
