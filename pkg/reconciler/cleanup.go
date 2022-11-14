package reconciler

import (
	"context"
	"fmt"
	"strconv"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/keys"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"go.uber.org/zap"
)

func (r *Reconciler) cleanupSecrets(ctx context.Context, logger *zap.SugaredLogger, repo *v1alpha1.Repository, pr *v1beta1.PipelineRun) error {
	var gitAuthSecretName string
	if annotation, ok := pr.Annotations[keys.GitAuthSecret]; ok {
		gitAuthSecretName = annotation
	} else {
		return fmt.Errorf("cannot get annotation %s as set on PR", keys.GitAuthSecret)
	}

	err := r.kinteract.DeleteBasicAuthSecret(ctx, logger, repo.GetNamespace(), gitAuthSecretName)
	if err != nil {
		return fmt.Errorf("deleting basic auth secret has failed: %w ", err)
	}
	return nil
}

func (r *Reconciler) cleanupPipelineRuns(ctx context.Context, logger *zap.SugaredLogger, repo *v1alpha1.Repository, pr *v1beta1.PipelineRun) error {
	keepMaxPipeline, ok := pr.Annotations[keys.MaxKeepRuns]
	if ok {
		max, err := strconv.Atoi(keepMaxPipeline)
		if err != nil {
			return err
		}
		// if annotation value is more than max limit defined in config then use from config
		if r.run.Info.Pac.MaxKeepRunsUpperLimit > 0 && max > r.run.Info.Pac.MaxKeepRunsUpperLimit {
			logger.Infof("max-keep-run value in annotation (%v) is more than max-keep-run-upper-limit (%v), so using upper-limit", max, r.run.Info.Pac.MaxKeepRunsUpperLimit)
			max = r.run.Info.Pac.MaxKeepRunsUpperLimit
		}
		err = r.kinteract.CleanupPipelines(ctx, logger, repo, pr, max)
		if err != nil {
			return err
		}
		return nil
	}

	// if annotation is not defined but default max-keep-run value is defined then use that
	if r.run.Info.Pac.DefaultMaxKeepRuns > 0 {
		max := r.run.Info.Pac.DefaultMaxKeepRuns

		err := r.kinteract.CleanupPipelines(ctx, logger, repo, pr, max)
		if err != nil {
			return err
		}
	}
	return nil
}
