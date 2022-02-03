package sync

import (
	"context"
	"encoding/json"
	"time"

	"github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"knative.dev/pkg/apis"
)

func syncer(m *Manager, lockKey string, pipelineCS versioned.Interface) error {
	for {
		sema := m.syncLockMap[lockKey]

		// check if current running pr is complete
		for _, pr := range sema.getCurrentHolders() {
			// decode pipelineRun namespace/name
			prNs, prName := DecodeHolderKey(pr)

			pipelineRun, err := pipelineCS.TektonV1beta1().PipelineRuns(prNs).Get(context.Background(), prName, v1.GetOptions{})
			if err != nil {
				if errors.IsNotFound(err) {
					m.logger.Infof("pipelineRun not found, releasing %v for lock %v", pr, lockKey)
					m.Release(lockKey, pr)
					continue
				}
				m.logger.Infof("failed to get pipelinerun %v : %v", pr, err)
				return err
			}

			// check if is completed, if yes then remove from holder Q
			succeeded := pipelineRun.Status.GetCondition(apis.ConditionSucceeded)
			if succeeded == nil {
				continue
			}
			if succeeded.Status == corev1.ConditionTrue || succeeded.Status == corev1.ConditionFalse {
				// pipelinerun has been succeeded so can be removed from Q
				m.Release(lockKey, pr)
			}
		}

		// after checking all running now check if the limit has been reached
		// or start pipelinerun from pending queue

		limit := sema.getLimit()

		if len(sema.getCurrentPending()) == 0 && len(sema.getCurrentHolders()) == 0 {
			// all pipelinerun are completed
			// we can return
			return nil
		}

		for len(sema.getCurrentHolders()) < limit {
			if len(sema.getCurrentPending()) == 0 {
				break
			}

			// move the top most from pending queue to running
			ready := sema.acquireForLatest()
			if ready == "" {
				break
			}

			prNs, prName := DecodeHolderKey(ready)

			// fetch and update pipelinerun by removing pending
			tobeUpdated, err := pipelineCS.TektonV1beta1().PipelineRuns(prNs).Get(context.Background(), prName, v1.GetOptions{})
			if err != nil {
				if errors.IsNotFound(err) {
					m.logger.Infof("pipelineRun not found, removing from queue %v ", ready)
					m.Remove(lockKey, ready)
					continue
				}
				m.logger.Infof("failed to get pipelinerun %v : %v", ready, err)
				return err
			}

			// remove pending status from spec
			mergePatch := map[string]interface{}{
				"spec": map[string]interface{}{
					"status": "",
				},
			}

			patch, err := json.Marshal(mergePatch)
			if err != nil {
				m.logger.Infof("failed to marshal jsong")
			}

			patcher := pipelineCS.TektonV1beta1().PipelineRuns(tobeUpdated.Namespace)
			_, err = patcher.Patch(context.Background(), tobeUpdated.Name, types.MergePatchType, patch, v1.PatchOptions{})
			if err != nil {
				m.logger.Infof("failed to patch pipelinerun %v : %v", tobeUpdated.Name, err)
				return err
			}
		}

		time.Sleep(10 * time.Second)
	}
}
