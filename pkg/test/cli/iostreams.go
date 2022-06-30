package cli

import (
	"bytes"
	"io/ioutil"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli"
)

// NewIOStream return a fake iostreams
func NewIOStream() (*cli.IOStreams, *bytes.Buffer) {
	in := &bytes.Buffer{}
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	return &cli.IOStreams{
		In:     ioutil.NopCloser(in),
		Out:    out,
		ErrOut: errOut,
	}, out
}
