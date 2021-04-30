package main

import (
	"os"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cmd"
)

func main() {
	tp := &cli.PacParams{}
	tkn := cmd.Root(tp)

	cmd, _, err := tkn.Find(os.Args[1:])
	// default cmd if no cmd is given
	if err != nil || cmd.Use == "" {
		args := append([]string{"run"}, os.Args[1:]...)
		tkn.SetArgs(args)
	}

	if err := tkn.Execute(); err != nil {
		os.Exit(1)
	}
}
