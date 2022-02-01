package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/gorilla/mux"
	pacClientSet "github.com/openshift-pipelines/pipelines-as-code/pkg/generated/clientset/versioned"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/scheduler"
	pipelineClientSet "github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	"go.uber.org/zap"
	"k8s.io/client-go/rest"
)

func main() {

	logger := initLogger()

	clusterConfig, err := rest.InClusterConfig()
	if err != nil {
		log.Fatalf("Failed to get in cluster config: %v", err)
	}

	pipelineCS, err := pipelineClientSet.NewForConfig(clusterConfig)
	if err != nil {
		log.Fatalf("Failed to create pipeline client set: %v", err)
	}

	pacCS, err := pacClientSet.NewForConfig(clusterConfig)
	if err != nil {
		log.Fatalf("Failed to create pipeline client set: %v", err)
	}

	router := mux.NewRouter()

	router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		fmt.Fprint(w, "ok!")
	})

	s := scheduler.New(pipelineCS, pacCS, logger)

	router.HandleFunc("/register/pipelinerun/{namespace}/{name}", s.Register())

	if err := http.ListenAndServe(":8080", router); err != nil {
		log.Fatalf("failed to start server: %v", err)
	}
}

func initLogger() *zap.SugaredLogger {
	prod, _ := zap.NewProduction()
	logger := prod.Sugar()
	defer func() {
		_ = logger.Sync() // flushes buffer, if any
	}()
	return logger
}
