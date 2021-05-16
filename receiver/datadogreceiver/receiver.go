// Copyright 2021, OpenTelemetry Authors
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

package datadogreceiver

import (
	"context"
	"fmt"
	"go.opentelemetry.io/collector/config"
	"net/http"
	"sync"

	"github.com/DataDog/datadog-agent/pkg/trace/exportable/pb"
	"github.com/gorilla/mux"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenterror"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/obsreport"
)

type datadogReceiver struct {
	config       *Config
	id           config.ComponentID
	params       component.ReceiverCreateParams
	nextConsumer consumer.Traces
	server       *http.Server
	longLivedCtx context.Context

	mu        sync.Mutex
	startOnce sync.Once
	stopOnce  sync.Once
}

func newDataDogReceiver(config *Config, nextConsumer consumer.Traces, params component.ReceiverCreateParams) (component.TracesReceiver, error) {
	if nextConsumer == nil {
		return nil, componenterror.ErrNilNextConsumer
	}
	return &datadogReceiver{
		params:       params,
		config:       config,
		nextConsumer: nextConsumer,
		server: &http.Server{
			ReadTimeout: config.ReadTimeout,
			Addr:        config.HTTPServerSettings.Endpoint,
		},
	}, nil
}

const collectorHTTPTransport = "http_collector"

func (ddr *datadogReceiver) Start(ctx context.Context, host component.Host) error {
	ddr.longLivedCtx = obsreport.ReceiverContext(ctx, ddr.config.ID(), collectorHTTPTransport)
	ddmux := mux.NewRouter()
	ddmux.HandleFunc("/v0.3/traces", ddr.handleTraces)
	ddmux.HandleFunc("/v0.4/traces", ddr.handleTraces)
	ddmux.HandleFunc("/v0.5/traces", ddr.handleTraces)
	ddr.server.Handler = ddmux
	go ddr.startOnce.Do(func() {
		if err := ddr.server.ListenAndServe(); err != http.ErrServerClosed {
			host.ReportFatalError(fmt.Errorf("error starting datadog receiver: %v", err))
		}
	})
	return nil
}

func (ddr *datadogReceiver) Shutdown(ctx context.Context) error {
	var err error
	ddr.stopOnce.Do(func() {
		err = ddr.server.Shutdown(ctx)
	})
	return err
}

func (ddr *datadogReceiver) handleTraces(w http.ResponseWriter, req *http.Request) {
	obsreport.StartTraceDataReceiveOp(ddr.longLivedCtx, ddr.config.ID(), "http")
	var ddTraces pb.Traces

	err := decodeRequest(req, &ddTraces)
	if err != nil {
		http.Error(w, "Unable to unmarshal reqs", http.StatusInternalServerError)
		obsreport.EndTraceDataReceiveOp(ddr.longLivedCtx, typeStr, 0, err)
		return
	}

	otelTraces := ToTraces(ddTraces, req)
	spanCount := otelTraces.SpanCount()
	err = ddr.nextConsumer.ConsumeTraces(req.Context(), otelTraces)
	if err != nil {
		http.Error(w, "Trace consumer errored out", http.StatusInternalServerError)
	} else {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	}
	obsreport.EndTraceDataReceiveOp(ddr.longLivedCtx, typeStr, spanCount, err)
}
