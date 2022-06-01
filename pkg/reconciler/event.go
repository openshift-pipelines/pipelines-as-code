package reconciler

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/formatting"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider/bitbucketcloud"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider/bitbucketserver"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider/github"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider/gitlab"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"go.uber.org/zap"
)

func (r *Reconciler) detectProvider(ctx context.Context, logger *zap.SugaredLogger, pr *v1beta1.PipelineRun) (provider.Interface, *info.Event, error) {
	gitProvider, ok := pr.GetLabels()[filepath.Join(pipelinesascode.GroupName, "git-provider")]
	if !ok {
		return nil, nil, fmt.Errorf("failed to detect git provider for pipleinerun %s : git-provider label not found", pr.GetName())
	}

	event := buildEventFromPipelineRun(pr)

	var provider provider.Interface
	switch gitProvider {
	case "github", "github-enterprise", "gitea":
		gh := &github.Provider{}
		if event.InstallationID != 0 {
			if err := gh.InitAppClient(ctx, r.run.Clients.Kube, event); err != nil {
				return nil, nil, err
			}
		}
		provider = gh
	case "gitlab":
		provider = &gitlab.Provider{}
	case "bitbucket-cloud":
		provider = &bitbucketcloud.Provider{}
	case "bitbucket-server":
		provider = &bitbucketserver.Provider{}
	default:
		return nil, nil, fmt.Errorf("failed to detect provider for pipelinerun: %s : unknown provider", pr.GetName())
	}
	provider.SetLogger(logger)
	return provider, event, nil
}

func buildEventFromPipelineRun(pr *v1beta1.PipelineRun) *info.Event {
	event := info.NewEvent()

	prLabels := pr.GetLabels()
	prAnno := pr.GetAnnotations()

	event.URL = prAnno[filepath.Join(pipelinesascode.GroupName, "repo-url")]
	// it's safer to get repo, org from repo.url since we have to remove the / and other chars in labels which drops
	// the SubPath that gitlab is using.
	repo, org, _ := formatting.GetRepoOwnerSplitted(event.URL)
	event.Organization = repo
	event.Repository = org
	event.EventType = prLabels[filepath.Join(pipelinesascode.GroupName, "event-type")]
	event.BaseBranch = prLabels[filepath.Join(pipelinesascode.GroupName, "branch")]
	event.SHA = prLabels[filepath.Join(pipelinesascode.GroupName, "sha")]

	event.SHATitle = prAnno[filepath.Join(pipelinesascode.GroupName, "sha-title")]
	event.SHAURL = prAnno[filepath.Join(pipelinesascode.GroupName, "sha-url")]

	prNumber := prAnno[filepath.Join(pipelinesascode.GroupName, "pull-request")]
	if prNumber != "" {
		event.PullRequestNumber, _ = strconv.Atoi(prNumber)
	}

	// GitHub
	if prNumber, ok := prAnno[filepath.Join(pipelinesascode.GroupName, "installation-id")]; ok {
		id, _ := strconv.Atoi(prNumber)
		event.InstallationID = int64(id)
	}
	if gheURL, ok := prAnno[filepath.Join(pipelinesascode.GroupName, "ghe-url")]; ok {
		event.GHEURL = gheURL
	}

	// Gitlab
	if projectID, ok := prAnno[filepath.Join(pipelinesascode.GroupName, "source-project-id")]; ok {
		event.SourceProjectID, _ = strconv.Atoi(projectID)
	}
	if projectID, ok := prAnno[filepath.Join(pipelinesascode.GroupName, "target-project-id")]; ok {
		event.TargetProjectID, _ = strconv.Atoi(projectID)
	}
	return event
}
