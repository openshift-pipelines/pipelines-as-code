package sync

import (
	"context"
	"time"

	"github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

			// move the top most from pending queue to running
			ready := sema.acquireForLatest()

			prNs, prName := DecodeHolderKey(ready)

			// fetch and update pipelinerun by removing pending
			tobeUpdated, err := pipelineCS.TektonV1beta1().PipelineRuns(prNs).Get(context.Background(), prName, v1.GetOptions{})
			if err != nil {
				m.logger.Infof("failed to get pipelinerun %v : %v", ready, err)
				return err
			}

			// remove pending status
			tobeUpdated.Spec.Status = ""

			_, err = pipelineCS.TektonV1beta1().PipelineRuns(prNs).Update(context.Background(), tobeUpdated, v1.UpdateOptions{})
			if err != nil {
				m.logger.Infof("failed to update pipelinerun %v : %v", tobeUpdated.Name, err)
				return err
			}
		}

		time.Sleep(10 * time.Second)
	}
}
