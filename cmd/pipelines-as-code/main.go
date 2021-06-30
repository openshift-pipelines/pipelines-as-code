package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cmd/pipelineascode"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cmd/tknpac"
	"github.com/spf13/cobra"
)

// RouteBinary according to $0
func RouteBinary(binary string) *cobra.Command {
	tp := &cli.PacParams{}
	if filepath.Base(binary) == "pipelines-as-code" {
		return pipelineascode.Command(tp)
	}
	return tknpac.Root(tp)
}

func main() {
	executable, err := os.Executable()
	if err != nil {
		fmt.Fprint(os.Stderr, err.Error())
		os.Exit(1)
	}

	cmd := RouteBinary(executable)

	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
