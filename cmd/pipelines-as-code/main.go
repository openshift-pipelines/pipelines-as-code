package main

import (
	"os"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cmd/pipelineascode"
)

func main() {
	tp := &cli.PacParams{}
	pac := pipelineascode.Command(tp)

	if err := pac.Execute(); err != nil {
		os.Exit(1)
	}
}
