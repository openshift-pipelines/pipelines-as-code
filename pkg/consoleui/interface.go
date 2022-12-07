package consoleui

import (
	"context"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"k8s.io/client-go/dynamic"
)

const consoleIsnotConfiguredURL = "https://dashboard.is.not.configured"

type Interface interface {
	DetailURL(ns, pr string) string
	TaskLogURL(ns, pr, task string) string
	UI(ctx context.Context, kdyn dynamic.Interface) error
	URL() string
	GetName() string
}

type FallBackConsole struct{}

func (f FallBackConsole) GetName() string {
	return "Not configured"
}

func (f FallBackConsole) DetailURL(ns, pr string) string {
	return consoleIsnotConfiguredURL
}

func (f FallBackConsole) TaskLogURL(ns, pr, task string) string {
	return consoleIsnotConfiguredURL
}

func (f FallBackConsole) UI(ctx context.Context, kdyn dynamic.Interface) error {
	return nil
}

func (f FallBackConsole) URL() string {
	return consoleIsnotConfiguredURL
}

func New(ctx context.Context, kdyn dynamic.Interface, _ *info.Info) Interface {
	oc := &OpenshiftConsole{}
	if err := oc.UI(ctx, kdyn); err == nil {
		return oc
	}

	// TODO: Try to detect TektonDashboard somehow by ingress?
	return FallBackConsole{}
}
