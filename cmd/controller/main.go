package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/scheduler"
)

func main() {

	router := mux.NewRouter()

	router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		fmt.Fprint(w, "ok!")
	})

	s := scheduler.New()

	router.HandleFunc("/register/pipelinerun/{namespace}/{name}", s.Register())

	if err := http.ListenAndServe(":8080", router); err != nil {
		log.Fatalf("failed to start server: %v", err)
	}
}
