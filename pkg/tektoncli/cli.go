package tektoncli

import (
	"bytes"
	"io"
	"os"

	cliinterface "github.com/tektoncd/cli/pkg/cli"
	"github.com/tektoncd/cli/pkg/log"
	clilog "github.com/tektoncd/cli/pkg/log"
	clioptions "github.com/tektoncd/cli/pkg/options"
	cliprdesc "github.com/tektoncd/cli/pkg/pipelinerun/description"
)

type Interface interface {
	FollowLogs(string, string) (string, error)
	PipelineRunDescribe(string, string) (string, error)
}

type TektonCLI struct {
	cliOpts clioptions.LogOptions
}

func New(namespace string, cliParam cliinterface.Params) (*TektonCLI, error) {
	cliParam.SetNamespace(namespace)

	_, err := cliParam.Clients()
	if err != nil {
		return &TektonCLI{}, err
	}
	cliParam.SetNoColour(true)
	return &TektonCLI{
		cliOpts: clioptions.LogOptions{
			Params:   cliParam,
			AllSteps: true,
			Follow:   true,
		},
	}, nil
}

func (t *TektonCLI) FollowLogs(prName, namespace string) (string, error) {
	var outputBuffer bytes.Buffer

	t.cliOpts.Params.SetNamespace(namespace)
	t.cliOpts.PipelineRunName = prName
	lr, err := clilog.NewReader(clilog.LogTypePipeline, &t.cliOpts)
	if err != nil {
		return "", err
	}
	logC, errC, err := lr.Read()
	if err != nil {
		return "", err
	}

	mwr := io.MultiWriter(os.Stdout, &outputBuffer)

	t.cliOpts.Stream = &cliinterface.Stream{
		Out: mwr,
		Err: mwr,
	}

	log.NewWriter(log.LogTypePipeline).Write(t.cliOpts.Stream, logC, errC)
	return outputBuffer.String(), nil
}

func (t *TektonCLI) PipelineRunDescribe(prName, namespace string) (string, error) {
	var outputBuffer bytes.Buffer
	t.cliOpts.Params.SetNamespace(namespace)
	t.cliOpts.PipelineRunName = prName
	mwr := io.MultiWriter(os.Stdout, &outputBuffer)
	t.cliOpts.Stream = &cliinterface.Stream{
		Out: mwr,
		Err: mwr,
	}
	err := cliprdesc.PrintPipelineRunDescription(t.cliOpts.Stream, prName, t.cliOpts.Params)
	return outputBuffer.String(), err
}
