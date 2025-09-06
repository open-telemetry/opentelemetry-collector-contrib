// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kedascalerexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kedascalerexporter"

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/mux"
	jsoniter "github.com/json-iterator/go"
	"github.com/kedacore/keda/v2/pkg/scalers/externalscaler"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

var contentType = http.CanonicalHeaderKey("Content-Type")

type kedaScalerExporter struct {
	config           Config
	settings         component.TelemetrySettings
	scaler           externalscaler.ExternalScalerServer
	shutdownWG       sync.WaitGroup
	serverGRPC       *grpc.Server
	monitoringServer *http.Server
	metricStore      *OTLPMetricStore
	cancel           context.CancelFunc
}

type QueryBody struct {
	Query string   `json:"query"`
	_     struct{} `json:"-"`
}

func newKedaScalerExporter(
	ctx context.Context,
	config *Config,
	set exporter.Settings,
) (*kedaScalerExporter, error) {
	metricStore, err := NewOTLPMetricStore(ctx, config, set.Logger)
	if err != nil {
		return nil, err
	}
	scaler := newKedaScaler(set.Logger, metricStore)

	return &kedaScalerExporter{
		config:      *config,
		settings:    set.TelemetrySettings,
		metricStore: metricStore,
		scaler:      scaler,
	}, nil
}

func validateContentType(req *http.Request) error {
	reqContentType := req.Header.Get(contentType)
	if reqContentType != "" {
		if _, _, err := mime.ParseMediaType(reqContentType); err != nil {
			return fmt.Errorf("invalid Content-Type: %w", err)
		}
	}
	return nil
}

func (e *kedaScalerExporter) handleCleanupInterval(ctx context.Context) {
	ticker := time.NewTicker(e.config.CleanupInterval)
	go func() {
		for {
			select {
			case <-ticker.C:
				e.metricStore.CleanupExpiredData()
			case <-ctx.Done():
				ticker.Stop()
				return
			}
		}
	}()
}

func (e *kedaScalerExporter) HandleGetSeries(w http.ResponseWriter, _ *http.Request) {
	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json := jsoniter.ConfigCompatibleWithStandardLibrary
	_ = json.NewEncoder(w).Encode(e.metricStore.GetSeries())
}

func (e *kedaScalerExporter) HandleGetSeriesNames(w http.ResponseWriter, _ *http.Request) {
	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	json := jsoniter.ConfigCompatibleWithStandardLibrary
	_ = json.NewEncoder(w).Encode(e.metricStore.GetSeriesNames())
}

func (e *kedaScalerExporter) HandleQuery(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "application/json")

	if err := validateContentType(r); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		e.settings.Logger.Error("Invalid content type", zap.Error(err))
		return
	}

	body := r.Body

	defer func() {
		if body, ok := body.(io.Closer); ok {
			if closeErr := body.Close(); closeErr != nil {
				e.settings.Logger.Warn("failed to close reader", zap.Error(closeErr))
			}
		}
	}()

	bodyBytes, err := io.ReadAll(body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		e.settings.Logger.Error("Failed to read request body", zap.Error(err))
		return
	}

	var queryBody QueryBody
	json := jsoniter.ConfigCompatibleWithStandardLibrary
	err = json.Unmarshal(bodyBytes, &queryBody)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		e.settings.Logger.Error("Failed to parse body", zap.Error(err))
		return
	}

	result, err := e.metricStore.Query(queryBody.Query, time.Now())
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		e.settings.Logger.Error("Failed to query metrics", zap.Error(err))
		return
	}

	w.WriteHeader(http.StatusOK)
	response := map[string]any{
		"result": result,
	}

	_ = json.NewEncoder(w).Encode(response)
}

func (e *kedaScalerExporter) Start(ctx context.Context, host component.Host) error {
	var err error
	var cln net.Listener

	if e.config.MonitoringHTTP != nil {
		cln, err = e.config.MonitoringHTTP.ToListener(ctx)
		if err != nil {
			return fmt.Errorf("failed to bind to monitoring http address %q: %w",
				e.config.MonitoringHTTP.Endpoint, err)
		}

		nr := mux.NewRouter()
		nr.HandleFunc("/data", e.HandleGetSeries).Methods(http.MethodGet)
		nr.HandleFunc("/names", e.HandleGetSeriesNames).Methods(http.MethodGet)
		nr.HandleFunc("/query", e.HandleQuery).Methods(http.MethodPost)
		e.monitoringServer, err = e.config.MonitoringHTTP.ToServer(ctx, host, e.settings, nr)
		if err != nil {
			return err
		}

		e.settings.Logger.Info(
			"Starting HTTP server for monitoring",
			zap.String("endpoint", e.config.MonitoringHTTP.Endpoint),
		)

		e.shutdownWG.Add(1)
		go func() {
			defer e.shutdownWG.Done()
			if errHTTP := e.monitoringServer.Serve(cln); !errors.Is(
				errHTTP,
				http.ErrServerClosed,
			) &&
				errHTTP != nil {
				componentstatus.ReportStatus(host, componentstatus.NewFatalErrorEvent(errHTTP))
			}
		}()
	}

	if e.config.Scaler != nil {
		if e.serverGRPC, err = e.config.Scaler.ToServer(ctx, host, e.settings); err != nil {
			return err
		}

		externalscaler.RegisterExternalScalerServer(
			e.serverGRPC,
			newKedaScaler(
				e.settings.Logger,
				e.metricStore,
			),
		)

		e.settings.Logger.Info(
			"Starting GRPC server for keda scaler",
			zap.String("endpoint", e.config.Scaler.NetAddr.Endpoint),
		)

		var gln net.Listener
		if gln, err = e.config.Scaler.NetAddr.Listen(ctx); err != nil {
			return err
		}

		e.shutdownWG.Add(1)

		go func() {
			defer e.shutdownWG.Done()

			if errGrpc := e.serverGRPC.Serve(gln); errGrpc != nil &&
				!errors.Is(errGrpc, grpc.ErrServerStopped) {
				componentstatus.ReportStatus(host, componentstatus.NewFatalErrorEvent(errGrpc))
			}
		}()
	}

	ctx, cancel := context.WithCancel(ctx)
	e.cancel = cancel
	go e.handleCleanupInterval(ctx)

	return nil
}

func (e *kedaScalerExporter) Shutdown(ctx context.Context) error {
	if e.cancel != nil {
		e.cancel()
	}
	if e.serverGRPC != nil {
		e.serverGRPC.GracefulStop()
	}

	if e.monitoringServer != nil {
		if err := e.monitoringServer.Shutdown(ctx); err != nil {
			e.settings.Logger.Error("Failed to shutdown monitoring server", zap.Error(err))
		}
	}

	e.shutdownWG.Wait()
	return nil
}

func (e *kedaScalerExporter) ConsumeMetrics(_ context.Context, md pmetric.Metrics) error {
	return e.metricStore.ProcessMetrics(md)
}
