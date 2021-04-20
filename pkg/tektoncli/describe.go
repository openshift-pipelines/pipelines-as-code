package tektoncli

import (
	"bytes"
	"io"
	"os"

	cliinterface "github.com/tektoncd/cli/pkg/cli"
	cliprdesc "github.com/tektoncd/cli/pkg/pipelinerun/description"
)

func PipelineRunDescribe(prName, namespace string) (string, error) {
	var outputBuffer bytes.Buffer
	cliopts, err := setupCliOpts(namespace, prName)
	if err != nil {
		return "", err
	}
	mwr := io.MultiWriter(os.Stdout, &outputBuffer)
	cliopts.Stream = &cliinterface.Stream{
		Out: mwr,
		Err: mwr,
	}

	err = cliprdesc.PrintPipelineRunDescription(cliopts.Stream, prName, cliopts.Params)
	return outputBuffer.String(), err
}
