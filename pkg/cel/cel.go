package customparams

import (
	"encoding/json"
	"fmt"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/checker/decls"
	"github.com/google/cel-go/common/types/ref"
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

// CelValue evaluates a CEL expression with the given body, headers and
// / pacParams, it will output a Cel value or an error if selectedjm.
func CelValue(query string, body any, headers, pacParams map[string]string, changedFiles map[string]interface{}) (ref.Val, error) {
	// Marshal/Unmarshal the body to a map[string]interface{} so we can access it from the CEL
	nbody, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	var jsonMap map[string]interface{}
	err = json.Unmarshal(nbody, &jsonMap)
	if err != nil {
		return nil, err
	}

	mapStrDyn := decls.NewMapType(decls.String, decls.Dyn)
	celDec, _ := cel.NewEnv(
		cel.Declarations(
			decls.NewVar("body", mapStrDyn),
			decls.NewVar("headers", mapStrDyn),
			decls.NewVar("pac", mapStrDyn),
			decls.NewVar("files", mapStrDyn),
		))
	val, err := celEvaluate(query, celDec, map[string]any{
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
