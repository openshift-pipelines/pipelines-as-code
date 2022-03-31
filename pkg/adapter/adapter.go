package adapter

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/kubeinteraction"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/version"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider/bitbucketcloud"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider/bitbucketserver"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider/github"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider/gitlab"
	"go.uber.org/zap"
	"knative.dev/eventing/pkg/adapter/v2"
	"knative.dev/pkg/logging"
)

type envConfig struct {
	adapter.EnvConfig
}

func NewEnvConfig() adapter.EnvConfigAccessor {
	return &envConfig{}
}

type listener struct {
	run    *params.Run
	kint   *kubeinteraction.Interaction
	logger *zap.SugaredLogger
}

var _ adapter.Adapter = (*listener)(nil)

func New(run *params.Run, k *kubeinteraction.Interaction) adapter.AdapterConstructor {
	return func(ctx context.Context, processed adapter.EnvConfigAccessor, ceClient cloudevents.Client) adapter.Adapter {
		return &listener{
			logger: logging.FromContext(ctx),
			run:    run,
			kint:   k,
		}
	}
}

func (l *listener) Start(ctx context.Context) error {
	l.run.Clients.Log.Infof("Starting Pipelines as Code version: %s", version.Version)

	mux := http.NewServeMux()

	// for handling probes
	mux.HandleFunc("/live", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		fmt.Fprint(w, "ok")
	})

	mux.HandleFunc("/", l.handleEvent())

	srv := &http.Server{
		Addr: ":8080",
		Handler: http.TimeoutHandler(mux,
			10*time.Second, "Listener Timeout!\n"),
	}

	// TODO: support TLS/Certs
	if err := srv.ListenAndServe(); err != nil {
		return err
	}

	return nil
}

func (l listener) handleEvent() http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		ctx := context.Background()

		// event body
		payload, err := ioutil.ReadAll(request.Body)
		if err != nil {
			l.run.Clients.Log.Errorf("failed to read body : %v", err)
			return
		}

		// figure out which provider request coming from
		gitProvider, logger, err := l.detectProvider(&request.Header, string(payload))
		if err != nil || gitProvider == nil {
			l.logger.Errorf("invalid event or got error while processing : %v", err)
			return
		}

		// TODO: decouple logger from clients so each event
		// has a logger with its own fields
		// eg. logger.With("provider", "github", "event", request.Header.Get("X-GitHub-Delivery"))
		l.run.Clients.Log = logger

		s := sinker{
			run:   l.run,
			vcx:   gitProvider,
			kint:  l.kint,
			event: info.NewEvent(),
		}

		// clone the request to use it further
		localRequest := request.Clone(request.Context())

		go func() {
			s.processEvent(ctx, localRequest, payload)
		}()

		response.WriteHeader(http.StatusAccepted)
		fmt.Fprintf(response, `{"status": "%d", "message": "accepted"}`, http.StatusAccepted)
	}
}

func (l listener) detectProvider(reqHeader *http.Header, reqBody string) (provider.Interface, *zap.SugaredLogger, error) {
	log := *l.logger

	gitHub := &github.Provider{}
	isGH, processReq, logger, err := gitHub.Detect(reqHeader, reqBody, &log)
	if isGH {
		if err != nil {
			return nil, logger, err
		}
		if processReq {
			return gitHub, logger, nil
		}
		return nil, nil, nil
	}

	bitServer := &bitbucketserver.Provider{}
	isBitServer, processReq, logger, err := bitServer.Detect(reqHeader, reqBody, &log)
	if isBitServer {
		if err != nil {
			return nil, logger, err
		}
		if processReq {
			return bitServer, logger, nil
		}
		return nil, nil, nil
	}

	gitLab := &gitlab.Provider{}
	isGitlab, processReq, logger, err := gitLab.Detect(reqHeader, reqBody, &log)
	if isGitlab {
		if err != nil {
			return nil, logger, err
		}
		if processReq {
			return gitLab, logger, nil
		}
		return nil, nil, nil
	}

	bitCloud := &bitbucketcloud.Provider{}
	isBitCloud, processReq, logger, err := bitCloud.Detect(reqHeader, reqBody, &log)
	if isBitCloud {
		if err != nil {
			return nil, logger, err
		}
		if processReq {
			return bitCloud, logger, nil
		}
		return nil, nil, nil
	}

	return nil, nil, fmt.Errorf("no supported Git Provider is detected")
}
