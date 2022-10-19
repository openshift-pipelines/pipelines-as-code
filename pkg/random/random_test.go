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

func TestCryptoString(t *testing.T) {
	// there are no prime numbers <= 2
	_, err := CryptoString(1)
	if err == nil {
		t.Fail()
	}
}

func TestCryptoStringLength(t *testing.T) {
	randomCrypto, err := CryptoString(16)
	if err != nil {
		t.Fail()
	}
	assert.Assert(t, len(randomCrypto) == 5)
}
