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

package collectdreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/collectdreceiver"

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"

	internaldata "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/opencensus"
)

var _ receiver.Metrics = (*collectdReceiver)(nil)

// collectdReceiver implements the receiver.Metrics for CollectD protocol.
type collectdReceiver struct {
	logger             *zap.Logger
	addr               string
	server             *http.Server
	defaultAttrsPrefix string
	nextConsumer       consumer.Metrics
}

// newCollectdReceiver creates the CollectD receiver with the given parameters.
func newCollectdReceiver(
	logger *zap.Logger,
	addr string,
	timeout time.Duration,
	defaultAttrsPrefix string,
	nextConsumer consumer.Metrics) (receiver.Metrics, error) {
	if nextConsumer == nil {
		return nil, component.ErrNilNextConsumer
	}

	r := &collectdReceiver{
		logger:             logger,
		addr:               addr,
		nextConsumer:       nextConsumer,
		defaultAttrsPrefix: defaultAttrsPrefix,
	}
	r.server = &http.Server{
		Addr:         addr,
		Handler:      r,
		ReadTimeout:  timeout,
		WriteTimeout: timeout,
	}
	return r, nil
}

// Start starts an HTTP server that can process CollectD JSON requests.
func (cdr *collectdReceiver) Start(_ context.Context, host component.Host) error {
	go func() {
		if err := cdr.server.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) && err != nil {
			host.ReportFatalError(fmt.Errorf("error starting collectd receiver: %w", err))
		}
	}()
	return nil
}

// Shutdown stops the CollectD receiver.
func (cdr *collectdReceiver) Shutdown(context.Context) error {
	return cdr.server.Shutdown(context.Background())
}

// ServeHTTP acts as the default and only HTTP handler for the CollectD receiver.
func (cdr *collectdReceiver) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	recordRequestReceived()

	if r.Method != "POST" {
		recordRequestErrors()
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		recordRequestErrors()
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	var records []collectDRecord
	err = json.Unmarshal(body, &records)
	if err != nil {
		cdr.handleHTTPErr(w, err, "unable to decode json")
		return
	}

	defaultAttrs := cdr.defaultAttributes(r)

	var metrics []*metricspb.Metric
	ctx := context.Background()
	for _, record := range records {
		metrics, err = record.appendToMetrics(metrics, defaultAttrs)
		if err != nil {
			cdr.handleHTTPErr(w, err, "unable to process metrics")
			return
		}
	}

	err = cdr.nextConsumer.ConsumeMetrics(ctx, internaldata.OCToMetrics(nil, nil, metrics))
	if err != nil {
		cdr.handleHTTPErr(w, err, "unable to process metrics")
		return
	}

	_, err = w.Write([]byte("OK"))
	if err != nil {
		cdr.handleHTTPErr(w, err, "unable to write response")
		return
	}

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
				recordDefaultBlankAttrs()
				continue
			}
			key = key[len(cdr.defaultAttrsPrefix):]
			attrs[key] = value
		}
	}
	return attrs
}

func (cdr *collectdReceiver) handleHTTPErr(w http.ResponseWriter, err error, msg string) {
	recordRequestErrors()
	w.WriteHeader(http.StatusBadRequest)
	cdr.logger.Error(msg, zap.Error(err))
	_, err = w.Write([]byte(msg))
	if err != nil {
		cdr.logger.Error("error writing to response writer", zap.Error(err))
	}
}
