package pipelineascode

import (
	"context"
	"fmt"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/matcher"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/resolve"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/templates"
	tektonv1beta1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
)

// matchRepoPR matches the repo and the PRs from the event
func (p *PacRun) matchRepoPR(ctx context.Context) ([]matcher.Match, *v1alpha1.Repository, error) {
	// Match the Event URL to a Repository URL,
	repo, err := matcher.MatchEventURLRepo(ctx, p.run, p.event, "")
	if err != nil {
		return nil, nil, err
	}

	if repo == nil {
		if p.event.Provider.Token == "" {
			p.logger.Warn("cannot set status since no token has been set")
			return nil, nil, nil
		}
		msg := fmt.Sprintf("cannot find a namespace match for %s", p.event.URL)
		p.logger.Warn(msg)

		status := provider.StatusOpts{
			Status:     "completed",
			Conclusion: "skipped",
			Text:       msg,
			DetailsURL: "https://tenor.com/search/sad-cat-gifs",
		}
		if err := p.vcx.CreateStatus(ctx, p.event, p.run.Info.Pac, status); err != nil {
			return nil, nil, fmt.Errorf("failed to run create status on repo not found: %w", err)
		}
		return nil, nil, nil
	}

	// If we have a git_provider field in repository spec, then get all the
	// information from there, including the webhook secret.
	// otherwise get the secret from the current ns (i.e: pipelines-as-code/openshift-pipelines.)
	//
	// TODO: there is going to be some improvements later we may want to do if
	// they are use cases for it :
	// allow webhook providers users to have a global webhook secret to be used,
	// so instead of having to specify their in Repo each time, they use a
	// shared one from pac.
	if repo.Spec.GitProvider != nil {
		err := secretFromRepository(ctx, p.run, p.k8int, p.vcx.GetConfig(), p.event, repo, p.logger)
		if err != nil {
			return nil, nil, err
		}
	} else {
		p.event.Provider.WebhookSecret, _ = getCurrentNSWebhookSecret(ctx, p.k8int)
	}
	if err := p.vcx.Validate(ctx, p.run, p.event); err != nil {
		return nil, nil, fmt.Errorf("could not validate payload, check your webhook secret?: %w", err)
	}

	// Set the client, we should error out if there is a problem with
	// token or secret or we won't be able to do much.
	err = p.vcx.SetClient(ctx, p.event)
	if err != nil {
		return nil, nil, err
	}

	// Get the SHA commit info, we want to get the URL and commit title
	err = p.vcx.GetCommitInfo(ctx, p.event)
	if err != nil {
		return nil, nil, err
	}

	// Check if the submitter is allowed to run this.
	allowed, err := p.vcx.IsAllowed(ctx, p.event)
	if err != nil {
		return nil, nil, err
	}

	if !allowed {
		msg := fmt.Sprintf("User %s is not allowed to run CI on this repo.", p.event.Sender)
		p.logger.Info(msg)
		if p.event.AccountID != "" {
			msg = fmt.Sprintf("User: %s AccountID: %s is not allowed to run CI on this repo.", p.event.Sender, p.event.AccountID)
		}
		status := provider.StatusOpts{
			Status:     "completed",
			Conclusion: "skipped",
			Text:       msg,
			DetailsURL: "https://tenor.com/search/police-cat-gifs",
		}
		if err := p.vcx.CreateStatus(ctx, p.event, p.run.Info.Pac, status); err != nil {
			return nil, nil, fmt.Errorf("failed to run create status, user is not allowed to run: %w", err)
		}
		return nil, nil, nil
	}

	pipelineRuns, err := p.getAllPipelineRuns(ctx)
	if err != nil {
		return nil, nil, err
	}

	if pipelineRuns == nil {
		msg := fmt.Sprintf("cannot locate templates in %s/ directory for this repository in %s", tektonDir, p.event.HeadBranch)
		p.logger.Info(msg)
		return nil, nil, nil
	}

	// Match the PipelineRun with annotation
	matchedPRs, err := matcher.MatchPipelinerunByAnnotation(ctx, p.logger, pipelineRuns, p.run, p.event)
	if err != nil {
		// Don't fail when you don't have a match between pipeline and annotations
		// TODO: better reporting
		p.logger.Warn(err.Error())
		return nil, nil, nil
	}

	return matchedPRs, repo, nil
}

func (p *PacRun) getAllPipelineRuns(ctx context.Context) ([]*tektonv1beta1.PipelineRun, error) {
	// Get everything in tekton directory
	allTemplates, err := p.vcx.GetTektonDir(ctx, p.event, tektonDir)
	if allTemplates == "" || err != nil {
		// nolint: nilerr
		return nil, nil
	}

	// Replace those {{var}} placeholders user has in her template to the run.Info variable
	allTemplates = templates.Process(p.event, allTemplates)

	// Merge everything (i.e: tasks/pipeline etc..) as a single pipelinerun
	return resolve.Resolve(ctx, p.run, p.logger, p.vcx, p.event, allTemplates, &resolve.Opts{
		GenerateName: true,
		RemoteTasks:  p.run.Info.Pac.RemoteTasks,
	})
}
