package secrets

import (
	"testing"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/secrets/types"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/test/kubernetestint"
	tektonv1beta1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"gotest.tools/v3/assert"
	corev1 "k8s.io/api/core/v1"
	rtesting "knative.dev/pkg/reconciler/testing"
)

func TestGetSecretsAttachedToPipelineRun(t *testing.T) {
	samplePr := tektonv1beta1.PipelineRun{
		Spec: tektonv1beta1.PipelineRunSpec{
			PipelineSpec: &tektonv1beta1.PipelineSpec{
				Tasks: []tektonv1beta1.PipelineTask{
					{
						TaskSpec: &tektonv1beta1.EmbeddedTask{
							TaskSpec: tektonv1beta1.TaskSpec{
								Steps: []tektonv1beta1.Step{
									{
										Env: []corev1.EnvVar{},
									},
								},
							},
						},
					},
					{
						TaskRef: &tektonv1beta1.TaskRef{
							Name: "git-clone",
							Kind: "ClusterTask",
						},
					},
				},
			},
		},
	}
	tests := []struct {
		name           string
		pr             tektonv1beta1.PipelineRun
		envs           []corev1.EnvVar
		secretsFake    map[string]string
		results        []types.SecretValue
		nosecretKeyRef bool
	}{
		{
			name: "get secrets",
			pr:   samplePr,
			secretsFake: map[string]string{
				"secret1": "uno",
				"secret2": "segundo",
			},
			envs: []corev1.EnvVar{
				{
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							Key: "key1",
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "secret1",
							},
						},
					},
				},
				{
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							Key: "key2",
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "secret2",
							},
						},
					},
				},
			},
			results: []types.SecretValue{
				{
					Name:  "secret1-key1",
					Value: "uno",
				},
				{
					Name:  "secret2-key2",
					Value: "segundo",
				},
			},
		},
		{
			name: "remove doublons",
			pr:   samplePr,
			secretsFake: map[string]string{
				"secret1": "uno",
			},
			envs: []corev1.EnvVar{
				{
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							Key: "key1",
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "secret1",
							},
						},
					},
				},
				{
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							Key: "key1",
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "secret1",
							},
						},
					},
				},
			},
			results: []types.SecretValue{
				{
					Name:  "secret1-key1",
					Value: "uno",
				},
			},
		},
		{
			name: "no secrets skip",
			secretsFake: map[string]string{
				"secret1": "uno",
			},
			pr: samplePr,
			envs: []corev1.EnvVar{
				{
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							Key: "key1",
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "secret1",
							},
						},
					},
				},
				{
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							Key: "key2",
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "secret2",
							},
						},
					},
				},
			},
			results: []types.SecretValue{
				{
					Name:  "secret1-key1",
					Value: "uno",
				},
			},
		},
		{
			name:           "no secret key ref skip",
			pr:             samplePr,
			nosecretKeyRef: true,
			results:        []types.SecretValue{},
			envs: []corev1.EnvVar{
				{
					ValueFrom: &corev1.EnvVarSource{
						ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "config",
							},
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.pr.Spec.PipelineSpec.Tasks[0].TaskSpec.TaskSpec.Steps[0].Env = tt.envs
			k := &kubernetestint.KinterfaceTest{
				GetSecretResult: tt.secretsFake,
			}
			ctx, _ := rtesting.SetupFakeContext(t)
			ret := GetSecretsAttachedToPipelineRun(ctx, k, &tt.pr)
			assert.DeepEqual(t, tt.results, ret)
		})
	}
}

func TestReplaceSecretsInText(t *testing.T) {
	tests := []struct {
		name, text, result string
		values             []types.SecretValue
	}{
		{
			name:   "replace secrets in text",
			text:   "I am beautiful",
			result: "I am *****",
			values: []types.SecretValue{
				{
					Name:  "beautiful-secret",
					Value: "beautiful",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ret := ReplaceSecretsInText(tt.text, tt.values)
			assert.Equal(t, ret, tt.result)
		})
	}
}
