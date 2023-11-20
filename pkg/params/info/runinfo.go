package info

import (
	"context"
)

type Info struct {
	Pac        *PacOpts
	Kube       *KubeOpts
	Controller *ControllerInfo
}

func (i *Info) DeepCopy(out *Info) {
	*out = *i
}

type (
	contextKey string
)

type CtxInfo struct {
	Pac        *PacOpts
	Kube       *KubeOpts
	Controller *ControllerInfo
}

// GetInfo Pac Settings for that label.
func GetInfo(ctx context.Context, label string) *Info {
	labelContextKey := contextKey(label)
	if val := ctx.Value(labelContextKey); val != nil {
		if ctxInfo, ok := val.(CtxInfo); ok {
			return &Info{
				Pac:        ctxInfo.Pac,
				Kube:       ctxInfo.Kube,
				Controller: ctxInfo.Controller,
			}
		}
	}
	return nil
}

// StoreInfo Pac Settings for a label.
func StoreInfo(ctx context.Context, label string, info *Info) context.Context {
	labelContextKey := contextKey(label)
	if val := ctx.Value(labelContextKey); val != nil {
		if ctxInfo, ok := val.(CtxInfo); ok {
			if ctxInfo.Pac == nil {
				ctxInfo.Pac = &PacOpts{}
			}
			if ctxInfo.Kube == nil {
				ctxInfo.Kube = &KubeOpts{
					Namespace: GetNS(ctx),
				}
			}
			if ctxInfo.Controller == nil {
				ctxInfo.Controller = &ControllerInfo{}
			}
			ctxInfo.Pac = info.Pac
			ctxInfo.Kube = info.Kube
			ctxInfo.Controller = info.Controller
			return context.WithValue(ctx, labelContextKey, ctxInfo)
		}
	}
	return context.WithValue(ctx, labelContextKey, CtxInfo{
		Pac:        info.Pac,
		Kube:       info.Kube,
		Controller: info.Controller,
	})
}
