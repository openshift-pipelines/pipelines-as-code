package pipelineascode

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	apipac "github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/keys"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/matcher"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/opscomments"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/triggertype"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider/github"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/resolve"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/secrets"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/templates"
	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"go.uber.org/zap"
)

func (p *PacRun) matchRepoPR(ctx context.Context) ([]matcher.Match, *v1alpha1.Repository, error) {
	repo, err := p.verifyRepoAndUser(ctx)
	if err != nil {
		return nil, nil, err
	}
	if repo == nil {
		return nil, nil, nil
	}

	if p.event.CancelPipelineRuns {
		return nil, repo, p.cancelPipelineRuns(ctx, repo)
	}

	matchedPRs, err := p.getPipelineRunsFromRepo(ctx, repo)
	if err != nil {
		return nil, repo, err
	}
	return matchedPRs, repo, nil
}

// verifyRepoAndUser verifies if the Repo CR exists for the Git Repository,
// if the user has permission to run CI  and also initialise provider client.
func (p *PacRun) verifyRepoAndUser(ctx context.Context) (*v1alpha1.Repository, error) {
	// Match the Event URL to a Repository URL,
	repo, err := matcher.MatchEventURLRepo(ctx, p.run, p.event, "")
	if err != nil {
		return nil, err
	}

	if repo == nil {
		msg := fmt.Sprintf("cannot find a repository match for %s", p.event.URL)
		p.eventEmitter.EmitMessage(nil, zap.WarnLevel, "RepositoryNamespaceMatch", msg)
		return nil, nil
	}

	p.logger = p.logger.With("namespace", repo.Namespace)
	p.vcx.SetLogger(p.logger)
	p.eventEmitter.SetLogger(p.logger)
	// If we have a git_provider field in repository spec, then get all the
	// information from there, including the webhook secret.
	// otherwise get the secret from the current ns (i.e: pipelines-as-code/openshift-pipelines.)
	//
	// TODO: there is going to be some improvements later we may want to do if
	// they are use cases for it :
	// allow webhook providers users to have a global webhook secret to be used,
	// so instead of having to specify their in Repo each time, they use a
	// shared one from pac.
	if p.event.InstallationID > 0 {
		p.event.Provider.WebhookSecret, _ = GetCurrentNSWebhookSecret(ctx, p.k8int, p.run)
	} else {
		err := SecretFromRepository(ctx, p.run, p.k8int, p.vcx.GetConfig(), p.event, repo, p.logger)
		if err != nil {
			return repo, err
		}
	}

	// validate payload  for webhook secret
	// we don't need to validate it in incoming since we already do this
	if p.event.EventType != "incoming" {
		if err := p.vcx.Validate(ctx, p.run, p.event); err != nil {
			// check that webhook secret has no /n or space into it
			if strings.ContainsAny(p.event.Provider.WebhookSecret, "\n ") {
				msg := `we have failed to validate the payload with the webhook secret,
it seems that we have detected a \n or a space at the end of your webhook secret, 
is that what you want? make sure you use -n when generating the secret, eg: echo -n secret|base64`
				p.eventEmitter.EmitMessage(repo, zap.ErrorLevel, "RepositorySecretValidation", msg)
			}
			return repo, fmt.Errorf("could not validate payload, check your webhook secret?: %w", err)
		}
	}

	// Set the client, we should error out if there is a problem with
	// token or secret or we won't be able to do much.
	err = p.vcx.SetClient(ctx, p.run, p.event, repo, p.eventEmitter)
	if err != nil {
		return repo, err
	}

	if p.event.InstallationID > 0 {
		token, err := github.ScopeTokenToListOfRepos(ctx, p.vcx, repo, p.run, p.event, p.eventEmitter, p.logger)
		if err != nil {
			return nil, err
		}
		// If Global and Repo level configurations are not provided then lets not override the provider token.
		if token != "" {
			p.event.Provider.Token = token
		}
	}

	// Get the SHA commit info, we want to get the URL and commit title
	err = p.vcx.GetCommitInfo(ctx, p.event)
	if err != nil {
		return repo, err
	}

	// Check if the submitter is allowed to run this.
	// on push we don't need to check the policy since the user has pushed to the repo so it has access to it.
	// on comment we skip it for now, we are going to check later on
	if p.event.TriggerTarget != triggertype.Push && p.event.EventType != opscomments.NoOpsCommentEventType.String() {
		if allowed, err := p.checkAccessOrErrror(ctx, repo, "via "+p.event.TriggerTarget.String()); !allowed {
			return nil, err
		}
	}
	return repo, nil
}

// getPipelineRunsFromRepo fetches pipelineruns from git repository and prepare them for creation.
func (p *PacRun) getPipelineRunsFromRepo(ctx context.Context, repo *v1alpha1.Repository) ([]matcher.Match, error) {
	provenance := "source"
	if repo.Spec.Settings != nil && repo.Spec.Settings.PipelineRunProvenance != "" {
		provenance = repo.Spec.Settings.PipelineRunProvenance
	}
	rawTemplates, err := p.vcx.GetTektonDir(ctx, p.event, tektonDir, provenance)
	if err != nil && strings.Contains(err.Error(), "error unmarshalling yaml file") {
		// make the error a bit more friendly for users who don't know what marshalling or intricacies of the yaml parser works
		errmsg := err.Error()
		errmsg = strings.ReplaceAll(errmsg, " error converting YAML to JSON: yaml:", "")
		errmsg = strings.ReplaceAll(errmsg, "unmarshalling", "while parsing the")
		return nil, fmt.Errorf(errmsg)
	}
	if err != nil || rawTemplates == "" {
		msg := fmt.Sprintf("cannot locate templates in %s/ directory for this repository in %s", tektonDir, p.event.HeadBranch)
		if err != nil {
			msg += fmt.Sprintf(" err: %s", err.Error())
		}
		p.eventEmitter.EmitMessage(nil, zap.InfoLevel, "RepositoryPipelineRunNotFound", msg)
		return nil, nil
	}

	// check for condition if need update the pipelinerun with regexp from the
	// "raw" pipelinerun string
	if msg, needUpdate := p.checkNeedUpdate(rawTemplates); needUpdate {
		p.eventEmitter.EmitMessage(repo, zap.InfoLevel, "RepositoryNeedUpdate", msg)
		return nil, fmt.Errorf(msg)
	}

	// This is for bitbucket
	if p.event.CloneURL == "" {
		p.event.AccountID = ""
	}

	// Replace those {{var}} placeholders user has in her template to the run.Info variable
	allTemplates := p.makeTemplate(ctx, repo, rawTemplates)

	types, err := resolve.ReadTektonTypes(ctx, p.logger, allTemplates)
	if err != nil {
		return nil, err
	}
	pipelineRuns := types.PipelineRuns
	if len(pipelineRuns) == 0 {
		msg := fmt.Sprintf("cannot locate templates in %s/ directory for this repository in %s", tektonDir, p.event.HeadBranch)
		p.eventEmitter.EmitMessage(nil, zap.InfoLevel, "RepositoryCannotLocatePipelineRun", msg)
		return nil, nil
	}

	pipelineRuns, err = resolve.MetadataResolve(pipelineRuns)
	if err != nil && len(pipelineRuns) == 0 {
		p.eventEmitter.EmitMessage(repo, zap.ErrorLevel, "FailedToResolvePipelineRunMetadata", err.Error())
		return nil, err
	}

	// Match the PipelineRun with annotation
	matchedPRs, err := matcher.MatchPipelinerunByAnnotation(ctx, p.logger, pipelineRuns, p.run, p.event, p.vcx)
	if err != nil {
		// Don't fail when you don't have a match between pipeline and annotations
		p.eventEmitter.EmitMessage(nil, zap.WarnLevel, "RepositoryNoMatch", err.Error())
		return nil, nil
	}

	// if the event is a comment event, but we don't have any match from the keys.OnComment then do the ACL checks again
	// we skipped previously so we can get the match from the event to the pipelineruns
	if p.event.EventType == opscomments.NoOpsCommentEventType.String() || p.event.EventType == opscomments.OnCommentEventType.String() {
		if allowed, err := p.checkAccessOrErrror(ctx, repo, "by gitops comment"); !allowed {
			return nil, err
		}
	}

	// if event type is incoming then filter out the pipelineruns related to incoming event
	pipelineRuns = matcher.MatchRunningPipelineRunForIncomingWebhook(p.event.EventType, p.event.TargetPipelineRun, pipelineRuns)
	if pipelineRuns == nil {
		msg := fmt.Sprintf("cannot find pipelinerun %s for matching an incoming event in this repository", p.event.TargetPipelineRun)
		p.eventEmitter.EmitMessage(repo, zap.InfoLevel, "RepositoryCannotLocatePipelineRunForIncomingEvent", msg)
		return nil, nil
	}

	// if /test command is used then filter out the pipelinerun
	pipelineRuns = filterRunningPipelineRunOnTargetTest(p.event.TargetTestPipelineRun, pipelineRuns)
	if pipelineRuns == nil {
		msg := fmt.Sprintf("cannot find pipelinerun %s in this repository", p.event.TargetTestPipelineRun)
		p.eventEmitter.EmitMessage(repo, zap.InfoLevel, "RepositoryCannotLocatePipelineRun", msg)
		return nil, nil
	}

	// finally resolve with fetching the remote tasks (if enabled)
	if p.run.Info.Pac.RemoteTasks {
		// only resolve on the matched pipelineruns
		types.PipelineRuns = nil
		for _, match := range matchedPRs {
			for pr := range pipelineRuns {
				if match.PipelineRun.GetName() == "" && match.PipelineRun.GetGenerateName() == pipelineRuns[pr].GenerateName ||
					match.PipelineRun.GetName() != "" && match.PipelineRun.GetName() == pipelineRuns[pr].Name {
					types.PipelineRuns = append(types.PipelineRuns, pipelineRuns[pr])
				}
			}
		}
		pipelineRuns, err = resolve.Resolve(ctx, p.run, p.logger, p.vcx, types, p.event, &resolve.Opts{
			GenerateName: true,
			RemoteTasks:  true,
		})
		if err != nil {
			p.eventEmitter.EmitMessage(repo, zap.ErrorLevel, "RepositoryFailedToMatch", fmt.Sprintf("failed to match pipelineRuns: %s", err.Error()))
			return nil, err
		}
	}

	err = changeSecret(pipelineRuns)
	if err != nil {
		return nil, err
	}
	matchedPRs, err = matcher.MatchPipelinerunByAnnotation(ctx, p.logger, pipelineRuns, p.run, p.event, p.vcx)
	if err != nil {
		// Don't fail when you don't have a match between pipeline and annotations
		p.eventEmitter.EmitMessage(nil, zap.WarnLevel, "RepositoryNoMatch", err.Error())
		return nil, nil
	}

	return matchedPRs, nil
}

func filterRunningPipelineRunOnTargetTest(testPipeline string, prs []*tektonv1.PipelineRun) []*tektonv1.PipelineRun {
	if testPipeline == "" {
		return prs
	}
	for _, pr := range prs {
		if prName, ok := pr.GetAnnotations()[apipac.OriginalPRName]; ok {
			if prName == testPipeline {
				return []*tektonv1.PipelineRun{pr}
			}
		}
	}
	return nil
}

// changeSecret we need to go in each pipelinerun,
// change the secret template variable with a random one as generated from GetBasicAuthSecretName
// and store in the annotations so we can create one delete after.
func changeSecret(prs []*tektonv1.PipelineRun) error {
	for k, p := range prs {
		b, err := json.Marshal(p)
		if err != nil {
			return err
		}

		name := secrets.GenerateBasicAuthSecretName()
		processed := templates.ReplacePlaceHoldersVariables(string(b), map[string]string{
			"git_auth_secret": name,
		}, nil, nil, map[string]interface{}{})

		var np *tektonv1.PipelineRun
		err = json.Unmarshal([]byte(processed), &np)
		if err != nil {
			return err
		}
		// don't crash when we don't have any annotations
		if np.Annotations == nil {
			np.Annotations = map[string]string{}
		}
		np.Annotations[apipac.GitAuthSecret] = name
		prs[k] = np
	}
	return nil
}

// checkNeedUpdate checks if the template needs an update form the user, try to
// match some patterns for some issues in a template to let the user know they need to
// update.
//
// We otherwise fail with a descriptive error message to the user (check run
// interface on as comment for other providers) on how to update.
//
// Checks are deprecated/removed to n+2 release of OSP.
func (p *PacRun) checkNeedUpdate(_ string) (string, bool) {
	return "", false
}

func (p *PacRun) checkAccessOrErrror(ctx context.Context, repo *v1alpha1.Repository, viamsg string) (bool, error) {
	allowed, err := p.vcx.IsAllowed(ctx, p.event)
	if err != nil {
		return false, err
	}
	if allowed {
		return true, nil
	}
	msg := fmt.Sprintf("User %s is not allowed to trigger CI %s on this repo.", p.event.Sender, viamsg)
	if p.event.AccountID != "" {
		msg = fmt.Sprintf("User: %s AccountID: %s is not allowed to trigger CI %s on this repo.", p.event.Sender, p.event.AccountID, viamsg)
	}
	p.eventEmitter.EmitMessage(repo, zap.InfoLevel, "RepositoryPermissionDenied", msg)
	status := provider.StatusOpts{
		Status:     "queued",
		Title:      "Pending approval",
		Conclusion: "pending",
		Text:       msg,
		DetailsURL: p.event.URL,
	}
	if err := p.vcx.CreateStatus(ctx, p.event, status); err != nil {
		return false, fmt.Errorf("failed to run create status, user is not allowed to run the CI:: %w", err)
	}
	return false, nil
}
