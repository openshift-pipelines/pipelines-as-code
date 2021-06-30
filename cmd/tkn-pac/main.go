package main

import (
	"os"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cmd/tknpac"
)

func main() {
	tp := &cli.PacParams{}
	pac := tknpac.Root(tp)

	if err := pac.Execute(); err != nil {
		os.Exit(1)
	}
}
