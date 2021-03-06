package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/reconciler"
	"knative.dev/pkg/injection/sharedmain"
)

const globalProbesPort = "8080"

func main() {
	probesPort := globalProbesPort
	envProbePort := os.Getenv("PAC_WATCHER_PORT")
	if envProbePort != "" {
		probesPort = envProbePort
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/live", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		_, _ = fmt.Fprint(w, "ok")
	})
	go func() {
		// start the web server on port and accept requests
		log.Printf("Readiness and health check server listening on port %s", probesPort)
		log.Fatal(http.ListenAndServe(":"+probesPort, mux))
	}()

	sharedmain.Main("pac-watcher", reconciler.NewController())
}
