package action

import (
	"context"
	"encoding/json"
	"fmt"

	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
)

func PatchPipelineRun(ctx context.Context, logger *zap.SugaredLogger, whatPatching string, tekton versioned.Interface, pr *tektonv1.PipelineRun, mergePatch map[string]interface{}) (*tektonv1.PipelineRun, error) {
	if pr == nil {
		return nil, nil
	}
	var patchedPR *tektonv1.PipelineRun
	// double the retry; see https://issues.redhat.com/browse/SRVKP-3134
	doubleRetry := retry.DefaultRetry
	doubleRetry.Steps *= 2
	doubleRetry.Duration *= 2
	doubleRetry.Factor *= 2
	doubleRetry.Jitter *= 2
	err := retry.RetryOnConflict(doubleRetry, func() error {
		patch, err := json.Marshal(mergePatch)
		if err != nil {
			return err
		}
		patchedPR, err = tekton.TektonV1().PipelineRuns(pr.GetNamespace()).Patch(ctx, pr.GetName(), types.MergePatchType, patch, metav1.PatchOptions{})
		if err != nil {
			logger.Infof("could not patch Pipelinerun with %v, retrying %v/%v: %v", whatPatching, pr.GetNamespace(), pr.GetName(), err)
			return err
		}
		logger.Infof("patched pipelinerun with %v: %v/%v", whatPatching, patchedPR.Namespace, patchedPR.Name)
		return nil
	})
	if err != nil {
		// return the original PipelineRun, let the caller decide what to do with it after the error is processed
		return pr, fmt.Errorf("failed to patch pipelinerun %v/%v with %v: %w", pr.Namespace, whatPatching, pr.Name, err)
	}
	return patchedPR, nil
}
