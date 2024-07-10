// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusremotewritereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusremotewritereceiver"

import (
	"context"
	"errors"

	"github.com/prometheus/prometheus/storage/remote"
	"go.opentelemetry.io/collector/receiver/receiverhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheusremotewrite"

	"net"
	"net/http"
	"sync"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"
)

const (
	receiverFormat = "protobuf"
)

// NewReceiver - remote write
func NewReceiver(settings receiver.Settings, cfg *Config, nextConsumer consumer.Metrics) (receiver.Metrics, error) {
	obsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{
		ReceiverID:             settings.ID,
		Transport:              "http",
		ReceiverCreateSettings: settings,
	})

	prwrecv := &PrometheusRemoteWriteReceiver{
		settings:     settings,
		nextConsumer: nextConsumer,
		config:       cfg,
		logger:       settings.Logger,
		obsrecv:      obsrecv,
	}
	return prwrecv, err
}

type PrometheusRemoteWriteReceiver struct {
	settings     receiver.Settings
	host         component.Host
	nextConsumer consumer.Metrics

	shutdownWG sync.WaitGroup

	server  *http.Server
	config  *Config
	logger  *zap.Logger
	obsrecv *receiverhelper.ObsReport
}

// Start - remote write
func (prwc *PrometheusRemoteWriteReceiver) Start(ctx context.Context, host component.Host) error {
	if host == nil {
		return errors.New("nil host")
	}
	var err error
	prwc.host = host
	prwc.server, err = prwc.config.ServerConfig.ToServer(ctx, host, prwc.settings.TelemetrySettings, prwc)

	var listener net.Listener
	listener, err = prwc.config.ServerConfig.ToListener(ctx)
	if err != nil {
		return err
	}
	prwc.shutdownWG.Add(1)
	go func() {
		defer prwc.shutdownWG.Done()

		if errHTTP := prwc.server.Serve(listener); !errors.Is(errHTTP, http.ErrServerClosed) && errHTTP != nil {
			prwc.settings.TelemetrySettings.ReportStatus(component.NewFatalErrorEvent(errHTTP))
		}
	}()

	return err
}

// Shutdown - remote write
func (prwc *PrometheusRemoteWriteReceiver) Shutdown(context.Context) error {
	var err error
	if prwc.server != nil {
		err = prwc.server.Close()
	}
	prwc.shutdownWG.Wait()
	return err
}

func (prwc *PrometheusRemoteWriteReceiver) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := prwc.obsrecv.StartMetricsOp(r.Context())
	req, err := remote.DecodeWriteRequest(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	pms, err := prometheusremotewrite.FromTimeSeries(req.Timeseries, prometheusremotewrite.PRWToMetricSettings{
		TimeThreshold: prwc.config.TimeThreshold,
		Logger:        *prwc.logger,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	metricCount := pms.ResourceMetrics().Len()
	dataPointCount := pms.DataPointCount()
	if metricCount != 0 {
		err = prwc.nextConsumer.ConsumeMetrics(ctx, pms)
	}
	prwc.obsrecv.EndMetricsOp(ctx, receiverFormat, dataPointCount, err)
	w.WriteHeader(http.StatusAccepted)
}
