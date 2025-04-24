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

	faro "github.com/grafana/faro/pkg/go"
	farotranslator "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/faro"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
	"go.uber.org/zap"
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
	if cfg.TLSSetting != nil {
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
	cfg        *Config
	serverHTTP *http.Server

	nextTraces consumer.Traces
	nextLogs   consumer.Logs

	obsrepHTTP *receiverhelper.ObsReport

	settings *receiver.Settings
}

func (r *faroReceiver) Start(ctx context.Context, host component.Host) error {
	r.settings.Logger.Info("Starting FaroReceiver")
	if r.nextTraces == nil && r.nextLogs == nil {
		return fmt.Errorf("at least one consumer (traces or logs) must be registered")
	}
	return r.startHTTPServer(ctx, host)
}

func (r *faroReceiver) Shutdown(ctx context.Context) error {
	if r.serverHTTP != nil {
		return r.serverHTTP.Shutdown(ctx)
	}
	return nil
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

	go func() {
		if err := r.serverHTTP.Serve(listener); !errors.Is(err, http.ErrServerClosed) && err != nil {
			r.settings.Logger.Error("Failed to start HTTP server", zap.Error(err))
			componentstatus.ReportStatus(host, componentstatus.NewFatalErrorEvent(err))
		}
	}()

	r.settings.Logger.Info("HTTP server started", zap.String("address", r.serverHTTP.Addr))
	return nil
}

func (r *faroReceiver) handleFaroRequest(resp http.ResponseWriter, req *http.Request) {
	resp.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	resp.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	// Preflight request
	if req.Method == http.MethodOptions {
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
		traces, err := farotranslator.TranslateToTraces(req.Context(), payload)
		if err == nil {
			err = r.nextTraces.ConsumeTraces(req.Context(), traces)
			if err != nil {
				errors = append(errors, fmt.Sprintf("failed to push traces: %v", err))
			}
		} else {
			r.settings.Logger.Debug("Faro traces are nil, skipping")
		}
	} else {
		r.settings.Logger.Debug("Traces consumer not registered, skipping")
	}

	if r.nextLogs != nil {
		logs, err := farotranslator.TranslateToLogs(req.Context(), payload)
		if err == nil {
			err = r.nextLogs.ConsumeLogs(req.Context(), logs)
			if err != nil {
				errors = append(errors, fmt.Sprintf("failed to push logs: %v", err))
			}
		} else {
			r.settings.Logger.Debug("Faro logs are nil, skipping")
		}
	} else {
		r.settings.Logger.Debug("Logs consumer not registered, skipping")
	}

	if len(errors) > 0 {
		r.settings.Logger.Error("Failed to process Faro payload", zap.Any("payload", payload), zap.Any("errors", errors))
		r.errorHandler(resp, req, strings.Join(errors, "\n"), http.StatusInternalServerError)
		return
	}

	resp.WriteHeader(http.StatusOK)
}

func (r *faroReceiver) errorHandler(w http.ResponseWriter, _ *http.Request, errMsg string, statusCode int) {
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(statusCode)
	if _, err := w.Write([]byte(errMsg)); err != nil {
		r.settings.Logger.Error("Could not write response", zap.String("message", errMsg), zap.Error(err))
	}
}
