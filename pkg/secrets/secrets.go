package secrets

import (
	"context"
	"fmt"
	"strings"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/kubeinteraction"
	ktypes "github.com/openshift-pipelines/pipelines-as-code/pkg/secrets/types"
	tektonv1beta1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
)

const leakedReplacement = "*****"

// GetSecretsAttachedToPipelineRun get all secrets attached to a PipelineRun and grab their values
func GetSecretsAttachedToPipelineRun(ctx context.Context, k kubeinteraction.Interface, pr *tektonv1beta1.PipelineRun) []ktypes.SecretValue {
	ret := []ktypes.SecretValue{}
	// check if pipelineRef is defined or exist
	if pr.Spec.PipelineSpec == nil {
		return ret
	}

	for _, pt := range append(pr.Spec.PipelineSpec.Finally, pr.Spec.PipelineSpec.Tasks...) {
		for _, step := range pt.TaskSpec.Steps {
			for _, ev := range step.Env {
				if ev.ValueFrom == nil {
					continue
				}
				if ev.ValueFrom.SecretKeyRef == nil {
					continue
				}
				secretValue, err := k.GetSecret(ctx, ktypes.GetSecretOpt{
					Name:      ev.ValueFrom.SecretKeyRef.Name,
					Key:       ev.ValueFrom.SecretKeyRef.Key,
					Namespace: pr.GetNamespace(),
				})
				// that really should not happen but let's go on and continue if that's the case
				if err != nil {
					continue
				}
				keyv := fmt.Sprintf("%s-%s", ev.ValueFrom.SecretKeyRef.Name, ev.ValueFrom.SecretKeyRef.Key)
				there := false
				for _, value := range ret {
					if value.Name == keyv {
						there = true
					}
				}
				if !there {
					ret = append(ret, ktypes.SecretValue{
						Name:  keyv,
						Value: secretValue,
					})
				}
			}
		}
	}

	return ret
}

// ReplaceSecretsInText this will take a text snippet and hide the leaked secret
func ReplaceSecretsInText(text string, values []ktypes.SecretValue) string {
	for _, sv := range values {
		text = strings.ReplaceAll(text, sv.Value, leakedReplacement)
	}
	return text
}
