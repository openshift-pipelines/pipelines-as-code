package formatting

import (
	corev1 "k8s.io/api/core/v1"
	knative1 "knative.dev/pkg/apis/duck/v1beta1"
)

const nonAttributedStr = "---"

func ConditionEmoji(c knative1.Conditions) string {
	var status string
	if len(c) == 0 {
		return nonAttributedStr
	}

	// TODO: there is other weird errors we need to handle.

	switch c[0].Status {
	case corev1.ConditionFalse:
		return "❌ Failed"
	case corev1.ConditionTrue:
		return "✅ Succeeded"
	case corev1.ConditionUnknown:
		return "🏃 Running"
	}

	return status
}
