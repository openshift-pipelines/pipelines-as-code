package main

import (
	"context"
	"log"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/adapter"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/kubeinteraction"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	evadapter "knative.dev/eventing/pkg/adapter/v2"
	"knative.dev/pkg/client/injection/kube/client"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/signals"
	"knative.dev/pkg/system"
)

const (
	PACControllerLogKey = "pipelinesascode"
)

func main() {
	ctx := signals.NewContext()
	ns := system.Namespace()
	run := params.New()
	rinfo := &run.Info
	rinfo.Controller = info.GetControllerInfoFromEnvOrDefault()
	err := run.Clients.NewClients(ctx, rinfo)
	if err != nil {
		log.Fatal("failed to init clients : ", err)
	}

	kinteract, err := kubeinteraction.NewKubernetesInteraction(run)
	if err != nil {
		log.Fatal("failed to init kinit client : ", err)
	}

	loggerConfiguratorOpt := evadapter.WithLoggerConfiguratorConfigMapName(logging.ConfigMapName())
	loggerConfigurator := evadapter.NewLoggerConfiguratorFromConfigMap(PACControllerLogKey, loggerConfiguratorOpt)
	copt := evadapter.WithLoggerConfigurator(loggerConfigurator)
	// put logger configurator to ctx
	ctx = evadapter.WithConfiguratorOptions(ctx, []evadapter.ConfiguratorOption{copt})

	ctx = info.StoreNS(ctx, ns)
	ctx = info.StoreCurrentControllerName(ctx, rinfo.Controller.Name)

	ctx = context.WithValue(ctx, client.Key{}, run.Clients.Kube)
	ctx = evadapter.WithNamespace(ctx, system.Namespace())
	ctx = evadapter.WithConfigWatcherEnabled(ctx)
	evadapter.MainWithContext(ctx, PACControllerLogKey, adapter.NewEnvConfig, adapter.New(run, kinteract))
}
