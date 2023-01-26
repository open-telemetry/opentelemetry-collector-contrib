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

package datadogreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/datadogreceiver"

import (
	"context"
	"fmt"
	"net/http"
	"sync"

	datadogpb "github.com/DataDog/datadog-agent/pkg/trace/exportable/pb"
	"github.com/gorilla/mux"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/obsreport"
)

type datadogReceiver struct {
	config       *Config
	params       component.ReceiverCreateSettings
	nextConsumer consumer.Traces
	server       *http.Server
	shutdownWG   sync.WaitGroup
	obs          *obsreport.Receiver

	startOnce sync.Once
	stopOnce  sync.Once
}

func newDataDogReceiver(config *Config, nextConsumer consumer.Traces, params component.ReceiverCreateSettings) (component.TracesReceiver, error) {
	if nextConsumer == nil {
		return nil, component.ErrNilNextConsumer
	}

	return &datadogReceiver{
		params:       params,
		config:       config,
		nextConsumer: nextConsumer,
		server: &http.Server{
			ReadTimeout: config.ReadTimeout,
			Addr:        config.HTTPServerSettings.Endpoint,
		},
		obs: obsreport.NewReceiver(obsreport.ReceiverSettings{LongLivedCtx: false, ReceiverID: config.ReceiverSettings.ID(), Transport: "http", ReceiverCreateSettings: params}),
	}, nil
}

func (ddr *datadogReceiver) Start(_ context.Context, host component.Host) error {
	go ddr.startOnce.Do(func() {
		defer ddr.shutdownWG.Done()
		ddmux := mux.NewRouter()
		ddmux.HandleFunc("/v0.3/traces", ddr.handleTraces)
		ddmux.HandleFunc("/v0.4/traces", ddr.handleTraces)
		ddmux.HandleFunc("/v0.5/traces", ddr.handleTraces)
		ddr.server.Handler = ddmux
		if err := ddr.server.ListenAndServe(); err != http.ErrServerClosed {
			host.ReportFatalError(fmt.Errorf("error starting datadog receiver: %w", err))
		}
	})
	ddr.shutdownWG.Add(1)
	return nil
}

func (ddr *datadogReceiver) Shutdown(ctx context.Context) (err error) {
	ddr.stopOnce.Do(func() {
		err = ddr.server.Shutdown(ctx)
	})
	ddr.shutdownWG.Wait()
	return err
}

func (ddr *datadogReceiver) handleTraces(w http.ResponseWriter, req *http.Request) {
	obsCtx := ddr.obs.StartTracesOp(req.Context())
	var err error
	var spanCount int
	defer func(spanCount *int) {
		ddr.obs.EndTracesOp(obsCtx, "datadog", *spanCount, err)
	}(&spanCount)
	var ddTraces datadogpb.Traces

	err = decodeRequest(req, &ddTraces)
	if err != nil {
		http.Error(w, "Unable to unmarshal reqs", http.StatusInternalServerError)
		return
	}

	otelTraces := toTraces(ddTraces, req)
	spanCount = otelTraces.SpanCount()
	err = ddr.nextConsumer.ConsumeTraces(obsCtx, otelTraces)
	if err != nil {
		http.Error(w, "Trace consumer errored out", http.StatusInternalServerError)
	} else {
		_, _ = w.Write([]byte("OK"))
	}
}
