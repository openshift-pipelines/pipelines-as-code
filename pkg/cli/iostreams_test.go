package cli

import (
	"os"
	"testing"

	"gotest.tools/v3/assert"
)

func TestNewIOStreams(t *testing.T) {
	ios := NewIOStreams()
	assert.Assert(t, ios != nil)
	assert.Assert(t, ios.In != nil)
	assert.Assert(t, ios.Out != nil)
	assert.Assert(t, ios.ErrOut != nil)
}

func TestIOStreams_ColorEnabled(t *testing.T) {
	ios := &IOStreams{colorEnabled: true}
	assert.Equal(t, ios.ColorEnabled(), true)

	ios.SetColorEnabled(false)
	assert.Equal(t, ios.ColorEnabled(), false)
}

func TestIOStreams_ColorSupport256(t *testing.T) {
	ios := &IOStreams{is256enabled: true}
	assert.Equal(t, ios.ColorSupport256(), true)

	ios.is256enabled = false
	assert.Equal(t, ios.ColorSupport256(), false)
}

func TestIOStreams_IsStdoutTTY(t *testing.T) {
	tests := []struct {
		name     string
		setup    func() *IOStreams
		expected bool
	}{
		{
			name: "with override true",
			setup: func() *IOStreams {
				ios := &IOStreams{}
				ios.SetStdoutTTY(true)
				return ios
			},
			expected: true,
		},
		{
			name: "with override false",
			setup: func() *IOStreams {
				ios := &IOStreams{}
				ios.SetStdoutTTY(false)
				return ios
			},
			expected: false,
		},
		{
			name: "with actual file",
			setup: func() *IOStreams {
				return &IOStreams{Out: os.Stdout}
			},
			expected: isTerminal(os.Stdout),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ios := tt.setup()
			assert.Equal(t, ios.IsStdoutTTY(), tt.expected)
		})
	}
}

func TestIOTest(t *testing.T) {
	ios, in, out, errOut := IOTest()
	assert.Assert(t, ios != nil)
	assert.Assert(t, in != nil)
	assert.Assert(t, out != nil)
	assert.Assert(t, errOut != nil)

	// Test writing to streams
	testData := []byte("test")
	n, err := out.Write(testData)
	assert.NilError(t, err)
	assert.Equal(t, n, len(testData))
	assert.DeepEqual(t, out.Bytes(), testData)

	n, err = errOut.Write(testData)
	assert.NilError(t, err)
	assert.Equal(t, n, len(testData))
	assert.DeepEqual(t, errOut.Bytes(), testData)
}
