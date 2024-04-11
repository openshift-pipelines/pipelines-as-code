package configutil

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"go.uber.org/zap"
)

func ValidateAndAssignValues(logger *zap.SugaredLogger, configData map[string]string, configStruct interface{}, customValidations map[string]func(string) error, logUpdates bool) error {
	structValue := reflect.ValueOf(configStruct).Elem()
	structType := reflect.TypeOf(configStruct).Elem()

	var errors []error

	for i := 0; i < structType.NumField(); i++ {
		field := structType.Field(i)
		fieldName := field.Name

		jsonTag := field.Tag.Get("json")
		// Skip field which doesn't have json tag
		if jsonTag == "" {
			continue
		}

		// Read value from ConfigMap
		fieldValue := configData[strings.ToLower(jsonTag)]

		// If value is missing in ConfigMap, use default value from struct tag
		if fieldValue == "" {
			fieldValue = field.Tag.Get("default")
			if fieldValue == "" {
				// Skip field if default value is not provided
				continue
			}
		}

		fieldValueKind := field.Type.Kind()

		//nolint
		switch fieldValueKind {
		case reflect.String:
			if validator, ok := customValidations[fieldName]; ok {
				if err := validator(fieldValue); err != nil {
					errors = append(errors, fmt.Errorf("custom validation failed for field %s: %w", fieldName, err))
					continue
				}
			}
			oldValue := structValue.FieldByName(fieldName).String()
			if oldValue != fieldValue && logUpdates {
				logger.Infof("updating value for field %s: from '%s' to '%s'", fieldName, oldValue, fieldValue)
			}
			structValue.FieldByName(fieldName).SetString(fieldValue)

		case reflect.Bool:
			boolValue, err := strconv.ParseBool(fieldValue)
			if err != nil {
				errors = append(errors, fmt.Errorf("invalid value for bool field %s: %w", fieldName, err))
				continue
			}
			oldValue := structValue.FieldByName(fieldName).Bool()
			if oldValue != boolValue && logUpdates {
				logger.Infof("updating value for field %s: from '%v' to '%v'", fieldName, oldValue, boolValue)
			}
			structValue.FieldByName(fieldName).SetBool(boolValue)

		case reflect.Int:
			if validator, ok := customValidations[fieldName]; ok {
				if err := validator(fieldValue); err != nil {
					errors = append(errors, fmt.Errorf("custom validation failed for field %s: %w", fieldName, err))
					continue
				}
			}
			intValue, err := strconv.ParseInt(fieldValue, 10, 64)
			if err != nil {
				errors = append(errors, fmt.Errorf("invalid value for int field %s: %w", fieldName, err))
				continue
			}
			oldValue := structValue.FieldByName(fieldName).Int()
			if oldValue != intValue && logUpdates {
				logger.Infof("updating value for field %s: from '%d' to '%d'", fieldName, oldValue, intValue)
			}
			structValue.FieldByName(fieldName).SetInt(intValue)

		default:
			// Skip unsupported field types
			continue
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("validation errors: %v", errors)
	}

	return nil
}
