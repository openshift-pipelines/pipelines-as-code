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

func FollowLogs(prName string, namespace string, cs *cli.Clients) (string, error) {
	var outputBuffer bytes.Buffer

	cliparam := cliinterface.TektonParams{}
	cliparam.SetNamespace(namespace)
	cliparam.Clients()
	cliparam.SetNoColour(true)
	cliopts := clioptions.LogOptions{
		Params:          &cliparam,
		AllSteps:        true,
		PipelineRunName: prName,
		Follow:          true,
	}
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
