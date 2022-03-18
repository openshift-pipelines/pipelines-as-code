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
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider/github"
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
	mux := http.NewServeMux()

	// TODO: to be used in health check in deployment
	mux.HandleFunc("/hello", func(w http.ResponseWriter, r *http.Request) {
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

		// figure out which provider request coming from
		gitProvider, logger, err := l.whichProvider(request)
		if err != nil {
			l.logger.Error(err)
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
			event: &info.Event{},
		}

		// clone the request to use it further
		localRequest := request.Clone(request.Context())

		// event payload
		payload, err := ioutil.ReadAll(request.Body)
		if err != nil {
			l.run.Clients.Log.Errorf("failed to read body : %v", err)
			return
		}

		go func() {
			s.processEvent(ctx, localRequest, payload)
		}()

		response.WriteHeader(http.StatusAccepted)
	}
}

func (l listener) whichProvider(request *http.Request) (provider.Interface, *zap.SugaredLogger, error) {
	res := request.Header.Get("X-Github-Event")
	if res != "" {
		logger := l.logger.With("provider", "github", "event", request.Header.Get("X-GitHub-Delivery"))
		return &github.Provider{}, logger, nil
	}

	return nil, nil, fmt.Errorf("no supported Git Provider is detected")
}
