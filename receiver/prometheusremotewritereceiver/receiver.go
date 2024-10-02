// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusremotewritereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusremotewritereceiver"

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gogo/protobuf/proto"
	promconfig "github.com/prometheus/prometheus/config"
	writev2 "github.com/prometheus/prometheus/prompb/io/prometheus/write/v2"
	promremote "github.com/prometheus/prometheus/storage/remote"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap/zapcore"
)

func newRemoteWriteReceiver(settings receiver.Settings, cfg *Config, nextConsumer consumer.Metrics) (receiver.Metrics, error) {
	return &prometheusRemoteWriteReceiver{
		settings:     settings,
		nextConsumer: nextConsumer,
		config:       cfg,
		server: &http.Server{
			ReadTimeout: 60 * time.Second,
		},
	}, nil
}

type prometheusRemoteWriteReceiver struct {
	settings     receiver.Settings
	nextConsumer consumer.Metrics

	config *Config
	server *http.Server
}

func (prw *prometheusRemoteWriteReceiver) Start(ctx context.Context, host component.Host) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/write", prw.handlePRW)
	var err error

	prw.server, err = prw.config.ToServer(ctx, host, prw.settings.TelemetrySettings, mux)
	if err != nil {
		return fmt.Errorf("failed to create server definition: %w", err)
	}
	listener, err := prw.config.ToListener(ctx)
	if err != nil {
		return fmt.Errorf("failed to create prometheus remote-write listener: %w", err)
	}

	go func() {
		if err := prw.server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			componentstatus.ReportStatus(host, componentstatus.NewFatalErrorEvent(fmt.Errorf("error starting prometheus remote-write receiver: %w", err)))
		}
	}()
	return nil
}

func (prw *prometheusRemoteWriteReceiver) Shutdown(ctx context.Context) error {
	if prw.server == nil {
		return nil
	}
	return prw.server.Shutdown(ctx)
}

func (prw *prometheusRemoteWriteReceiver) handlePRW(w http.ResponseWriter, req *http.Request) {
	contentType := req.Header.Get("Content-Type")
	if contentType == "" {
		prw.settings.Logger.Warn("message received without Content-Type header, rejecting")
		http.Error(w, "Content-Type header is required", http.StatusUnsupportedMediaType)
		return
	}

	msgType, err := prw.parseProto(contentType)
	if err != nil {
		prw.settings.Logger.Warn("Error decoding remote-write request", zapcore.Field{Key: "error", Type: zapcore.ErrorType, Interface: err})
		http.Error(w, err.Error(), http.StatusUnsupportedMediaType)
		return
	}
	if msgType != promconfig.RemoteWriteProtoMsgV2 {
		prw.settings.Logger.Warn("message received with unsupported proto version, rejecting")
		http.Error(w, "Unsupported proto version", http.StatusUnsupportedMediaType)
		return
	}

	// After parsing the content-type header, the next step would be to handle content-encoding.
	// Luckly confighttp's Server has middleware that already decompress the request body for us.

	body, err := io.ReadAll(req.Body)
	if err != nil {
		prw.settings.Logger.Warn("Error decoding remote write request", zapcore.Field{Key: "error", Type: zapcore.ErrorType, Interface: err})
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var prw2Req writev2.Request
	if err = proto.Unmarshal(body, &prw2Req); err != nil {
		prw.settings.Logger.Warn("Error decoding remote write request", zapcore.Field{Key: "error", Type: zapcore.ErrorType, Interface: err})
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	_, stats, err := prw.translateV2(req.Context(), &prw2Req)
	stats.SetHeaders(w)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest) // Following instructions at https://prometheus.io/docs/specs/remote_write_spec_2_0/#invalid-samples
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// parseProto parses the content-type header and returns the version of the remote-write protocol.
// We can't expect that senders of remote-write v1 will add the "proto=" parameter since it was not
// a requirement in v1. So, if the parameter is not found, we assume v1.
func (prw *prometheusRemoteWriteReceiver) parseProto(contentType string) (promconfig.RemoteWriteProtoMsg, error) {
	contentType = strings.TrimSpace(contentType)

	parts := strings.Split(contentType, ";")
	if parts[0] != "application/x-protobuf" {
		return "", fmt.Errorf("expected %q as the first (media) part, got %v content-type", "application/x-protobuf", contentType)
	}

	for _, part := range parts[1:] {
		parameter := strings.Split(part, "=")
		if len(parameter) != 2 {
			return "", fmt.Errorf("as per https://www.rfc-editor.org/rfc/rfc9110#parameter expected parameters to be key-values, got %v in %v content-type", part, contentType)
		}

		if strings.TrimSpace(parameter[0]) == "proto" {
			ret := promconfig.RemoteWriteProtoMsg(parameter[1])
			if err := ret.Validate(); err != nil {
				return "", fmt.Errorf("got %v content type; %w", contentType, err)
			}
			return ret, nil
		}
	}

	// No "proto=" parameter found, assume v1.
	return promconfig.RemoteWriteProtoMsgV1, nil
}

// translateV2 translates a v2 remote-write request into OTLP metrics.
// For now translateV2 is not implemented and returns an empty metrics.
// nolint
func (prw *prometheusRemoteWriteReceiver) translateV2(_ context.Context, _ *writev2.Request) (pmetric.Metrics, promremote.WriteResponseStats, error) {
	return pmetric.NewMetrics(), promremote.WriteResponseStats{}, nil
}
