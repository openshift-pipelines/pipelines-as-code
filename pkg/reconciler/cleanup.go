package reconciler

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"go.uber.org/zap"
)

var gitAuthSecretAnnotation = filepath.Join(pipelinesascode.GroupName, "git-auth-secret")

func (r *Reconciler) cleanupSecrets(ctx context.Context, logger *zap.SugaredLogger, repo *v1alpha1.Repository, pr *v1beta1.PipelineRun) error {
	var gitAuthSecretName string
	if annotation, ok := pr.Annotations[gitAuthSecretAnnotation]; ok {
		gitAuthSecretName = annotation
	} else {
		return fmt.Errorf("cannot get annotation %s as set on PR", gitAuthSecretAnnotation)
	}

	err := r.kinteract.DeleteBasicAuthSecret(ctx, logger, repo.GetNamespace(), gitAuthSecretName)
	if err != nil {
		return fmt.Errorf("deleting basic auth secret has failed: %w ", err)
	}
	return nil
}

func (r *Reconciler) cleanupPipelineRuns(ctx context.Context, logger *zap.SugaredLogger, repo *v1alpha1.Repository, pr *v1beta1.PipelineRun) error {
	keepMaxPipeline, ok := pr.Annotations[filepath.Join(pipelinesascode.GroupName, "max-keep-runs")]
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
	return nil
}
