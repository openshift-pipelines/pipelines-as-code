package action

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
)

func PatchPipelineRun(ctx context.Context, logger *zap.SugaredLogger, tekton versioned.Interface, pr *v1beta1.PipelineRun, mergePatch map[string]interface{}) error {
	if pr == nil {
		return nil
	}
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		patch, err := json.Marshal(mergePatch)
		if err != nil {
			return err
		}
		patchedPR, err := tekton.TektonV1beta1().PipelineRuns(pr.GetNamespace()).Patch(ctx, pr.GetName(), types.MergePatchType, patch, metav1.PatchOptions{})
		if err != nil {
			logger.Infof("could not patch Pipelinerun, retrying %v/%v: %v", pr.GetNamespace(), pr.GetName(), err)
			return err
		}
		logger.Infof("patched pipelinerun: %v/%v", patchedPR.Namespace, patchedPR.Name)
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to patch pipelinerun %v/%v: %w", pr.Namespace, pr.Name, err)
	}
	return nil
}
