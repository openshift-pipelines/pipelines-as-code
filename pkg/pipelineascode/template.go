package pipelineascode

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/checker/decls"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/formatting"
	sectypes "github.com/openshift-pipelines/pipelines-as-code/pkg/secrets/types"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/templates"
	"go.uber.org/zap"
)

func celEvaluate(expr string, env *cel.Env, data map[string]interface{}) (ref.Val, error) {
	parsed, issues := env.Parse(expr)
	if issues != nil && issues.Err() != nil {
		return nil, fmt.Errorf("failed to parse expression %#v: %w", expr, issues.Err())
	}

	checked, issues := env.Check(parsed)
	if issues != nil && issues.Err() != nil {
		return nil, fmt.Errorf("expression %#v check failed: %w", expr, issues.Err())
	}

	prg, err := env.Program(checked, cel.EvalOptions(cel.OptOptimize))
	if err != nil {
		return nil, fmt.Errorf("expression %#v failed to create a Program: %w", expr, err)
	}

	out, _, err := prg.Eval(data)
	if err != nil {
		return nil, fmt.Errorf("expression %#v failed to evaluate: %w", expr, err)
	}

	return out, nil
}

// celFilter filters a query with cel, it sets two variables for the user to use
// in the cel filter. body and pac.
// body is the payload of the event, since it's already casted we un-marshall/and
// marshall it as a map[string]interface{}
func celFilter(query string, body any, pacParams map[string]string) (bool, error) {
	nbody, err := json.Marshal(body)
	if err != nil {
		return false, err
	}
	var jsonMap map[string]interface{}
	err = json.Unmarshal(nbody, &jsonMap)
	if err != nil {
		return false, err
	}

	mapStrDyn := decls.NewMapType(decls.String, decls.Dyn)
	celDec, _ := cel.NewEnv(
		cel.Declarations(
			decls.NewVar("body", mapStrDyn),
			decls.NewVar("pac", mapStrDyn),
		))
	val, err := celEvaluate(query, celDec, map[string]any{
		"body": jsonMap,
		"pac":  pacParams,
	})
	if err != nil {
		return false, err
	}

	//nolint: gocritic
	switch val.(type) {
	case types.Bool:
		if val.Value() == true {
			return true, nil
		}
	}
	return false, nil
}

// processParams will process the parameters as set in the repo.Spec CR.
// value can come from a string or from a secretKeyRef or from a string value
// if both is set we pick the value and issue a warning in the user namespace
// we let the user specify a cel filter. If false then we skip the parameters.
// if multiple params name has a filter we pick up the first one that has
// matched true.
func (p *PacRun) processParams(ctx context.Context, repo *v1alpha1.Repository, maptemplate map[string]string) error {
	mapFilters := map[string]string{}
	if repo.Spec.Params == nil {
		return nil
	}
	for index, value := range *repo.Spec.Params {
		// if the name is empty we skip it
		if value.Name == "" {
			p.eventEmitter.EmitMessage(repo, zap.ErrorLevel,
				"ParamsFilterSkipped", fmt.Sprintf("no name has been set in params[%d] of repo %s", index, repo.GetName()))
			continue
		}
		if value.Filter != "" {
			// if we already have a filter that has matched we skip it
			if _, ok := mapFilters[value.Name]; ok {
				p.eventEmitter.EmitMessage(repo, zap.WarnLevel,
					"ParamsFilterSkipped", fmt.Sprintf("skipping params name %s, filter has already been matched previously", value.Name))
				continue
			}

			// if the cel filter condition is false we skip it
			cond, err := celFilter(value.Filter, p.event.Event, maptemplate)
			if err != nil {
				return err
			}
			if !cond {
				p.eventEmitter.EmitMessage(repo, zap.InfoLevel,
					"ParamsFilterSkipped", fmt.Sprintf("skipping params name %s, filter condition is false", value.Name))
				continue
			}
			mapFilters[value.Name] = value.Value
		}

		if value.SecretRef != nil && value.Value != "" {
			p.eventEmitter.EmitMessage(repo, zap.InfoLevel,
				"ParamsFilterUsedValue",
				fmt.Sprintf("repo %s, param name %s has a value and secretref, picking value", repo.GetName(), value.Name))
		}
		if value.Value != "" {
			maptemplate[value.Name] = value.Value
		} else if value.SecretRef != nil {
			secretValue, err := p.k8int.GetSecret(ctx, sectypes.GetSecretOpt{
				Namespace: repo.GetNamespace(),
				Name:      value.SecretRef.Name,
				Key:       value.SecretRef.Key,
			})
			if err != nil {
				return err
			}
			maptemplate[value.Name] = secretValue
		}
	}
	return nil
}

// makeTemplate will process all templates replacing the value from the event and from the
// params as set on Repo CR
func (p *PacRun) makeTemplate(ctx context.Context, repo *v1alpha1.Repository, template string) string {
	repoURL := p.event.URL
	// On bitbucket server you are have a special url for checking it out, they
	// seemed to fix it in 2.0 but i guess we have to live with this until then.
	if p.event.CloneURL != "" {
		repoURL = p.event.CloneURL
	}

	if p.event.CloneURL == "" {
		p.event.AccountID = ""
	}

	maptemplate := map[string]string{
		"revision":         p.event.SHA,
		"repo_url":         repoURL,
		"repo_owner":       strings.ToLower(p.event.Organization),
		"repo_name":        strings.ToLower(p.event.Repository),
		"target_branch":    formatting.SanitizeBranch(p.event.BaseBranch),
		"source_branch":    formatting.SanitizeBranch(p.event.HeadBranch),
		"sender":           strings.ToLower(p.event.Sender),
		"target_namespace": repo.GetNamespace(),
		"event_type":       p.event.EventType,
	}

	if err := p.processParams(ctx, repo, maptemplate); err != nil {
		p.eventEmitter.EmitMessage(repo, zap.ErrorLevel, "ParamsError",
			fmt.Sprintf("error processing repository CR custom params: %s", err.Error()))
	}

	// convert to string
	if p.event.PullRequestNumber != 0 {
		maptemplate["pull_request_number"] = fmt.Sprintf("%d", p.event.PullRequestNumber)
	}

	return templates.ReplacePlaceHoldersVariables(template, maptemplate)
}
