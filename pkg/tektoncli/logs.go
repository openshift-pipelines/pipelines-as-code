package tektoncli

import (
	"bytes"
	"os"

	"io"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli"
	cliinterface "github.com/tektoncd/cli/pkg/cli"
	"github.com/tektoncd/cli/pkg/log"
	clilog "github.com/tektoncd/cli/pkg/log"
	clioptions "github.com/tektoncd/cli/pkg/options"
	cliprdesc "github.com/tektoncd/cli/pkg/pipelinerun/description"
)

// setupCliOpts setup clip options for a prName in a namespace
func setupCliOpts(namespace, prName string) clioptions.LogOptions {
	cliparam := cliinterface.TektonParams{}
	cliparam.SetNamespace(namespace)
	cliparam.Clients()
	cliparam.SetNoColour(true)
	return clioptions.LogOptions{
		Params:          &cliparam,
		AllSteps:        true,
		PipelineRunName: prName,
		Follow:          true,
	}
}

// FollowLogs follow log of a PR `prName` into the namesapce `namespace`. Output
// stdin and stderr to stdout and as a string
func FollowLogs(prName, namespace string, cs *cli.Clients) (string, error) {
	var outputBuffer bytes.Buffer

	cliopts := setupCliOpts(namespace, prName)
	lr, err := clilog.NewReader(clilog.LogTypePipeline, &cliopts)

	logC, errC, err := lr.Read()
	if err != nil {
		return "", err
	}

	mwr := io.MultiWriter(os.Stdout, &outputBuffer)

	cs.Log.Infof("Watching PipelineRun %s", prName)
	cliopts.Stream = &cliinterface.Stream{
		Out: mwr,
		Err: mwr,
	}

	log.NewWriter(log.LogTypePipeline).Write(cliopts.Stream, logC, errC)
	return outputBuffer.String(), nil
}

func PipelineRunDescribe(prName, namespace string) (string, error) {
	var outputBuffer bytes.Buffer
	cliopts := setupCliOpts(namespace, prName)
	mwr := io.MultiWriter(os.Stdout, &outputBuffer)
	cliopts.Stream = &cliinterface.Stream{
		Out: mwr,
		Err: mwr,
	}

	err := cliprdesc.PrintPipelineRunDescription(cliopts.Stream, prName, cliopts.Params)
	return outputBuffer.String(), err
}
