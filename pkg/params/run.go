package params

import (
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/clients"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
)

type Run struct {
	Clients clients.Clients
	Info    info.Info
}
