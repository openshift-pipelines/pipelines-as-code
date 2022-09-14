package formatting

import (
	"strings"
	"testing"

	"gotest.tools/v3/assert"
	corev1 "k8s.io/api/core/v1"
	duckv1beta1 "knative.dev/pkg/apis/duck/v1beta1"
)

func TestConditionEmoji(t *testing.T) {
	tests := []struct {
		name      string
		condition duckv1beta1.Conditions
		substr    string
	}{
		{
			name: "failed",
			condition: duckv1beta1.Conditions{
				{
					Status: corev1.ConditionFalse,
				},
			},
			substr: "Failed",
		},
		{
			name: "success",
			condition: duckv1beta1.Conditions{
				{
					Status: corev1.ConditionTrue,
				},
			},
			substr: "Succeeded",
		},
		{
			name: "Running",
			condition: duckv1beta1.Conditions{
				{
					Status: corev1.ConditionUnknown,
				},
			},
			substr: "Running",
		},
		{
			name:      "None",
			condition: duckv1beta1.Conditions{},
			substr:    nonAttributedStr,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ConditionEmoji(tt.condition)
			assert.Assert(t, strings.Contains(got, tt.substr))
		})
	}
}

func TestSkipEmoji(t *testing.T) {
	got := ConditionSad(
		duckv1beta1.Conditions{{Status: corev1.ConditionTrue}})
	assert.Assert(t, !strings.Contains(got, "✅"))
}
