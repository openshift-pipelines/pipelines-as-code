package reconciler

import (
	"context"
	"fmt"
	"strconv"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/keys"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/formatting"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider/bitbucketcloud"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider/bitbucketserver"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider/gitea"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider/github"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider/gitlab"
	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"go.uber.org/zap"
)

func (r *Reconciler) detectProvider(ctx context.Context, logger *zap.SugaredLogger, pr *tektonv1.PipelineRun) (provider.Interface, *info.Event, error) {
	gitProvider, ok := pr.GetAnnotations()[keys.GitProvider]
	if !ok {
		return nil, nil, fmt.Errorf("failed to detect git provider for pipleinerun %s : git-provider label not found", pr.GetName())
	}

	event := buildEventFromPipelineRun(pr)

	var provider provider.Interface
	switch gitProvider {
	case "github", "github-enterprise":
		gh := github.New()
		gh.Logger = logger
		gh.Run = r.run
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
	case "gitea":
		provider = &gitea.Provider{}
	default:
		return nil, nil, fmt.Errorf("failed to detect provider for pipelinerun: %s : unknown provider", pr.GetName())
	}
	provider.SetLogger(logger)
	return provider, event, nil
}

func buildEventFromPipelineRun(pr *tektonv1.PipelineRun) *info.Event {
	event := info.NewEvent()

	prAnno := pr.GetAnnotations()

	event.URL = prAnno[keys.RepoURL]
	// it's safer to get repo, org from repo.url since we have to remove the / and other chars in labels which drops
	// the SubPath that gitlab is using.
	repo, org, _ := formatting.GetRepoOwnerSplitted(event.URL)
	event.Organization = repo
	event.Repository = org
	event.EventType = prAnno[keys.EventType]
	event.BaseBranch = prAnno[keys.Branch]
	event.SHA = prAnno[keys.SHA]

	event.SHATitle = prAnno[keys.ShaTitle]
	event.SHAURL = prAnno[keys.ShaURL]

	prNumber := prAnno[keys.PullRequest]
	if prNumber != "" {
		event.PullRequestNumber, _ = strconv.Atoi(prNumber)
	}

	// GitHub
	if installationID, ok := prAnno[keys.InstallationID]; ok {
		id, _ := strconv.Atoi(installationID)
		event.InstallationID = int64(id)
	}
	if gheURL, ok := prAnno[keys.GHEURL]; ok {
		event.GHEURL = gheURL
	}

	// Gitlab
	if projectID, ok := prAnno[keys.SourceProjectID]; ok {
		event.SourceProjectID, _ = strconv.Atoi(projectID)
	}
	if projectID, ok := prAnno[keys.TargetProjectID]; ok {
		event.TargetProjectID, _ = strconv.Atoi(projectID)
	}
	return event
}
