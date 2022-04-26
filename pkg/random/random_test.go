package random

import (
	"testing"

	"gotest.tools/v3/assert"
)

func TestRandomAlphaString(t *testing.T) {
	// statichcek SA4000 error if we inline on the same line :\
	f := AlphaString(4)
	assert.Assert(t, f != AlphaString(4))
}

func TestRandomAlphaStringLength(t *testing.T) {
	assert.Assert(t, len(AlphaString(10)) == 10)
}
