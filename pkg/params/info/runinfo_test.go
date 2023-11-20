package info

import (
	"context"
	"testing"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/settings"
	"gotest.tools/v3/assert"
)

func TestRunInfoContext(t *testing.T) {
	label1 := "label1"
	info := &Info{
		Pac: &PacOpts{
			Settings: &settings.Settings{
				ApplicationName: "App for " + label1,
			},
		},
		Kube: &KubeOpts{
			Namespace: label1,
		},
	}
	ctx := context.TODO()
	ctx = StoreInfo(ctx, label1, info)

	label2 := "label2"
	info2 := &Info{
		Pac: &PacOpts{
			Settings: &settings.Settings{
				ApplicationName: "App for " + label2,
			},
		},
		Kube: &KubeOpts{
			Namespace: label2,
		},
	}
	ctx = StoreInfo(ctx, label2, info2)

	t.Run("Get", func(t *testing.T) {
		rinfo1 := GetInfo(ctx, label1)
		assert.Assert(t, rinfo1 != nil)
		assert.Assert(t, rinfo1.Pac.Settings.ApplicationName == "App for "+label1)
		assert.Assert(t, rinfo1.Kube.Namespace == label1)
		rinfo2 := GetInfo(ctx, label2)
		assert.Assert(t, rinfo2 != nil)
		assert.Assert(t, rinfo2.Pac.Settings.ApplicationName == "App for "+label2)
		assert.Assert(t, rinfo2.Kube.Namespace == label2)
	})
	t.Run("Get no context", func(t *testing.T) {
		nctx := context.TODO()
		assert.Assert(t, GetInfo(nctx, label1) == nil)
	})
}
