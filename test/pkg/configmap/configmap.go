package configmap

import (
	"context"
	"testing"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"golang.org/x/exp/maps"
	"gotest.tools/v3/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func ChangeGlobalConfig(ctx context.Context, t *testing.T, runcnx *params.Run, configMapName string, data map[string]string) func() {
	ns := info.GetNS(ctx)
	// grab the old configmap content
	origCfgmap, err := runcnx.Clients.Kube.CoreV1().ConfigMaps(ns).Get(ctx, configMapName, metav1.GetOptions{})
	assert.NilError(t, err)
	newData := map[string]string{}
	maps.Copy(newData, origCfgmap.Data)
	maps.Copy(newData, data)
	newConfigMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configMapName,
			Namespace: ns,
		},
		Data: newData,
	}
	_, err = runcnx.Clients.Kube.CoreV1().ConfigMaps(ns).Update(ctx, newConfigMap, metav1.UpdateOptions{})
	assert.NilError(t, err)
	return func() {
		orgNew := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      configMapName,
				Namespace: ns,
			},
			Data: origCfgmap.Data,
		}
		_, err := runcnx.Clients.Kube.CoreV1().ConfigMaps(ns).Update(ctx, orgNew, metav1.UpdateOptions{})
		assert.NilError(t, err)
	}
}
