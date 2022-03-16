package adapter

import (
	"context"
	"fmt"
	"net/http"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/kubeinteraction"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/version"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/pipelineascode"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider"
)

type sinker struct {
	run  *params.Run
	vcx  provider.Interface
	kint *kubeinteraction.Interaction
}

func (s *sinker) processEvent(ctx context.Context, request *http.Request, payload []byte) {
	s.run.Info.Pac.LogURL = s.run.Clients.ConsoleUI.URL()

	if err := s.vcx.ParseEventType(request, s.run.Info.Event); err != nil {
		s.run.Clients.Log.Errorf("failed to find event type: %v", err)
		return
	}

	var err error
	s.run.Info.Event, err = s.vcx.ParseEventPayload(ctx, s.run, string(payload))
	if err != nil {
		s.run.Clients.Log.Errorf("failed to parse event: %v", err)
		return
	}

	s.run.Clients.Log.Infof("Starting Pipelines as Code version: %s", version.Version)
	err = pipelineascode.Run(ctx, s.run, s.vcx, s.kint)
	if err != nil {
		createStatusErr := s.vcx.CreateStatus(ctx, s.run.Info.Event, s.run.Info.Pac, provider.StatusOpts{
			Status:     "completed",
			Conclusion: "failure",
			Text:       fmt.Sprintf("There was an issue validating the commit: %q", err),
			DetailsURL: s.run.Clients.ConsoleUI.URL(),
		})
		if createStatusErr != nil {
			s.run.Clients.Log.Errorf("Cannot create status: %s %s", err, createStatusErr)
		}
	}
}
