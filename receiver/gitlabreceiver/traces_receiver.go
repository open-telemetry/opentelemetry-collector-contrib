// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package gitlabreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/gitlabreceiver"

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"

	"github.com/gorilla/mux"
	gitlab "gitlab.com/gitlab-org/api/client-go"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
	conventions "go.opentelemetry.io/otel/semconv/v1.38.0"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/gitlabreceiver/internal/metadata"
)

// Pipeline and job status constants are now in constants.go

var (
	// Error messages
	errMissingEndpoint      = errors.New("missing a receiver endpoint")
	errGitlabClient         = errors.New("failed to create gitlab client")
	errUnexpectedEvent      = errors.New("unexpected event type")
	errInvalidHTTPMethod    = errors.New("invalid HTTP method")
	errInvalidHeader        = errors.New("invalid header")
	errMissingHeader        = errors.New("missing header")
	errMissingRequiredField = errors.New("missing required field")
)

// HealthyResponse is now in constants.go

// gitlabReceiver is the main receiver that handles both traces and metrics
// It uses a shared component pattern to support multiple signal types
type gitlabReceiver struct {
	cfg            *Config
	settings       receiver.Settings
	traceConsumer  consumer.Traces
	metricConsumer consumer.Metrics
	obsrecv        *receiverhelper.ObsReport
	server         *http.Server
	shutdownWG     sync.WaitGroup
	logger         *zap.Logger
	gitlabClient   *gitlab.Client
	eventRouter    *eventRouter
}

// gitlabTracesReceiver is kept for backward compatibility with existing pipeline handling code
// It's now embedded in gitlabReceiver
type gitlabTracesReceiver struct {
	logger *zap.Logger
	cfg    *Config
}

func newGitLabReceiver(settings receiver.Settings, cfg *Config) (*gitlabReceiver, error) {
	if cfg.WebHook.NetAddr.Endpoint == "" {
		return nil, errMissingEndpoint
	}

	transport := "http"
	if cfg.WebHook.TLS.HasValue() {
		transport = "https"
	}

	obsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{
		ReceiverID:             settings.ID,
		Transport:              transport,
		ReceiverCreateSettings: settings,
	})
	if err != nil {
		return nil, err
	}

	client, err := gitlab.NewClient("")
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errGitlabClient, err)
	}

	receiver := &gitlabReceiver{
		cfg:          cfg,
		settings:     settings,
		logger:       settings.Logger,
		obsrecv:      obsrecv,
		gitlabClient: client,
		eventRouter:  newEventRouter(settings.Logger, cfg),
	}

	return receiver, nil
}

// newTracesReceiver creates a receiver instance for traces (used by factory)
func newTracesReceiver(settings receiver.Settings, cfg *Config, traceConsumer consumer.Traces) (*gitlabReceiver, error) {
	receiver, err := newGitLabReceiver(settings, cfg)
	if err != nil {
		return nil, err
	}
	receiver.traceConsumer = traceConsumer
	return receiver, nil
}

// newMetricsReceiver creates a receiver instance for metrics (used by factory)
func newMetricsReceiver(settings receiver.Settings, cfg *Config, metricConsumer consumer.Metrics) (*gitlabReceiver, error) {
	receiver, err := newGitLabReceiver(settings, cfg)
	if err != nil {
		return nil, err
	}
	receiver.metricConsumer = metricConsumer
	return receiver, nil
}

func (gtr *gitlabReceiver) Start(ctx context.Context, host component.Host) error {
	endpoint := fmt.Sprintf("%s%s", gtr.cfg.WebHook.NetAddr.Endpoint, gtr.cfg.WebHook.Path)
	gtr.logger.Info("Starting GitLab WebHook receiving server", zap.String("endpoint", endpoint))

	// noop if not nil. if start has not been called before these values should be nil.
	if gtr.server != nil && gtr.server.Handler != nil {
		return nil
	}

	// create listener from config
	ln, err := gtr.cfg.WebHook.ToListener(ctx)
	if err != nil {
		return err
	}

	// use gorilla mux to set up a router
	router := mux.NewRouter()

	// setup health route
	router.HandleFunc(gtr.cfg.WebHook.HealthPath, gtr.handleHealthCheck)

	// setup webhook route for traces
	router.HandleFunc(gtr.cfg.WebHook.Path, gtr.handleWebhook)

	// webhook server standup and configuration
	gtr.server, err = gtr.cfg.WebHook.ToServer(ctx, host.GetExtensions(), gtr.settings.TelemetrySettings, router)
	if err != nil {
		return err
	}
	gtr.logger.Info(
		"Health check now listening at",
		zap.String("health_path",
			fmt.Sprintf("%s%s", gtr.cfg.WebHook.NetAddr.Endpoint, gtr.cfg.WebHook.HealthPath),
		),
	)

	gtr.shutdownWG.Add(1)
	go func() {
		defer gtr.shutdownWG.Done()
		if errHTTP := gtr.server.Serve(ln); !errors.Is(errHTTP, http.ErrServerClosed) && errHTTP != nil {
			componentstatus.ReportStatus(host, componentstatus.NewFatalErrorEvent(errHTTP))
		}
	}()

	return nil
}

func (gtr *gitlabReceiver) Shutdown(ctx context.Context) error {
	if gtr.server != nil {
		err := gtr.server.Shutdown(ctx)
		return err
	}
	gtr.shutdownWG.Wait()
	return nil
}

func (gtr *gitlabReceiver) handleHealthCheck(w http.ResponseWriter, r *http.Request) {
	if r.Body != nil {
		_, err := io.Copy(io.Discard, r.Body)
		if err != nil {
			gtr.failBadReq(r.Context(), w, http.StatusInternalServerError, err, 0)
			return
		}
		_ = r.Body.Close()
	}

	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

		_, _ = w.Write([]byte(HealthyResponse))
}

// handleWebhook handles incoming request sent to the webhook endpoint
// It routes events to appropriate handlers using the event router
func (gtr *gitlabReceiver) handleWebhook(w http.ResponseWriter, r *http.Request) {
	ctx := gtr.obsrecv.StartTracesOp(r.Context())

	// Validate request
	eventType, err := gtr.validateReq(r)
	if err != nil {
		gtr.failBadReq(ctx, w, http.StatusBadRequest, err, 0)
		return
	}

	// Read payload
	payload, err := io.ReadAll(r.Body)
	if err != nil || len(payload) == 0 {
		gtr.failBadReq(ctx, w, http.StatusBadRequest, err, 0)
		return
	}

	// Parse webhook
	event, err := gitlab.ParseWebhook(eventType, payload)
	if err != nil {
		gtr.failBadReq(ctx, w, http.StatusBadRequest, err, 0)
		return
	}

	// Route event to appropriate handler
	result, err := gtr.eventRouter.Route(ctx, event, eventType)
	if err != nil {
		gtr.failBadReq(ctx, w, http.StatusInternalServerError, err, 0)
		return
	}

	// Process result based on type
	spanCount := 0
	if result != nil {
		if result.Traces != nil && result.Traces.SpanCount() > 0 && gtr.traceConsumer != nil {
			if err := gtr.traceConsumer.ConsumeTraces(ctx, *result.Traces); err != nil {
				gtr.failBadReq(ctx, w, http.StatusInternalServerError, err, 0)
				return
			}
			spanCount = result.Traces.SpanCount()
			w.WriteHeader(http.StatusOK)
		} else if result.Metrics != nil && result.Metrics.MetricCount() > 0 && gtr.metricConsumer != nil {
			if err := gtr.metricConsumer.ConsumeMetrics(ctx, *result.Metrics); err != nil {
				gtr.failBadReq(ctx, w, http.StatusInternalServerError, err, 0)
				return
			}
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusNoContent)
		}
	} else {
		w.WriteHeader(http.StatusNoContent)
	}

	gtr.obsrecv.EndTracesOp(ctx, metadata.Type.String(), spanCount, nil)
}

func (gtr *gitlabReceiver) validateReq(r *http.Request) (gitlab.EventType, error) {
	if r.Method != http.MethodPost {
		return "", errInvalidHTTPMethod
	}

	if gtr.cfg.WebHook.Secret != "" {
		secret := r.Header.Get(defaultGitLabSecretTokenHeader)
		if secret != gtr.cfg.WebHook.Secret {
			return "", fmt.Errorf("%w: %s", errInvalidHeader, defaultGitLabSecretTokenHeader)
		}
	}

	for key, value := range gtr.cfg.WebHook.RequiredHeaders {
		if r.Header.Get(key) != string(value) {
			return "", fmt.Errorf("%w: %s", errInvalidHeader, key)
		}
	}

	eventType := gitlab.WebhookEventType(r)
	if eventType == "" {
		return "", fmt.Errorf("%w: %s", errMissingHeader, defaultGitLabEventHeader)
	}
	return eventType, nil
}

// validatePipelineEvent is now in pipeline_event_handler.go to avoid duplication

func (gtr *gitlabReceiver) failBadReq(ctx context.Context,
	w http.ResponseWriter,
	httpStatusCode int,
	err error,
	spanCount int,
) {
	defer gtr.obsrecv.EndTracesOp(ctx, metadata.Type.String(), spanCount, err)

	if err == nil {
		w.WriteHeader(httpStatusCode)
		return
	}

	jsonResp, marshalErr := json.Marshal(err.Error())
	if marshalErr != nil {
		gtr.logger.Warn("failed to marshall error to json", zap.Error(marshalErr))
		w.WriteHeader(httpStatusCode)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpStatusCode)

	if _, writeErr := w.Write(jsonResp); writeErr != nil {
		gtr.logger.Warn("failed to write json response", zap.Error(writeErr))
	}

	gtr.logger.Debug(string(jsonResp), zap.Int(string(conventions.HTTPResponseStatusCodeKey), httpStatusCode), zap.Error(err))
}
