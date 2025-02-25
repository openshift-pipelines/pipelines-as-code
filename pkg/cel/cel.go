package cel

import (
	"encoding/json"
	"fmt"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/decls"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
)

func evaluate(expr string, env *cel.Env, data map[string]any) (ref.Val, error) {
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

// Value evaluates a CEL expression with the given body, headers and
// / pacParams, it will output a Cel value or an error if selectedjm.
func Value(query string, body any, headers, pacParams map[string]string, changedFiles map[string]any) (ref.Val, error) {
	// Marshal/Unmarshal the body to a map[string]any so we can access it from the CEL
	nbody, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	var jsonMap map[string]any
	err = json.Unmarshal(nbody, &jsonMap)
	if err != nil {
		return nil, err
	}

	mapStrDyn := types.NewMapType(types.StringType, types.DynType)
	celDec, _ := cel.NewEnv(
		cel.VariableDecls(
			decls.NewVariable("body", mapStrDyn),
			decls.NewVariable("headers", mapStrDyn),
			decls.NewVariable("pac", mapStrDyn),
			decls.NewVariable("files", mapStrDyn),
		))
	val, err := evaluate(query, celDec, map[string]any{
		"body":    jsonMap,
		"pac":     pacParams,
		"headers": headers,
		"files":   changedFiles,
	})
	if err != nil {
		return nil, err
	}
	return val, nil
}
