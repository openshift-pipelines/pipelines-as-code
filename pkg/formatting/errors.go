package formatting

import (
	"encoding/json"
	"errors"
	"fmt"
)

func HumanizeJSONErr(jsonContent string, err error) error {
	errorOffset := -1
	var syntaxErr *json.SyntaxError
	var typeErr *json.UnmarshalTypeError

	if errors.As(err, &syntaxErr) {
		err = fmt.Errorf("JSON syntax error: %w", err)
		errorOffset = int(syntaxErr.Offset)
	} else if errors.As(err, &typeErr) {
		err = fmt.Errorf("JSON type error: %w", err)
		errorOffset = int(typeErr.Offset)
	}

	if line, char, ok := lineAndCharacterFromOffset([]byte(jsonContent), errorOffset); ok {
		err = fmt.Errorf("%w on line %d char %d", err, line, char)
	}

	return err
}

func lineAndCharacterFromOffset(content []byte, offset int) (int, int, bool) {
	if len(content) < offset || offset < 1 {
		return 0, 0, false
	}
	line := 1
	lineOffset := 0

	for i := 0; i < offset; i++ {
		if content[i] == '\n' {
			line++
			lineOffset = 0
		} else {
			lineOffset++
		}
	}

	return line, lineOffset, true
}
