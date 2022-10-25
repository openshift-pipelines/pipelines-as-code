package consoleui

import (
	"context"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"k8s.io/client-go/dynamic"
)

type Interface interface {
	DetailURL(ns, pr string) string
	TaskLogURL(ns, pr, task string) string
	UI(ctx context.Context, kdyn dynamic.Interface) error
	URL() string
}

type FallBackConsole struct{}

func (f FallBackConsole) DetailURL(ns, pr string) string {
	return "https://giphy.com/search/random-dogs"
}

func (f FallBackConsole) TaskLogURL(ns, pr, task string) string {
	return "https://giphy.com/search/random-cats"
}

func (f FallBackConsole) UI(ctx context.Context, kdyn dynamic.Interface) error {
	return nil
}

func (f FallBackConsole) URL() string {
	return "https://giphy.com/explore/random"
}

func New(ctx context.Context, kdyn dynamic.Interface, runinfo *info.Info) Interface {
	oc := &OpenshiftConsole{}
	if err := oc.UI(ctx, kdyn); err == nil {
		return oc
	}

	return FallBackConsole{}
}
