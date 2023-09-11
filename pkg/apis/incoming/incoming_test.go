package incoming

import (
	"testing"

	"gotest.tools/v3/assert"
)

func TestParseIncomingPayload(t *testing.T) {
	// Test case where payload is valid JSON
	payload := []byte(`{"params": {"key": "value"}}`)
	expected := Payload{map[string]interface{}{"key": "value"}}
	actual, err := ParseIncomingPayload(payload)
	assert.NilError(t, err)
	assert.DeepEqual(t, expected, actual)

	// // Test case where payload is invalid JSON
	payload = []byte(`invalid json`)
	emptyExpected := Payload{}
	actual, err = ParseIncomingPayload(payload)
	assert.Assert(t, err != nil)
	assert.DeepEqual(t, emptyExpected, actual)
}
