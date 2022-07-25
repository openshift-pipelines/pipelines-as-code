package matcher

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/interpreter/functions"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider"
	exprpb "google.golang.org/genproto/googleapis/api/expr/v1alpha1"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/checker/decls"
	"github.com/google/cel-go/common/types/ref"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
)

func celEvaluate(ctx context.Context, expr string, event *info.Event, vcx provider.Interface) (ref.Val, error) {
	data := map[string]interface{}{
		"event":         event.TriggerTarget,
		"target_branch": event.BaseBranch,
		"source_branch": event.HeadBranch,
	}

	env, err := cel.NewEnv(
		cel.Lib(celPac{vcx, ctx, event}),
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

type celPac struct {
	vcx   provider.Interface
	ctx   context.Context
	event *info.Event
}

func (t celPac) ProgramOptions() []cel.ProgramOption {
	return []cel.ProgramOption{
		cel.Functions(
			&functions.Overload{
				Operator: "pathChanged",
				Unary:    t.pathChanged,
			},
		),
	}
}

func (t celPac) pathChanged(vals ref.Val) ref.Val {
	var match types.Bool
	fileList, err := t.vcx.GetFiles(t.ctx, t.event)
	if err != nil {
		return types.Bool(false)
	}
	for i := range fileList {
		if v, ok := vals.Value().(string); ok {
			if strings.Contains(fileList[i], v) {
				return types.Bool(true)
			}
		}
		match = types.Bool(false)
	}

	return match
}

func (celPac) CompileOptions() []cel.EnvOption {
	return []cel.EnvOption{cel.Declarations(
		decls.NewFunction("pathChanged",
			decls.NewInstanceOverload("pathChanged",
				[]*exprpb.Type{decls.String}, decls.Bool)),
	)}
}
