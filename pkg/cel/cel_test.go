package cel

import (
	"testing"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/checker/decls"
	"github.com/google/cel-go/common/types/ref"
	"gotest.tools/v3/assert"
)

func TestCelEvaluate(t *testing.T) {
	env, _ := cel.NewEnv(
		cel.Declarations(
			decls.NewVar("key", decls.String),
		),
	)
	data := map[string]interface{}{"key": "value"}

	// Test a valid expression
	val, err := evaluate("key == 'value'", env, data)
	assert.NilError(t, err)
	assert.Equal(t, ref.Val(val), val)

	// Test an invalid expression
	_, err = evaluate("invalid expression", env, data)
	assert.ErrorContains(t, err, "failed to parse expression")
}

func TestValue(t *testing.T) {
	body := map[string]interface{}{"key": "value"}
	headers := map[string]string{"header": "value"}
	pacParams := map[string]string{"param": "value"}
	changedFiles := map[string]interface{}{"file": "value"}

	// Test a valid query
	val, err := Value("body.key == 'value'", body, headers, pacParams, changedFiles)
	assert.NilError(t, err)
	assert.Equal(t, ref.Val(val), val)

	// Test an invalid query
	_, err = Value("invalid query", body, headers, pacParams, changedFiles)
	assert.ErrorContains(t, err, "failed to parse expression")
}
