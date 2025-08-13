package pipelineascode

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	pacerrors "github.com/openshift-pipelines/pipelines-as-code/pkg/errors"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider"
	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"go.uber.org/zap"
)

const (
	tektonDirMissingError = ".tekton/ directory doesn't exist in repository's root directory"
)

var regexpIgnoreErrors = regexp.MustCompile(`.*no kind.*is registered for version.*in scheme.*`)

func (p *PacRun) checkAccessOrError(ctx context.Context, repo *v1alpha1.Repository, status provider.StatusOpts, viamsg string) (bool, error) {
	allowed, err := p.vcx.IsAllowed(ctx, p.event)
	if err != nil {
		return false, fmt.Errorf("unable to verify event authorization: %w", err)
	}
	if allowed {
		return true, nil
	}
	msg := fmt.Sprintf("User %s is not allowed to trigger CI %s in this repo.", p.event.Sender, viamsg)
	if p.event.AccountID != "" {
		msg = fmt.Sprintf("User: %s AccountID: %s is not allowed to trigger CI %s in this repo.", p.event.Sender, p.event.AccountID, viamsg)
	}
	p.eventEmitter.EmitMessage(repo, zap.InfoLevel, "RepositoryPermissionDenied", msg)
	status.Text = msg

	if err := p.vcx.CreateStatus(ctx, p.event, status); err != nil {
		return false, fmt.Errorf("failed to run create status, user is not allowed to run the CI:: %w", err)
	}
	return false, nil
}

// reportValidationErrors reports validation errors found in PipelineRuns by:
// 1. Creating error messages for each validation error
// 2. Emitting error messages to the event system
// 3. Creating a markdown formatted comment on the repository with all errors.
func (p *PacRun) reportValidationErrors(ctx context.Context, repo *v1alpha1.Repository, validationErrors []*pacerrors.PacYamlValidations) {
	errorRows := make([]string, 0, len(validationErrors))
	for _, err := range validationErrors {
		// if the error is a TektonConversionError, we don't want to report it since it may be a file that is not a tekton resource
		// and we don't want to report it as a validation error.
		if !regexpIgnoreErrors.MatchString(err.Err.Error()) && (strings.HasPrefix(err.Schema, tektonv1.SchemeGroupVersion.Group) || err.Schema == pacerrors.GenericBadYAMLValidation) {
			errorRows = append(errorRows, fmt.Sprintf("| %s | `%s` |", err.Name, err.Err.Error()))
		}
		p.eventEmitter.EmitMessage(repo, zap.ErrorLevel, "PipelineRunValidationErrors",
			fmt.Sprintf("cannot read the PipelineRun: %s, error: %s", err.Name, err.Err.Error()))
	}
	if len(errorRows) == 0 {
		return
	}
	markdownErrMessage := fmt.Sprintf(`%s
%s`, provider.ValidationErrorTemplate, strings.Join(errorRows, "\n"))
	if err := p.vcx.CreateComment(ctx, p.event, markdownErrMessage, provider.ValidationErrorTemplate); err != nil {
		p.eventEmitter.EmitMessage(repo, zap.ErrorLevel, "PipelineRunCommentCreationError",
			fmt.Sprintf("failed to create comment: %s", err.Error()))
	}
}
