// Copyright  The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package prometheusremotewritereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusremotewritereceiver"

import (
	"context"
	"errors"
	"net"
	"net/http"
	"sync"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheusremotewrite"
	"github.com/prometheus/prometheus/storage/remote"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/obsreport"
	"go.uber.org/zap"
)

const (
	receiverFormat = "protobuf"
)

//var reg = regexp.MustCompile(`(\w+)_(\w+)_(\w+)\z`)

// PrometheusRemoteWriteReceiver - remote write
type PrometheusRemoteWriteReceiver struct {
	params       component.ReceiverCreateSettings
	host         component.Host
	nextConsumer consumer.Metrics

	mu         sync.Mutex
	startOnce  sync.Once
	stopOnce   sync.Once
	shutdownWG sync.WaitGroup

	server        *http.Server
	config        *Config
	timeThreshold *int64
	logger        *zap.Logger
	obsrecv       *obsreport.Receiver
}

// NewReceiver - remote write
func NewReceiver(params component.ReceiverCreateSettings, config *Config, consumer consumer.Metrics) (*PrometheusRemoteWriteReceiver, error) {
	obsrecv, err := obsreport.NewReceiver(obsreport.ReceiverSettings{
		ReceiverID:             config.ID(),
		ReceiverCreateSettings: params,
	})
	zr := &PrometheusRemoteWriteReceiver{
		params:        params,
		nextConsumer:  consumer,
		config:        config,
		logger:        params.Logger,
		obsrecv:       obsrecv,
		timeThreshold: &config.TimeThreshold,
	}
	return zr, err
}

// Start - remote write
func (rec *PrometheusRemoteWriteReceiver) Start(_ context.Context, host component.Host) error {
	if host == nil {
		return errors.New("nil host")
	}
	rec.mu.Lock()
	defer rec.mu.Unlock()
	var err = component.ErrNilNextConsumer
	rec.startOnce.Do(func() {
		err = nil
		rec.host = host
		rec.server, err = rec.config.HTTPServerSettings.ToServer(host, rec.params.TelemetrySettings, rec)
		var listener net.Listener
		listener, err = rec.config.HTTPServerSettings.ToListener()
		if err != nil {
			return
		}
		rec.shutdownWG.Add(1)
		go func() {
			defer rec.shutdownWG.Done()
			if errHTTP := rec.server.Serve(listener); errHTTP != http.ErrServerClosed {
				host.ReportFatalError(errHTTP)
			}
		}()
	})
	return err
}

func (rec *PrometheusRemoteWriteReceiver) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	ctx := rec.obsrecv.StartMetricsOp(r.Context())
	req, err := remote.DecodeWriteRequest(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	pms, err := prometheusremotewrite.FromTimeSeries(req.Timeseries, prometheusremotewrite.Settings{
		TimeThreshold: *rec.timeThreshold,
		Logger:        *rec.logger,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	metricCount := pms.ResourceMetrics().Len()
	dataPointCount := pms.DataPointCount()
	if metricCount != 0 {
		err = rec.nextConsumer.ConsumeMetrics(ctx, pms)
	}
	rec.obsrecv.EndMetricsOp(ctx, receiverFormat, dataPointCount, err)
	w.WriteHeader(http.StatusAccepted)
}

// Shutdown - remote write
func (rec *PrometheusRemoteWriteReceiver) Shutdown(context.Context) error {
	var err = component.ErrNilNextConsumer
	rec.stopOnce.Do(func() {
		err = rec.server.Close()
		rec.shutdownWG.Wait()
	})
	return err
}
