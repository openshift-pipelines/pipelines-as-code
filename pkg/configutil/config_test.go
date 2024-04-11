package configutil

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/test/logger"
	"gotest.tools/v3/assert"
)

type testStruct struct {
	ApplicationName string `default:"app-app"      json:"application-name"`
	BoolField       bool   `default:"true"         json:"bool-field"`
	IntField        int    `default:"43"           json:"int-field"`
	WithoutDefault  string `json:"without-default"`
	IgnoredField    string
}

func TestValidateAndAssignValues(t *testing.T) {
	logger, _ := logger.GetLogger()

	testCases := []struct {
		name              string
		configMap         map[string]string
		expectedStruct    testStruct
		customValidations map[string]func(string) error
		expectedError     string
	}{
		{
			name:      "With all default values",
			configMap: map[string]string{},
			expectedStruct: testStruct{
				ApplicationName: "app-app",
				BoolField:       true,
				IntField:        43,
				WithoutDefault:  "",
			},
			customValidations: map[string]func(string) error{},
		},
		{
			name: "override default values",
			configMap: map[string]string{
				"application-name": "pac-pac",
				"bool-field":       "false",
				"int-field":        "101",
				"without-default":  "random",
			},
			expectedStruct: testStruct{
				ApplicationName: "pac-pac",
				BoolField:       false,
				IntField:        101,
				WithoutDefault:  "random",
			},
			customValidations: map[string]func(string) error{},
		},
		{
			name: "custom validator for name to start with pac",
			configMap: map[string]string{
				"application-name": "invalid-name",
			},
			expectedStruct: testStruct{
				ApplicationName: "throw-error",
				BoolField:       false,
				IntField:        101,
			},
			customValidations: map[string]func(string) error{
				"ApplicationName": func(s string) error {
					if !strings.HasPrefix(s, "pac") {
						return fmt.Errorf("name should start with pac")
					}
					return nil
				},
			},
			expectedError: "custom validation failed for field ApplicationName: name should start with pac",
		},
		{
			name: "invalid value for bool field",
			configMap: map[string]string{
				"bool-field": "invalid",
			},
			expectedError: "invalid value for bool field BoolField: strconv.ParseBool: parsing \"invalid\": invalid syntax",
		},
		{
			name: "invalid value for int field",
			configMap: map[string]string{
				"int-field": "abcd",
			},
			expectedError: "invalid value for int field IntField: strconv.ParseInt: parsing \"abcd\": invalid syntax",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var test testStruct

			err := ValidateAndAssignValues(logger, tc.configMap, &test, tc.customValidations, true)

			if tc.expectedError != "" {
				assert.ErrorContains(t, err, tc.expectedError)
				return
			}
			assert.NilError(t, err)

			if !reflect.DeepEqual(test, tc.expectedStruct) {
				t.Errorf("failure, actual and expected struct:\nActual: %#v\nExpected: %#v", test, tc.expectedStruct)
			}
		})
	}
}
