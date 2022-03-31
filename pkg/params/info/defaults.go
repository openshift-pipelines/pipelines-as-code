package info

import "time"

const (
	PACConfigmapName          = "pipelines-as-code"
	PACApplicationName        = "Pipelines as Code CI"
	HubURL                    = "https://api.hub.tekton.dev/v1"
	DefaultPipelineRunTimeout = 2 * time.Hour
)
