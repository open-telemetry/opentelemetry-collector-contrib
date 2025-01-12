// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package githubreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/githubreceiver"

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"

	"github.com/google/go-github/v68/github"
	"github.com/gorilla/mux"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
	// "go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

var errMissingEndpoint = errors.New("missing a receiver endpoint")

const healthyResponse = `{"text": "GitHub receiver webhook is healthy"}`

type githubTracesReceiver struct {
	traceConsumer consumer.Traces
	cfg           *Config
	server        *http.Server
	shutdownWG    sync.WaitGroup
	settings      receiver.Settings
	logger        *zap.Logger
	obsrecv       *receiverhelper.ObsReport
	ghClient      *github.Client
}

func newTracesReceiver(
	params receiver.Settings,
	config *Config,
	traceConsumer consumer.Traces,
) (*githubTracesReceiver, error) {
	if config.WebHook.Endpoint == "" {
		return nil, errMissingEndpoint
	}

	transport := "http"
	if config.WebHook.TLSSetting != nil {
		transport = "https"
	}

	obsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{
		ReceiverID:             params.ID,
		Transport:              transport,
		ReceiverCreateSettings: params,
	})
	if err != nil {
		return nil, err
	}

	client := github.NewClient(nil)

	gtr := &githubTracesReceiver{
		traceConsumer: traceConsumer,
		cfg:           config,
		settings:      params,
		logger:        params.Logger,
		obsrecv:       obsrecv,
		ghClient:      client,
	}

	return gtr, nil
}

func (gtr *githubTracesReceiver) Start(ctx context.Context, host component.Host) error {
	endpoint := fmt.Sprintf("%s%s", gtr.cfg.WebHook.Endpoint, gtr.cfg.WebHook.Path)
	gtr.logger.Info("Starting GitHub WebHook receiving server", zap.String("endpoint", endpoint))

	// noop if not nil. if start has not been called before these values should be nil.
	if gtr.server != nil && gtr.server.Handler != nil {
		return nil
	}

	// create listener from config
	ln, err := gtr.cfg.WebHook.ServerConfig.ToListener(ctx)
	if err != nil {
		return err
	}

	// use gorilla mux to set up a router
	router := mux.NewRouter()

	// setup health route
    router.HandleFunc(gtr.cfg.WebHook.HealthPath, gtr.handleHealthCheck)
    
    // setup webhook route for traces
    router.HandleFunc(gtr.cfg.WebHook.Path, gtr.handleReq)

	// webhook server standup and configuration
	gtr.server, err = gtr.cfg.WebHook.ServerConfig.ToServer(ctx, host, gtr.settings.TelemetrySettings, router)
	if err != nil {
		return err
	}

	gtr.logger.Info("Health check now listening at", zap.String("health_path", gtr.cfg.WebHook.HealthPath))

	gtr.shutdownWG.Add(1)
	go func() {
		defer gtr.shutdownWG.Done()

		if errHTTP := gtr.server.Serve(ln); !errors.Is(errHTTP, http.ErrServerClosed) && errHTTP != nil {
			componentstatus.ReportStatus(host, componentstatus.NewFatalErrorEvent(errHTTP))
		}
	}()

	return nil
}

func (gtr *githubTracesReceiver) Shutdown(_ context.Context) error {
	// server must exist to be closed.
	if gtr.server == nil {
		return nil
	}

	err := gtr.server.Close()
	gtr.shutdownWG.Wait()
	return err
}

// handleReq handles incoming request sent to the webhook endoint. On success
// returns a 200 response code.
func (gtr *githubTracesReceiver) handleReq(w http.ResponseWriter, req *http.Request) {
    // ctx := gtr.obsrecv.StartTracesOp(req.Context())
    
    p, err := github.ValidatePayload(req, []byte(gtr.cfg.WebHook.Secret))
    if err != nil {
        gtr.logger.Sugar().Debugf("unable to validate payload", zap.Error(err))
        http.Error(w, "invalid payload", http.StatusBadRequest)
        return
    }

    eventType := github.WebHookType(req)
    event, err := github.ParseWebHook(eventType, p)
    if err != nil {
        gtr.logger.Sugar().Debugf("failed to parse event", zap.Error(err))
        http.Error(w, "failed to parse event", http.StatusBadRequest)
        return
    }

    // payload, err := github.Parse

    switch e := event.(type) {
    case *github.WorkflowRunEvent:
        if e.GetWorkflowRun().GetStatus() == "completed" {
            var rawEvent []interface{}
            event := append(rawEvent, e.GetWorkflowRun(), e.GetInstallation(), e.GetSender(), e.GetOrg(), e.GetRepo(), e.GetWorkflow())
            gtr.logger.Debug("event", zap.Any("event", event))
        }
		// if e.GetWorkflowRun().GetStatus() != "completed" {
		// 	gtr.logger.Debug("skipping non-completed WorkflowRunEvent", zap.String("status", e.GetWorkflowRun().GetStatus()))
		// 	w.WriteHeader(http.StatusNoContent)
		// 	return
		// }
        return
    case *github.WorkflowJobEvent:
		if e.GetWorkflowJob().GetStatus() != "completed" {
			gtr.logger.Debug("skipping non-completed WorkflowJobEvent", zap.String("status", e.GetWorkflowJob().GetStatus()))
			w.WriteHeader(http.StatusNoContent)
			return
		}
        return
    default:
        gtr.logger.Sugar().Debugf("event type not supported", zap.String("event_type", eventType))
        http.Error(w, "event type not supported", http.StatusBadRequest)
        return
    }

    // TODO: Figure this out
	// gtr.obsrecv.EndTracesOp(ctx, "protobuf", td.SpanCount(), err)
}

// Simple healthcheck endpoint.
func (gtr *githubTracesReceiver) handleHealthCheck(w http.ResponseWriter, _ *http.Request) {
	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	_, _ = w.Write([]byte(healthyResponse))
}

func (gtr *githubTracesReceiver) genSvcName(e interface{}) (string, error) {
    // uses e.(type)
    if e.(type) == *github.WorkflowRunEvent  || e.(type) == *github.WorkflowJobEvent {
    }
    return nil, nil
}

// maybe accept event.type here directly.
// func (gtr *githubTracesReceiver) eventToSpan(e interface{}) (ptrace.Traces, error) {
//     t := ptrace.NewTraces()
//     r := t.ResourceSpans().AppendEmpty()
//     s := r.ScopeSpans().AppendEmpty()
//
//     switch e := e.(type) {
// 	case *github.WorkflowJobEvent:
// 		jobResource := resourceSpans.Resource()
// 		createResourceAttributes(jobResource, e, config, logger)
//
// 		traceID, err := generateTraceID(e.GetWorkflowJob().GetRunID(), int(e.GetWorkflowJob().GetRunAttempt()))
// 		if err != nil {
// 			logger.Error("Failed to generate trace ID", zap.Error(err))
// 			return ptrace.Traces{}, fmt.Errorf("failed to generate trace ID: %w", err)
// 		}
//
// 		parentSpanID := createParentSpan(scopeSpans, e.GetWorkflowJob().Steps, e.GetWorkflowJob(), traceID, logger)
// 		processSteps(scopeSpans, e.GetWorkflowJob().Steps, e.GetWorkflowJob(), traceID, parentSpanID, logger)
//
// 	case *github.WorkflowRunEvent:
// 		runResource := resourceSpans.Resource()
//
// 		traceID, err := generateTraceID(e.GetWorkflowRun().GetID(), e.GetWorkflowRun().GetRunAttempt())
// 		if err != nil {
// 			logger.Error("Failed to generate trace ID", zap.Error(err))
// 			return ptrace.Traces{}, fmt.Errorf("failed to generate trace ID: %w", err)
// 		}
//
// 		createResourceAttributes(runResource, e, config, logger)
// 		_, err = createRootSpan(resourceSpans, e, traceID, logger)
// 		if err != nil {
// 			logger.Error("Failed to create root span", zap.Error(err))
// 			return ptrace.Traces{}, fmt.Errorf("failed to create root span: %w", err)
// 		}
//
// 	default:
// 		logger.Error("unknown event type, dropping payload")
// 		return ptrace.Traces{}, fmt.Errorf("unknown event type")
// 	}
//
//
//     return nil, nil
// }
