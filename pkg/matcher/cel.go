package matcher

import (
	"fmt"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/checker/decls"
	"github.com/google/cel-go/common/types/ref"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
)

func celEvaluate(expr string, event *info.Event) (ref.Val, error) {
	data := map[string]interface{}{
		"event":         event.TriggerTarget,
		"target_branch": event.BaseBranch,
		"source_branch": event.HeadBranch,
	}

	env, err := cel.NewEnv(
		cel.Declarations(
			decls.NewVar("event", decls.String),
			decls.NewVar("target_branch", decls.String),
			decls.NewVar("source_branch", decls.String)))
	if err != nil {
		return nil, err
	}

	parsed, issues := env.Parse(expr)
	if issues != nil && issues.Err() != nil {
		return nil, fmt.Errorf("failed to parse expression %#v: %w", expr, issues.Err())
	}

	checked, issues := env.Check(parsed)
	if issues != nil && issues.Err() != nil {
		return nil, fmt.Errorf("expression %#v check failed: %w", expr, issues.Err())
	}

	prg, err := env.Program(checked)
	if err != nil {
		return nil, fmt.Errorf("expression %#v failed to create a Program: %w", expr, err)
	}

	out, _, err := prg.Eval(data)
	if err != nil {
		return nil, fmt.Errorf("expression %#v failed to evaluate: %w", expr, err)
	}
	return out, nil
}
