package main

import (
	"log"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/adapter"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/kubeinteraction"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	evadapter "knative.dev/eventing/pkg/adapter/v2"
	"knative.dev/pkg/signals"
)

const (
	PACControllerLogKey = "pipelinesascode"
)

func main() {
	ctx := signals.NewContext()

	run := params.New()
	err := run.Clients.NewClients(ctx, &run.Info)
	if err != nil {
		log.Fatal("failed to init clients : ", err)
	}

	kinteract, err := kubeinteraction.NewKubernetesInteraction(run)
	if err != nil {
		log.Fatal("failed to init kinit client : ", err)
	}

	if err := run.GetConfigFromConfigMap(ctx); err != nil {
		log.Fatal("failed to get defaults : ", err)
	}

	run.Info.Pac.LogURL = run.Clients.ConsoleUI.URL()

	evadapter.MainWithContext(ctx, PACControllerLogKey, adapter.NewEnvConfig, adapter.New(run, kinteract))
}
