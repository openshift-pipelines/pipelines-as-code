package adapter

import (
	"bytes"
	"context"
	"fmt"
	"net/http"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/kubeinteraction"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/pipelineascode"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider"
)

type sinker struct {
	run   *params.Run
	vcx   provider.Interface
	kint  *kubeinteraction.Interaction
	event *info.Event
}

func (s *sinker) processEvent(ctx context.Context, request *http.Request, payload []byte) error {
	var err error
	s.event, err = s.vcx.ParsePayload(ctx, s.run, request, string(payload))
	if err != nil {
		s.run.Clients.Log.Errorf("failed to parse event: %v", err)
		return err
	}
	s.event.Request = &info.Request{
		Header:  request.Header,
		Payload: bytes.TrimSpace(payload),
	}

	err = pipelineascode.Run(ctx, s.run, s.vcx, s.kint, s.event)
	if err != nil {
		createStatusErr := s.vcx.CreateStatus(ctx, s.event, s.run.Info.Pac, provider.StatusOpts{
			Status:     "completed",
			Conclusion: "failure",
			Text:       fmt.Sprintf("There was an issue validating the commit: %q", err),
			DetailsURL: s.run.Clients.ConsoleUI.URL(),
		})
		if createStatusErr != nil {
			s.run.Clients.Log.Errorf("Cannot create status: %s %s", err, createStatusErr)
		}
	}
	return err
}
