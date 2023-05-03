// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package influxdbreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/influxdbreceiver"

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/influxdata/influxdb-observability/common"
	"github.com/influxdata/influxdb-observability/influx2otel"
	"github.com/influxdata/line-protocol/v2/lineprotocol"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumererror"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/sanitize"
)

type metricsReceiver struct {
	nextConsumer       consumer.Metrics
	httpServerSettings *confighttp.HTTPServerSettings
	converter          *influx2otel.LineProtocolToOtelMetrics

	server *http.Server
	wg     sync.WaitGroup

	logger common.Logger

	settings component.TelemetrySettings
}

func newMetricsReceiver(config *Config, settings component.TelemetrySettings, nextConsumer consumer.Metrics) (*metricsReceiver, error) {
	influxLogger := newZapInfluxLogger(settings.Logger)
	converter, err := influx2otel.NewLineProtocolToOtelMetrics(influxLogger)
	if err != nil {
		return nil, err
	}
	receiver := &metricsReceiver{
		nextConsumer:       nextConsumer,
		httpServerSettings: &config.HTTPServerSettings,
		converter:          converter,
		logger:             influxLogger,
		settings:           settings,
	}
	return receiver, nil
}

func (r *metricsReceiver) Start(_ context.Context, host component.Host) error {
	ln, err := r.httpServerSettings.ToListener()
	if err != nil {
		return fmt.Errorf("failed to bind to address %s: %w", r.httpServerSettings.Endpoint, err)
	}

	router := http.NewServeMux()
	router.HandleFunc("/write", r.handleWrite)        // InfluxDB 1.x
	router.HandleFunc("/api/v2/write", r.handleWrite) // InfluxDB 2.x

	r.wg.Add(1)
	r.server, err = r.httpServerSettings.ToServer(host, r.settings, router)
	if err != nil {
		return err
	}
	go func() {
		defer r.wg.Done()
		if errHTTP := r.server.Serve(ln); !errors.Is(errHTTP, http.ErrServerClosed) && errHTTP != nil {
			host.ReportFatalError(errHTTP)
		}
	}()

	return nil
}

func (r *metricsReceiver) Shutdown(_ context.Context) error {
	if r.server == nil {
		return nil
	}
	if err := r.server.Close(); err != nil {
		return err
	}
	r.wg.Wait()
	return nil
}

const defaultPrecision = lineprotocol.Nanosecond

var precisions = map[string]lineprotocol.Precision{
	lineprotocol.Nanosecond.String():  lineprotocol.Nanosecond,
	lineprotocol.Microsecond.String(): lineprotocol.Microsecond,
	lineprotocol.Millisecond.String(): lineprotocol.Millisecond,
	lineprotocol.Second.String():      lineprotocol.Second,
}

func (r *metricsReceiver) handleWrite(w http.ResponseWriter, req *http.Request) {
	defer func() {
		_ = req.Body.Close()
	}()

	precision := defaultPrecision
	if precisionStr := req.URL.Query().Get("precision"); precisionStr != "" {
		var ok bool
		if precision, ok = precisions[precisionStr]; !ok {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = fmt.Fprintf(w, "unrecognized precision '%s'", sanitize.String(precisionStr))
			return
		}
	}

	batch := r.converter.NewBatch()
	lpDecoder := lineprotocol.NewDecoder(req.Body)

	var k, vTag []byte
	var vField lineprotocol.Value
	for line := 0; lpDecoder.Next(); line++ {
		measurement, err := lpDecoder.Measurement()
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = fmt.Fprintf(w, "failed to parse measurement on line %d", line)
			return
		}

		tags := make(map[string]string)
		for k, vTag, err = lpDecoder.NextTag(); k != nil && err == nil; k, vTag, err = lpDecoder.NextTag() {
			tags[string(k)] = string(vTag)
		}
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = fmt.Fprintf(w, "failed to parse tag on line %d", line)
			return
		}

		fields := make(map[string]interface{})
		for k, vField, err = lpDecoder.NextField(); k != nil && err == nil; k, vField, err = lpDecoder.NextField() {
			fields[string(k)] = vField.Interface()
		}
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = fmt.Fprintf(w, "failed to parse field on line %d", line)
			return
		}

		ts, err := lpDecoder.Time(precision, time.Time{})
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = fmt.Fprintf(w, "failed to parse timestamp on line %d", line)
			return
		}

		if err = lpDecoder.Err(); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = fmt.Fprintf(w, "failed to parse line: %s", err.Error())
			return
		}

		err = batch.AddPoint(string(measurement), tags, fields, ts, common.InfluxMetricValueTypeUntyped)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = fmt.Fprintf(w, "failed to append to the batch")
			return
		}
	}

	if err := r.nextConsumer.ConsumeMetrics(req.Context(), batch.GetMetrics()); err != nil {
		if consumererror.IsPermanent(err) {
			w.WriteHeader(http.StatusBadRequest)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
		r.logger.Debug("failed to pass metrics to next consumer: %s", err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
