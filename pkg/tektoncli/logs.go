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
)

// setupCliOpts setup clip options for a prName in a namespace
func setupCliOpts(namespace, prName string) (clioptions.LogOptions, error) {
	cliparam := cliinterface.TektonParams{}
	cliparam.SetNamespace(namespace)
	_, err := cliparam.Clients()
	if err != nil {
		return clioptions.LogOptions{}, err
	}
	cliparam.SetNoColour(true)
	return clioptions.LogOptions{
		Params:          &cliparam,
		AllSteps:        true,
		PipelineRunName: prName,
		Follow:          true,
	}, nil
}

// FollowLogs follow log of a PR `prName` into the namesapce `namespace`. Output
// stdin and stderr to stdout and as a string
func FollowLogs(cs *cli.Clients, prName, namespace string) (string, error) {
	var outputBuffer bytes.Buffer

	cliopts, err := setupCliOpts(namespace, prName)
	if err != nil {
		return "", err
	}
	lr, err := clilog.NewReader(clilog.LogTypePipeline, &cliopts)
	if err != nil {
		return "", err
	}
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
