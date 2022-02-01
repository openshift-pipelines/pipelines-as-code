package scheduler

import (
	"fmt"
	"net/http"
)

type Scheduler interface {
	Register() http.HandlerFunc
}

var _ Scheduler = &scheduler{}

func New() Scheduler {
	return &scheduler{}
}

type scheduler struct {
}

func (s *scheduler) Register() http.HandlerFunc {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.WriteHeader(http.StatusOK)
		fmt.Fprint(response, "ok!")
	})
}
