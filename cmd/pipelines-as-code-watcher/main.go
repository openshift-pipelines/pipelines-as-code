package main

import (
	"github.com/openshift-pipelines/pipelines-as-code/pkg/reconciler"
	"knative.dev/pkg/injection/sharedmain"
)

func main() {
	sharedmain.Main("pac-watcher", reconciler.NewController())
}
