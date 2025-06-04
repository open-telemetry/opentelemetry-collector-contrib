// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package faroreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/faroreceiver"

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	faro "github.com/grafana/faro/pkg/go"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
	"go.uber.org/zap"

	farotranslator "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/faro"
)

const faroPath = "/"

func newFaroReceiver(cfg *Config, set *receiver.Settings) (*faroReceiver, error) {
	err := cfg.Validate()
	if err != nil {
		return nil, err
	}

	set.Logger.Debug("Valid FaroReceiver config", zap.Any("config", cfg))
	r := &faroReceiver{
		cfg:      cfg,
		settings: set,
	}

	transport := "http"
	if cfg.TLS != nil {
		transport = "https"
	}

	r.obsrepHTTP, err = receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{
		ReceiverID:             set.ID,
		Transport:              transport,
		ReceiverCreateSettings: *set,
	})
	if err != nil {
		return nil, err
	}

	return r, nil
}

type faroReceiver struct {
	cfg *Config

	serverHTTP *http.Server
	shutdownWg sync.WaitGroup

	nextTraces consumer.Traces
	nextLogs   consumer.Logs

	obsrepHTTP *receiverhelper.ObsReport

	settings *receiver.Settings
}

func (r *faroReceiver) Start(ctx context.Context, host component.Host) error {
	r.settings.Logger.Info("Starting FaroReceiver")
	if r.nextTraces == nil && r.nextLogs == nil {
		return errors.New("at least one consumer (traces or logs) must be registered")
	}
	return r.startHTTPServer(ctx, host)
}

func (r *faroReceiver) Shutdown(ctx context.Context) error {
	var err error
	if r.serverHTTP != nil {
		err = r.serverHTTP.Shutdown(ctx)
	}

	r.shutdownWg.Wait()
	return err
}

func (r *faroReceiver) RegisterTracesConsumer(tc consumer.Traces) {
	r.nextTraces = tc
}

func (r *faroReceiver) RegisterLogsConsumer(lc consumer.Logs) {
	r.nextLogs = lc
}

func (r *faroReceiver) startHTTPServer(ctx context.Context, host component.Host) error {
	// Noop if not nil
	if r.serverHTTP != nil {
		return nil
	}

	r.settings.Logger.Info("Starting HTTP server", zap.String("endpoint", r.cfg.Endpoint))

	httpMux := http.NewServeMux()
	httpMux.HandleFunc(faroPath, func(resp http.ResponseWriter, req *http.Request) {
		r.handleFaroRequest(resp, req)
	})
	var err error
	if r.serverHTTP, err = r.cfg.ToServer(
		ctx,
		host,
		r.settings.TelemetrySettings,
		httpMux,
		confighttp.WithErrorHandler(r.errorHandler),
	); err != nil {
		r.settings.Logger.Error("Failed to start HTTP server", zap.Error(err))
		return err
	}

	listener, err := r.cfg.ToListener(ctx)
	if err != nil {
		r.settings.Logger.Error("Failed to create faro receiver listener", zap.Error(err))
		return err
	}

	r.shutdownWg.Add(1)
	go func() {
		defer r.shutdownWg.Done()
		if err := r.serverHTTP.Serve(listener); !errors.Is(err, http.ErrServerClosed) && err != nil {
			r.settings.Logger.Error("Failed to start HTTP server", zap.Error(err))
			componentstatus.ReportStatus(host, componentstatus.NewFatalErrorEvent(err))
		}
	}()

	r.settings.Logger.Info("HTTP server started", zap.String("address", r.serverHTTP.Addr))
	return nil
}

func (r *faroReceiver) handleFaroRequest(resp http.ResponseWriter, req *http.Request) {
	// Preflight request
	if req.Method == http.MethodOptions {
		resp.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
		resp.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		resp.WriteHeader(http.StatusOK)
		return
	}

	if req.Method != http.MethodPost {
		r.errorHandler(resp, req, "only POST method is supported", http.StatusMethodNotAllowed)
		return
	}

	if req.Header.Get("Content-Type") != "application/json" {
		r.errorHandler(resp, req, "invalid Content-Type", http.StatusUnsupportedMediaType)
		return
	}

	body, err := io.ReadAll(req.Body)
	if err != nil {
		r.errorHandler(resp, req, fmt.Sprintf("failed to read request body: %v", err), http.StatusBadRequest)
		return
	}
	defer req.Body.Close()

	var payload faro.Payload
	if err := json.Unmarshal(body, &payload); err != nil {
		r.errorHandler(resp, req, fmt.Sprintf("failed to parse Faro payload: %v", err), http.StatusBadRequest)
		return
	}

	var errors []string

	if r.nextTraces != nil {
		traces, translatorErr := farotranslator.TranslateToTraces(req.Context(), payload)
		if translatorErr != nil {
			errors = append(errors, fmt.Sprintf("failed to translate traces: %v", translatorErr))
		} else {
			if traces.SpanCount() == 0 {
				r.settings.Logger.Debug("Faro traces are nil, skipping")
			} else {
				consumeErr := r.nextTraces.ConsumeTraces(req.Context(), traces)
				if consumeErr != nil {
					errors = append(errors, fmt.Sprintf("failed to pass traces to next consumer: %v", consumeErr))
				}
			}
		}
	}

	if r.failOnErrorsAndLog(errors, payload, resp, req) {
		return
	}

	if r.nextLogs != nil {
		logs, translatorErr := farotranslator.TranslateToLogs(req.Context(), payload)
		if translatorErr != nil {
			errors = append(errors, fmt.Sprintf("failed to translate logs: %v", translatorErr))
		} else {
			if logs.LogRecordCount() == 0 {
				r.settings.Logger.Debug("Faro logs are nil, skipping")
			} else {
				consumeErr := r.nextLogs.ConsumeLogs(req.Context(), logs)
				if consumeErr != nil {
					errors = append(errors, fmt.Sprintf("failed to pass logs to next consumer: %v", consumeErr))
				}
			}
		}
	}

	if r.failOnErrorsAndLog(errors, payload, resp, req) {
		return
	}

	resp.WriteHeader(http.StatusOK)
}

type errorResponse struct {
	Error   string `json:"error"`
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (r *faroReceiver) errorHandler(w http.ResponseWriter, _ *http.Request, errMsg string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	resp := errorResponse{
		Error:   http.StatusText(statusCode),
		Code:    statusCode,
		Message: errMsg,
	}

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		r.settings.Logger.Error("Could not write JSON response",
			zap.String("message", errMsg),
			zap.Error(err))
	}
}

func (r *faroReceiver) failOnErrorsAndLog(errors []string, payload any, resp http.ResponseWriter, req *http.Request) bool {
	if len(errors) > 0 {
		r.settings.Logger.Error("Failed to process Faro payload", zap.Any("payload", payload), zap.Any("errors", errors))
		r.errorHandler(resp, req, strings.Join(errors, "\n"), http.StatusInternalServerError)
		return true
	}
	return false
}
