package adapter

import (
	"bytes"
	"context"
	"net/http"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/kubeinteraction"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/pipelineascode"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider"
	"go.uber.org/zap"
)

type sinker struct {
	run        *params.Run
	vcx        provider.Interface
	kint       kubeinteraction.Interface
	event      *info.Event
	logger     *zap.SugaredLogger
	payload    []byte
	pacInfo    *info.PacOpts
	globalRepo *v1alpha1.Repository
}

func (s *sinker) processEventPayload(ctx context.Context, request *http.Request) error {
	var err error
	s.event, err = s.vcx.ParsePayload(ctx, s.run, request, string(s.payload))
	if err != nil {
		s.logger.Errorf("failed to parse event: %v", err)
		return err
	}

	// set logger with sha and event type
	s.logger = s.logger.With("event-sha", s.event.SHA, "event-type", s.event.EventType)
	s.vcx.SetLogger(s.logger)

	s.event.Request = &info.Request{
		Header:  request.Header,
		Payload: bytes.TrimSpace(s.payload),
	}
	return nil
}

func (s *sinker) processEvent(ctx context.Context, request *http.Request) error {
	if s.event.EventType == "incoming" {
		if request.Header.Get("X-GitHub-Enterprise-Host") != "" {
			s.event.Provider.URL = request.Header.Get("X-GitHub-Enterprise-Host")
			s.event.GHEURL = request.Header.Get("X-GitHub-Enterprise-Host")
		}
	} else {
		if err := s.processEventPayload(ctx, request); err != nil {
			return err
		}
	}

	p := pipelineascode.NewPacs(s.event, s.vcx, s.run, s.pacInfo, s.kint, s.logger, s.globalRepo)
	return p.Run(ctx)
}
