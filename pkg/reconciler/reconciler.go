package reconciler

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/kubeinteraction"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/pipelineascode"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider/bitbucketcloud"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider/bitbucketserver"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider/github"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider/gitlab"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	pipelinerunreconciler "github.com/tektoncd/pipeline/pkg/client/injection/reconciler/pipeline/v1beta1/pipelinerun"
	v1beta12 "github.com/tektoncd/pipeline/pkg/client/listers/pipeline/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/logging"
	pkgreconciler "knative.dev/pkg/reconciler"
)

type Reconciler struct {
	run               *params.Run
	pipelineRunLister v1beta12.PipelineRunLister
	kinteract         *kubeinteraction.Interaction
}

var _ pipelinerunreconciler.Interface = (*Reconciler)(nil)

var gitAuthSecretAnnotation = "pipelinesascode.tekton.dev/git-auth-secret"

func (r *Reconciler) ReconcileKind(ctx context.Context, pr *v1beta1.PipelineRun) pkgreconciler.Event {
	logger := logging.FromContext(ctx)
	if pr.IsDone() {
		logger.Infof("pipelineRun %v is done !!!  ", pr.GetName())
		return r.reportStatus(ctx, pr)
	}
	return nil
}

func (r *Reconciler) reportStatus(ctx context.Context, pr *v1beta1.PipelineRun) error {
	logger := logging.FromContext(ctx)

	prLabels := pr.GetLabels()
	prAnno := pr.GetAnnotations()

	// fetch repository CR for pipelineRun
	repoName := prLabels[filepath.Join(pipelinesascode.GroupName, "repository")]
	repo, err := r.run.Clients.PipelineAsCode.PipelinesascodeV1alpha1().
		Repositories(pr.Namespace).Get(ctx, repoName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	// Cleanup old succeeded pipelineruns
	keepMaxPipeline, ok := prAnno[filepath.Join(pipelinesascode.GroupName, "max-keep-runs")]
	if ok {
		max, err := strconv.Atoi(keepMaxPipeline)
		if err != nil {
			return err
		}

		err = r.kinteract.CleanupPipelines(ctx, logger, repo, pr, max)
		if err != nil {
			return err
		}
	}

	provider, err := detectProvider(pr)
	if err != nil {
		logger.Error(err)
		return nil
	}
	provider.SetLogger(logger)

	event := eventFromPipelineRun(pr)

	// if its a GH app pipelineRun then init client
	if event.InstallationID != 0 {
		// if check run id doesn't exist on the pipelineRun, then wait for it
		_, ok := prLabels[filepath.Join(pipelinesascode.GroupName, "check-run-id")]
		if !ok {
			logger.Infof("could not find check run id")
			return nil
		}
		gh := &github.Provider{}
		event.Provider.Token, err = gh.GetAppToken(ctx, r.run.Clients.Kube, event.GHEURL, event.InstallationID)
		if err != nil {
			return err
		}
		provider = gh
	}

	if repo.Spec.GitProvider != nil {
		if err := pipelineascode.SecretFromRepository(ctx, r.run, r.kinteract, provider.GetConfig(), event, repo, logger); err != nil {
			return err
		}
	} else {
		event.Provider.WebhookSecret, _ = pipelineascode.GetCurrentNSWebhookSecret(ctx, r.kinteract)
	}

	if r.run.Info.Pac.SecretAutoCreation {
		var gitAuthSecretName string
		if annotation, ok := prAnno[gitAuthSecretAnnotation]; ok {
			gitAuthSecretName = annotation
		} else {
			return fmt.Errorf("cannot get annotation %s as set on PR", gitAuthSecretAnnotation)
		}

		err = r.kinteract.DeleteBasicAuthSecret(ctx, logger, repo.GetNamespace(), gitAuthSecretName)
		if err != nil {
			return fmt.Errorf("deleting basic auth secret has failed: %w ", err)
		}
	}

	err = provider.SetClient(ctx, event)
	if err != nil {
		return err
	}

	newPr, err := r.postFinalStatus(ctx, logger, provider, event, pr)
	if err != nil {
		return err
	}

	if err := r.updateRepoRunStatus(ctx, logger, newPr, repo, event); err != nil {
		return err
	}

	return r.updatePipelineRunState(ctx, pr)
}

func (r *Reconciler) updatePipelineRunState(ctx context.Context, pr *v1beta1.PipelineRun) error {
	newPr, err := r.pipelineRunLister.PipelineRuns(pr.Namespace).Get(pr.Name)
	if err != nil {
		return fmt.Errorf("error getting PipelineRun %s when updating state: %w", pr.Name, err)
	}

	newPr = newPr.DeepCopy()
	newPr.Labels[filepath.Join(pipelinesascode.GroupName, "state")] = kubeinteraction.StateCompleted

	_, err = r.run.Clients.Tekton.TektonV1beta1().PipelineRuns(pr.Namespace).Update(ctx, newPr, metav1.UpdateOptions{})
	return err
}

func detectProvider(pr *v1beta1.PipelineRun) (provider.Interface, error) {
	gitProvider, ok := pr.GetLabels()[filepath.Join(pipelinesascode.GroupName, "git-provider")]
	if !ok {
		return nil, fmt.Errorf("failed to detect git provider for pipleinerun %s : git-provider label not found", pr.GetName())
	}

	var provider provider.Interface
	switch gitProvider {
	case "github", "github-enterprise", "gitea":
		provider = &github.Provider{}
	case "gitlab":
		provider = &gitlab.Provider{}
	case "bitbucket-cloud":
		provider = &bitbucketcloud.Provider{}
	case "bitbucket-server":
		provider = &bitbucketserver.Provider{}
	default:
		return nil, fmt.Errorf("failed to detect provider for pipelinerun: %s : unknown provider", pr.GetName())
	}
	return provider, nil
}

func eventFromPipelineRun(pr *v1beta1.PipelineRun) *info.Event {
	event := info.NewEvent()

	prLabels := pr.GetLabels()
	prAnno := pr.GetAnnotations()

	event.Organization = prLabels[filepath.Join(pipelinesascode.GroupName, "url-org")]
	event.Repository = prLabels[filepath.Join(pipelinesascode.GroupName, "url-repository")]
	event.EventType = prLabels[filepath.Join(pipelinesascode.GroupName, "event-type")]
	event.BaseBranch = prLabels[filepath.Join(pipelinesascode.GroupName, "branch")]
	event.SHA = prLabels[filepath.Join(pipelinesascode.GroupName, "sha")]

	event.SHATitle = prAnno[filepath.Join(pipelinesascode.GroupName, "sha-title")]
	event.SHAURL = prAnno[filepath.Join(pipelinesascode.GroupName, "sha-url")]

	prNumber := prAnno[filepath.Join(pipelinesascode.GroupName, "pull-request")]
	if prNumber != "" {
		event.PullRequestNumber, _ = strconv.Atoi(prNumber)
	}

	// GitHub Specific
	if prNumber, ok := prAnno[filepath.Join(pipelinesascode.GroupName, "installation-id")]; ok {
		id, _ := strconv.Atoi(prNumber)
		event.InstallationID = int64(id)
	}
	if gheURL, ok := prAnno[filepath.Join(pipelinesascode.GroupName, "ghe-url")]; ok {
		event.GHEURL = gheURL
	}

	return event
}
