package main

import (
	"os"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cmd"
)

func main() {
	tp := &cli.PacParams{}
	tkn := cmd.Root(tp)
	if err := tkn.Execute(); err != nil {
		os.Exit(1)
	}
}
