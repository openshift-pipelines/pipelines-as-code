package tektoncli

import (
	"os"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli"
	cliinterface "github.com/tektoncd/cli/pkg/cli"
	"github.com/tektoncd/cli/pkg/log"
	clilog "github.com/tektoncd/cli/pkg/log"
	clioptions "github.com/tektoncd/cli/pkg/options"
)

func FollowLogs(prName string, namespace string, cs *cli.Clients) error {
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
		return err
	}

	cs.Log.Infof("Watching PipelineRun %s", prName)
	cliopts.Stream = &cliinterface.Stream{
		Out: os.Stdout,
		Err: os.Stderr,
	}

	log.NewWriter(log.LogTypePipeline).Write(cliopts.Stream, logC, errC)
	return nil
}
