package main

import (
	"log"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/cmd/pipelineascode"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
)

func main() {
	clients := params.New()
	cmd := pipelineascode.Command(clients)
	if err := cmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
