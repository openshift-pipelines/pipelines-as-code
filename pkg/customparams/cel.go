package customparams

import (
	"encoding/json"
	"fmt"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/checker/decls"
	"github.com/google/cel-go/common/types"
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
