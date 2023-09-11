package customparams

import (
	"context"
	"fmt"

	apincoming "github.com/openshift-pipelines/pipelines-as-code/pkg/apis/incoming"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/events"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/kubeinteraction"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	sectypes "github.com/openshift-pipelines/pipelines-as-code/pkg/secrets/types"
	"go.uber.org/zap"
)

type CustomParams struct {
	event        *info.Event
	run          *params.Run
	k8int        kubeinteraction.Interface
	eventEmitter *events.EventEmitter
	repo         *v1alpha1.Repository
}

func NewCustomParams(event *info.Event, repo *v1alpha1.Repository, run *params.Run, k8int kubeinteraction.Interface, eventEmitter *events.EventEmitter) CustomParams {
	return CustomParams{
		event:        event,
		repo:         repo,
		run:          run,
		k8int:        k8int,
		eventEmitter: eventEmitter,
	}
}

// applyIncomingParams apply incoming params to an existing map (overwriting existing keys)
func (p *CustomParams) applyIncomingParams(ret map[string]string) map[string]string {
	if p.event.Request == nil {
		return ret
	}
	if incomingParams, err := apincoming.ParseIncomingPayload(p.event.Request.Payload); err == nil {
		for k, v := range incomingParams.Params {
			if vs, ok := v.(string); ok {
				ret[k] = vs
			} else {
				p.eventEmitter.EmitMessage(p.repo, zap.WarnLevel, "IncomingParamsNotString", fmt.Sprintf("cannot convert incoming param key: %s value: %v as string", k, v))
			}
		}
	}
	return ret
}

// GetParams will process the parameters as set in the repo.Spec CR.
// value can come from a string or from a secretKeyRef or from a string value
// if both is set we pick the value and issue a warning in the user namespace
// we let the user specify a cel filter. If false then we skip the parameters.
// if multiple params name has a filter we pick up the first one that has
// matched true.
func (p *CustomParams) GetParams(ctx context.Context) (map[string]string, error) {
	stdParams := p.makeStandardParamsFromEvent()
	if p.repo.Spec.Params == nil {
		return p.applyIncomingParams(stdParams), nil
	}
	ret := map[string]string{}
	mapFilters := map[string]string{}

	for index, value := range *p.repo.Spec.Params {
		// if the name is empty we skip it
		if value.Name == "" {
			p.eventEmitter.EmitMessage(p.repo, zap.ErrorLevel,
				"ParamsFilterSkipped", fmt.Sprintf("no name has been set in params[%d] of repo %s", index, p.repo.GetName()))
			continue
		}
		if value.Filter != "" {
			// if we already have a filter that has matched we skip it
			if _, ok := mapFilters[value.Name]; ok {
				p.eventEmitter.EmitMessage(p.repo, zap.WarnLevel,
					"ParamsFilterSkipped", fmt.Sprintf("skipping params name %s, filter has already been matched previously", value.Name))
				continue
			}

			// if the cel filter condition is false we skip it
			cond, err := celFilter(value.Filter, p.event.Event, stdParams)
			if err != nil {
				p.eventEmitter.EmitMessage(p.repo, zap.ErrorLevel,
					"ParamsFilterError", fmt.Sprintf("there is an error on the cel filter: %s: %s", value.Name, err.Error()))
				return map[string]string{}, err
			}
			if !cond {
				p.eventEmitter.EmitMessage(p.repo, zap.InfoLevel,
					"ParamsFilterSkipped", fmt.Sprintf("skipping params name %s, filter condition is false", value.Name))
				continue
			}
			mapFilters[value.Name] = value.Value
		}

		if value.SecretRef != nil && value.Value != "" {
			p.eventEmitter.EmitMessage(p.repo, zap.InfoLevel,
				"ParamsFilterUsedValue",
				fmt.Sprintf("repo %s, param name %s has a value and secretref, picking value", p.repo.GetName(), value.Name))
		}
		if value.Value != "" {
			ret[value.Name] = value.Value
		} else if value.SecretRef != nil {
			secretValue, err := p.k8int.GetSecret(ctx, sectypes.GetSecretOpt{
				Namespace: p.repo.GetNamespace(),
				Name:      value.SecretRef.Name,
				Key:       value.SecretRef.Key,
			})
			if err != nil {
				return ret, err
			}
			ret[value.Name] = secretValue
		}
	}

	// TODO: Should we let the user override the standard params?
	// we don't let them here
	for k, v := range stdParams {
		// check if not already there
		if _, ok := ret[k]; !ok && v != "" {
			ret[k] = v
		}
	}

	return p.applyIncomingParams(ret), nil
}
