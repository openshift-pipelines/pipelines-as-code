package formatting

import (
	"encoding/json"
	"fmt"
	"testing"

	"gotest.tools/v3/assert"
)

func TestHumanizeJSONErr(t *testing.T) {
	badJSONUnclosedValue := `{
  "key": "unclosed value
}`
	badJSONType := `{
  "key": 123
}`

	type NamedStruct struct {
		Key string
	}

	data := map[string]string{}
	tests := []struct {
		Name            string
		JSON            string
		Error           error
		ExpectedMessage string
	}{
		{
			Name:            "invalid json",
			JSON:            badJSONUnclosedValue,
			Error:           json.Unmarshal([]byte(badJSONUnclosedValue), &data),
			ExpectedMessage: "JSON syntax error: invalid character '\\n' in string literal on line 3 char 0",
		},
		{
			Name:            "type error",
			JSON:            badJSONType,
			Error:           json.Unmarshal([]byte(badJSONType), &data),
			ExpectedMessage: "JSON type error: json: cannot unmarshal number into Go value of type string on line 2 char 12",
		},
		{
			Name:            "type error in named struct",
			JSON:            badJSONType,
			Error:           json.Unmarshal([]byte(badJSONType), &NamedStruct{}),
			ExpectedMessage: "JSON type error: json: cannot unmarshal number into Go struct field NamedStruct.Key of type string on line 2 char 12",
		},
		{
			Name:            "non-json error",
			JSON:            "{ \"key\": \"dummy value\"}",
			Error:           fmt.Errorf("non-json error"),
			ExpectedMessage: "non-json error",
		},
	}
	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			assert.Equal(t, HumanizeJSONErr(tt.JSON, tt.Error).Error(), tt.ExpectedMessage)
		})
	}
}
