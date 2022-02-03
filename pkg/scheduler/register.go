package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (s *scheduler) Register() http.HandlerFunc {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		pipelineRun, repository, validationErr := s.validateRequest(response, request)
		if validationErr != nil {
			return
		}

		err := s.syncManager.Register(pipelineRun, repository, s.pipelineClient)
		if err != nil {
			s.logger.Error("failed to register pipelinerun", err)
			responseWriter(s.logger, http.StatusInternalServerError, "failed to register Pipelinerun", response)
			return
		}

		responseWriter(s.logger, http.StatusOK, "request registered successfully!", response)
	})
}

func (s *scheduler) validateRequest(response http.ResponseWriter, request *http.Request) (*v1beta1.PipelineRun, *v1alpha1.Repository, error) {
	vars := mux.Vars(request)
	prNamespace := vars["namespace"]
	prName := vars["name"]

	// check if pipelinerun exists with pending status
	pipelineRun, err := s.pipelineClient.TektonV1beta1().PipelineRuns(prNamespace).Get(context.Background(), prName, v1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			errStr := fmt.Sprintf("pipelinerun doesn't exist %s/%s", prNamespace, prName)
			s.logger.Error(errStr)
			responseWriter(s.logger, http.StatusBadRequest, errStr, response)
			return nil, nil, fmt.Errorf(errStr)
		}
		errStr := "failed to get pipelinerun"
		s.logger.Errorf("%v : %v ", errStr, err)
		responseWriter(s.logger, http.StatusInternalServerError, errStr, response)
		return nil, nil, fmt.Errorf(errStr)
	}

	// pipelineRun must be in pending state to be registered
	if pipelineRun.Spec.Status != v1beta1.PipelineRunSpecStatusPending {
		errStr := fmt.Sprintf("pipelineRun %s/%s is not in pending state, unable to register", prNamespace, prName)
		s.logger.Error(errStr)
		responseWriter(s.logger, http.StatusBadRequest, errStr, response)
		return nil, nil, fmt.Errorf(errStr)
	}

	// check if repository name annotation exist on pipelineRun
	repoName, ok := pipelineRun.Annotations[RepoNameAnnotation]
	if !ok {
		errStr := fmt.Sprintf("failed to find repository name annotation on pipelinerun %s/%s", prName, prNamespace)
		s.logger.Error(errStr)
		responseWriter(s.logger, http.StatusBadRequest, errStr, response)
		return nil, nil, fmt.Errorf(errStr)
	}

	// check if repository with repoName exist in pipelineRun Namespace
	repo, err := s.pacClient.PipelinesascodeV1alpha1().Repositories(pipelineRun.Namespace).Get(context.Background(), repoName, v1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			errStr := fmt.Sprintf("repository doesn't exist %s/%s", prNamespace, repoName)
			s.logger.Error(errStr)
			responseWriter(s.logger, http.StatusBadRequest, errStr, response)
			return nil, nil, fmt.Errorf(errStr)
		}
		errStr := "failed to get repository"
		s.logger.Errorf("%v : %v ", errStr, err)
		responseWriter(s.logger, http.StatusInternalServerError, errStr, response)
		return nil, nil, fmt.Errorf(errStr)
	}

	if repo.Spec.ConcurrencyLimit == 0 {
		errStr := fmt.Sprintf("invalid concurrency limit for repository : %s", repo.Name)
		s.logger.Error(errStr)
		responseWriter(s.logger, http.StatusBadRequest, errStr, response)
		return nil, nil, fmt.Errorf(errStr)
	}

	return pipelineRun, repo, nil
}

type Response struct {
	Message string `json:"message"`
}

func responseWriter(logger *zap.SugaredLogger, statusCode int, message string, response http.ResponseWriter) {
	body := Response{
		Message: message,
	}
	response.WriteHeader(statusCode)
	if err := json.NewEncoder(response).Encode(body); err != nil {
		logger.Errorf("failed to write back response: %v", err)
	}
}
