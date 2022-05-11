package pipelineascode

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"

	apipac "github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/kubeinteraction"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/matcher"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/resolve"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/templates"
	tektonv1beta1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
)

var gitAuthSecretAnnotation = "pipelinesascode.tekton.dev/git-auth-secret"

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

	rawTemplates := p.getAllPipelineRuns(ctx)
	// check for condition if need update the pipelinerun with regexp from the
	// "raw" pipelinerun string
	if msg, needUpdate := p.checkNeedUpdate(rawTemplates); needUpdate {
		p.logger.Info(msg)
		return nil, nil, fmt.Errorf(msg)
	}

	// Replace those {{var}} placeholders user has in her template to the run.Info variable
	allTemplates := templates.Process(p.event, rawTemplates)
	pipelineRuns, err := resolve.Resolve(ctx, p.run, p.logger, p.vcx, p.event, allTemplates, &resolve.Opts{
		GenerateName: true,
		RemoteTasks:  p.run.Info.Pac.RemoteTasks,
	})
	if pipelineRuns == nil || err != nil {
		msg := fmt.Sprintf("cannot locate templates in %s/ directory for this repository in %s", tektonDir, p.event.HeadBranch)
		if err != nil {
			msg += fmt.Sprintf("err: %s", err.Error())
		}
		p.logger.Info(msg)
		return nil, nil, nil
	}

	// if /test command is used then filter out the pipelinerun
	pipelineRuns = filterPipelineRun(p.event.TargetTestPipelineRun, pipelineRuns)
	if pipelineRuns == nil {
		p.logger.Info(fmt.Sprintf("cannot find pipelinerun %s in this repository", p.event.TargetTestPipelineRun))
		return nil, nil, nil
	}

	err = changeSecret(pipelineRuns)
	if err != nil {
		return nil, nil, err
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

func filterPipelineRun(testPipeline string, prs []*tektonv1beta1.PipelineRun) []*tektonv1beta1.PipelineRun {
	if testPipeline == "" {
		return prs
	}
	for _, pr := range prs {
		if prName, ok := pr.GetLabels()[filepath.Join(apipac.GroupName, "original-prname")]; ok {
			if prName == testPipeline {
				return []*tektonv1beta1.PipelineRun{pr}
			}
		}
	}
	return nil
}

// changeSecret we need to go in each pipelinerun,
// change the secret template variable with a random one as generated from GetBasicAuthSecretName and store in in the
// annotations so we can create one delete after.
func changeSecret(prs []*tektonv1beta1.PipelineRun) error {
	for k, p := range prs {
		b, err := json.Marshal(p)
		if err != nil {
			return err
		}

		name := kubeinteraction.GetBasicAuthSecretName()
		processed := templates.ReplacePlaceHoldersVariables(string(b), map[string]string{
			"git_auth_secret": name,
		})

		var np *tektonv1beta1.PipelineRun
		err = json.Unmarshal([]byte(processed), &np)
		if err != nil {
			return err
		}
		np.Annotations[gitAuthSecretAnnotation] = name
		prs[k] = np
	}
	return nil
}

// checkNeedUpdate using regexp, try to match some pattern for some issue in PR
// to let the user know they need to update. or otherwise we will fail.
// checks are depreacted/removed to n+1 release of OSP.
// each check should give a good error message on how to update.
func (p *PacRun) checkNeedUpdate(tmpl string) (string, bool) {
	// nolint: gosec
	oldBasicAuthSecretName := `\W*secretName: "pac-git-basic-auth-{{repo_owner}}-{{repo_name}}"`
	if matched, _ := regexp.MatchString(oldBasicAuthSecretName, tmpl); matched {
		return `!Update needed! you have a old basic auth secret name, you need to modify your pipelinerun and change the string "secret: pac-git-basic-auth-{{repo_owner}}-{{repo_name}}" to "secret: {{ git_auth_secret }}"`, true
	}
	return "", false
}

func (p *PacRun) getAllPipelineRuns(ctx context.Context) string {
	// Get everything in tekton directory
	allTemplates, err := p.vcx.GetTektonDir(ctx, p.event, tektonDir)
	if allTemplates == "" || err != nil {
		if err != nil {
			p.logger.Errorw("there was an error getting all files in tektondir: %w", err)
		}
	}
	return allTemplates
}
