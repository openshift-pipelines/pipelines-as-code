package params

import (
	"context"
	"strings"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/clients"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Run struct {
	Clients clients.Clients
	Info    info.Info
}

func (r *Run) GetConfigFromConfigMap(ctx context.Context) error {
	cfg, err := r.Clients.Kube.CoreV1().ConfigMaps(info.PACInstallNS).Get(ctx, info.PACConfigmapNS, v1.GetOptions{})
	if err != nil {
		return err
	}

	if r.Info.Pac.ApplicationName == "" {
		if applicationName, ok := cfg.Data["application-name"]; ok {
			r.Info.Pac.ApplicationName = applicationName
		} else {
			r.Info.Pac.ApplicationName = info.PACApplicationName
		}
	}

	if secretAutoCreation, ok := cfg.Data["secret-auto-create"]; ok {
		sv := false

		if strings.ToLower(secretAutoCreation) == "true" ||
			strings.ToLower(secretAutoCreation) == "yes" || secretAutoCreation == "1" {
			sv = true
		}
		r.Info.Pac.SecretAutoCreation = sv
	}
	return nil
}

func New() *Run {
	return &Run{
		Info: info.Info{
			Event: &info.Event{},
			Pac:   &info.PacOpts{},
		},
	}
}
