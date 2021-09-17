package main

import (
	"os"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/cmd/tknpac"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
)

func main() {
	clients := &params.Run{}
	clients.Info.Event = &info.Event{}
	pac := tknpac.Root(clients)

	if err := pac.Execute(); err != nil {
		os.Exit(1)
	}
}
