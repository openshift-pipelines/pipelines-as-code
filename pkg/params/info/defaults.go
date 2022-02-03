package info

import "time"

const (
	PACInstallNS              = "pipelines-as-code"
	PACConfigmapNS            = "pipelines-as-code"
	PACApplicationName        = "Pipelines as Code CI"
	HubURL                    = "https://api.hub.tekton.dev/v1"
	DefaultPipelineRunTimeout = 2 * time.Hour

	DefaultControllerService = "pipelines-as-code-controller"
	DefaultControllerPort    = "8080"
)
