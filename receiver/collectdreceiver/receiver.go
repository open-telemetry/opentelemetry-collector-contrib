// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package collectdreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/collectdreceiver"

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/collectdreceiver/internal/metadata"
)

var _ receiver.Metrics = (*collectdReceiver)(nil)

// collectdReceiver implements the receiver.Metrics for CollectD protocol.
type collectdReceiver struct {
	logger             *zap.Logger
	server             *http.Server
	shutdownWG         sync.WaitGroup
	defaultAttrsPrefix string
	nextConsumer       consumer.Metrics
	obsrecv            *receiverhelper.ObsReport
	createSettings     receiver.Settings
	config             *Config
}

// newCollectdReceiver creates the CollectD receiver with the given parameters.
func newCollectdReceiver(
	logger *zap.Logger,
	cfg *Config,
	defaultAttrsPrefix string,
	nextConsumer consumer.Metrics,
	createSettings receiver.Settings,
) (receiver.Metrics, error) {
	r := &collectdReceiver{
		logger:             logger,
		nextConsumer:       nextConsumer,
		defaultAttrsPrefix: defaultAttrsPrefix,
		config:             cfg,
		createSettings:     createSettings,
	}
	return r, nil
}

// Start starts an HTTP server that can process CollectD JSON requests.
func (cdr *collectdReceiver) Start(ctx context.Context, host component.Host) error {
	var err error
	cdr.server, err = cdr.config.ToServer(ctx, host, cdr.createSettings.TelemetrySettings, cdr)
	if err != nil {
		return err
	}
	cdr.server.ReadTimeout = cdr.config.Timeout
	cdr.server.WriteTimeout = cdr.config.Timeout
	cdr.obsrecv, err = receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{
		ReceiverID:             cdr.createSettings.ID,
		Transport:              "http",
		ReceiverCreateSettings: cdr.createSettings,
	})
	if err != nil {
		return err
	}
	l, err := cdr.config.ToListener(ctx)
	if err != nil {
		return err
	}
	cdr.shutdownWG.Add(1)
	go func() {
		defer cdr.shutdownWG.Done()
		if err := cdr.server.Serve(l); !errors.Is(err, http.ErrServerClosed) && err != nil {
			componentstatus.ReportStatus(host, componentstatus.NewFatalErrorEvent(err))
		}
	}()
	return nil
}

// Shutdown stops the CollectD receiver.
func (cdr *collectdReceiver) Shutdown(context.Context) error {
	if cdr.server == nil {
		return nil
	}
	err := cdr.server.Shutdown(context.Background())
	cdr.shutdownWG.Wait()
	return err
}

// ServeHTTP acts as the default and only HTTP handler for the CollectD receiver.
func (cdr *collectdReceiver) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	ctx = cdr.obsrecv.StartMetricsOp(ctx)

	if r.Method != http.MethodPost {
		cdr.obsrecv.EndMetricsOp(ctx, metadata.Type.String(), 0, errors.New("invalid http verb"))
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		cdr.obsrecv.EndMetricsOp(ctx, metadata.Type.String(), 0, err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	var records []collectDRecord
	err = json.Unmarshal(body, &records)
	if err != nil {
		cdr.obsrecv.EndMetricsOp(ctx, metadata.Type.String(), 0, err)
		cdr.handleHTTPErr(w, err, "unable to decode json")
		return
	}

	defaultAttrs := cdr.defaultAttributes(r)

	metrics := pmetric.NewMetrics()
	scopeMetrics := metrics.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty()
	for _, record := range records {
		err = record.appendToMetrics(cdr.logger, scopeMetrics, defaultAttrs)
		if err != nil {
			cdr.obsrecv.EndMetricsOp(ctx, metadata.Type.String(), len(records), err)
			cdr.handleHTTPErr(w, err, "unable to process metrics")
			return
		}
	}
	lenDp := metrics.DataPointCount()

	err = cdr.nextConsumer.ConsumeMetrics(ctx, metrics)
	if err != nil {
		cdr.obsrecv.EndMetricsOp(ctx, metadata.Type.String(), lenDp, err)
		return
	}

	_, err = w.Write([]byte("OK"))
	if err != nil {
		cdr.obsrecv.EndMetricsOp(ctx, metadata.Type.String(), lenDp, err)
		return
	}
	cdr.obsrecv.EndMetricsOp(ctx, metadata.Type.String(), lenDp, nil)
}

func (cdr *collectdReceiver) defaultAttributes(req *http.Request) map[string]string {
	if cdr.defaultAttrsPrefix == "" {
		return nil
	}
	params := req.URL.Query()
	attrs := make(map[string]string)
	for key := range params {
		if strings.HasPrefix(key, cdr.defaultAttrsPrefix) {
			value := params.Get(key)
			if len(value) == 0 {
				cdr.logger.Debug("blank attribute value", zap.String("key", key))
				continue
			}
			key = key[len(cdr.defaultAttrsPrefix):]
			attrs[key] = value
		}
	}
	return attrs
}

func (cdr *collectdReceiver) handleHTTPErr(w http.ResponseWriter, err error, msg string) {
	w.WriteHeader(http.StatusBadRequest)
	cdr.logger.Error(msg, zap.Error(err))
	_, err = w.Write([]byte(msg))
	if err != nil {
		cdr.logger.Error("error writing to response writer", zap.Error(err))
	}
}
