package errors

type PacYamlValidations struct {
	Name   string
	Err    error
	Schema string
}

const GenericBadYAMLValidation = "Generic bad YAML Validation"
