package adapter

import (
	"bytes"
	"context"
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

	p := pipelineascode.NewPacs(s.event, s.vcx, s.run, s.kint)
	return p.Run(ctx)
}
