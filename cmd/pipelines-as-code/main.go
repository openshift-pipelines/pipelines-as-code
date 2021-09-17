package main

import (
	"log"
	"os"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/cmd/pipelineascode"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
)

func main() {
	clients := &params.Run{Info: info.Info{Event: &info.Event{}}}
	cmd := pipelineascode.Command(clients)
	if err := cmd.Execute(); err != nil {
		log.Fatal(err)
		os.Exit(1)
	}
}
