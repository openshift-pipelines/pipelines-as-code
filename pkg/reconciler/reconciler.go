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
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider/bitbucketcloud"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider/bitbucketserver"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider/github"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider/gitlab"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	v1beta12 "github.com/tektoncd/pipeline/pkg/client/informers/externalversions/pipeline/v1beta1"
	pipelinerunreconciler "github.com/tektoncd/pipeline/pkg/client/injection/reconciler/pipeline/v1beta1/pipelinerun"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/logging"
	pkgreconciler "knative.dev/pkg/reconciler"
)

type Reconciler struct {
	run         *params.Run
	pipelinerun v1beta12.PipelineRunInformer
	kinteract   *kubeinteraction.Interaction
}

var (
	_ pipelinerunreconciler.Interface = (*Reconciler)(nil)
)

func (c *Reconciler) ReconcileKind(ctx context.Context, pr *v1beta1.PipelineRun) pkgreconciler.Event {
	logger := logging.FromContext(ctx)
	if pr.IsDone() {
		logger.Infof("pipelineRun %v is done !!!  ", pr.GetName())
		return c.reportStatus(ctx, pr)
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
		Repositories(pr.Namespace).Get(ctx, repoName, v1.GetOptions{})
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

	event := eventFromPipelineRun(pr)

	// if its a GH app pipelinerun then init client
	if event.InstallationID != -1 {
		gh := &github.Provider{}
		event.Provider.Token, err = gh.GetAppToken(ctx, r.run.Clients.Kube, event.GHEURL, event.InstallationID)
		if err != nil {
			return err
		}
		provider = gh
	}

	return nil
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
