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

// PatchPipelineRun patches a Tekton PipelineRun resource with the provided merge patch.
// It retries the patch operation on conflict, doubling the default retry parameters.
//
// Parameters:
// - ctx: The context for the patch operation.
// - logger: A SugaredLogger instance for logging information.
// - whatPatching: A string describing what is being patched, used for logging purposes.
// - tekton: A Tekton client interface for interacting with Tekton resources.
// - pr: The PipelineRun resource to be patched. If nil, the function returns nil.
// - mergePatch: A map representing the JSON merge patch to apply to the PipelineRun.
//
// Returns:
// - *tektonv1.PipelineRun: The patched PipelineRun resource, or the original PipelineRun if an error occurs.
// - error: An error if the patch operation fails after retries, or nil if successful.
//
// The function doubles the default retry parameters (steps, duration, factor, jitter) to handle conflicts more robustly.
// If the patch operation fails after retries, the original PipelineRun is returned along with the error.
func PatchPipelineRun(ctx context.Context, logger *zap.SugaredLogger, whatPatching string, tekton versioned.Interface, pr *tektonv1.PipelineRun, mergePatch map[string]any) (*tektonv1.PipelineRun, error) {
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
