package podlogs

import (
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"gotest.tools/v3/assert"
)

func TestControllerLabelSelectors(t *testing.T) {
	t.Run("default controller", func(t *testing.T) {
		got := controllerLabelSelectors("controller")
		assert.DeepEqual(t, got, []string{
			"app.kubernetes.io/name=controller",
			"app.kubernetes.io/name=pipelines-as-code-controller",
			"app.kubernetes.io/component=controller,app.kubernetes.io/part-of=pipelines-as-code",
		})
	})

	t.Run("second controller", func(t *testing.T) {
		got := controllerLabelSelectors("ghe-controller")
		assert.DeepEqual(t, got, []string{
			"app.kubernetes.io/name=ghe-controller",
			"app.kubernetes.io/component=controller,app.kubernetes.io/part-of=pipelines-as-code",
		})
	})
}

func TestControllerContainerNames(t *testing.T) {
	t.Run("default controller", func(t *testing.T) {
		got := controllerContainerNames("controller")
		assert.DeepEqual(t, got, []string{"pac-controller", "controller", "pipelines-as-code-controller"})
	})

	t.Run("second controller", func(t *testing.T) {
		got := controllerContainerNames("ghe-controller")
		assert.DeepEqual(t, got, []string{"ghe-controller"})
	})
}

func TestFindPodWithContainer(t *testing.T) {
	pods := []v1.Pod{
		{
			Spec: v1.PodSpec{
				Containers: []v1.Container{{Name: "ghe-controller"}},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "pipelines-as-code-controller-abc"},
			Spec: v1.PodSpec{
				Containers: []v1.Container{{Name: "pac-controller"}},
			},
		},
	}

	t.Run("select matching pod by container name", func(t *testing.T) {
		pod, err := findPodWithContainer(pods, "pac-controller")
		assert.NilError(t, err)
		assert.Equal(t, pod.Name, "pipelines-as-code-controller-abc")
	})

	t.Run("returns detailed error when container not found", func(t *testing.T) {
		_, err := findPodWithContainer(pods, "controller")
		assert.ErrorContains(t, err, `container "controller" is not present in any pod`)
		assert.ErrorContains(t, err, "pipelines-as-code-controller-abc[pac-controller]")
	})
}
