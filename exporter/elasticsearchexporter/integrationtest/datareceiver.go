// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package integrationtest

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/elastic/go-docappender/docappendertest"
	"github.com/gorilla/mux"
	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/receiver"
)

type esDataReceiver struct {
	testbed.DataReceiverBase
	receiver receiver.Logs
}

func NewElasticsearchDataReceiver() testbed.DataReceiver {
	return &esDataReceiver{
		DataReceiverBase: testbed.DataReceiverBase{},
	}
}

func (es *esDataReceiver) Start(_ consumer.Traces, _ consumer.Metrics, lc consumer.Logs) error {
	factory := receiver.NewFactory(
		component.MustNewType("mockelasticsearch"),
		nil,
		receiver.WithLogs(createLogsReceiver, component.StabilityLevelDevelopment),
	)
	var err error
	es.receiver, err = factory.CreateLogsReceiver(context.Background(), receiver.CreateSettings{}, nil, lc)
	if err != nil {
		return err
	}
	return es.receiver.Start(context.Background(), componenttest.NewNopHost())
}

func (es *esDataReceiver) Stop() error {
	if es.receiver != nil {
		return es.receiver.Shutdown(context.Background())
	}
	return nil
}

func (es *esDataReceiver) GenConfigYAMLStr() string {
	// Note that this generates an exporter config for agent.
	return `
  elasticsearch:
    endpoints: [http://127.0.0.1:9200]
    flush:
      interval: 1s
    sending_queue:
      enabled: true
    retry:
      enabled: true
      max_requests: 10000
`
}

func (es *esDataReceiver) ProtocolName() string {
	return "elasticsearch"
}

func createLogsReceiver(
	_ context.Context,
	_ receiver.CreateSettings,
	_ component.Config,
	next consumer.Logs,
) (receiver.Logs, error) {
	return newMockESReceiver(next)
}

type mockESReceiver struct {
	server *http.Server
}

func newMockESReceiver(next consumer.Logs) (receiver.Logs, error) {
	r := mux.NewRouter()
	r.Use(mux.MiddlewareFunc(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Elastic-Product", "Elasticsearch")
			next.ServeHTTP(w, r)
		})
	}))
	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, `{"version":{"number":"1.2.3"}}`)
	})
	r.HandleFunc("/_bulk", func(w http.ResponseWriter, r *http.Request) {
		_, response := docappendertest.DecodeBulkRequest(r)
		for _, itemMap := range response.Items {
			for k, item := range itemMap {
				// Ideally bulk request should be converted to log record
				// however, since we only assert count for now there is no
				// need to do the actual translation.
				logs := plog.NewLogs()
				logs.ResourceLogs().AppendEmpty().
					ScopeLogs().AppendEmpty().
					LogRecords().AppendEmpty()

				if err := next.ConsumeLogs(context.Background(), logs); err != nil {
					response.HasErrors = true
					item.Status = http.StatusInternalServerError
					item.Error.Type = "simulated_es_error"
					item.Error.Reason = err.Error()
				}
				itemMap[k] = item
			}
		}
		json.NewEncoder(w).Encode(response)
	})

	return &mockESReceiver{
		server: &http.Server{
			Addr:    "127.0.0.1:9200",
			Handler: r,
		},
	}, nil
}

func (es *mockESReceiver) Start(_ context.Context, host component.Host) error {
	go func() {
		es.server.ListenAndServe()
	}()
	return nil
}

func (es *mockESReceiver) Shutdown(ctx context.Context) error {
	return es.server.Shutdown(ctx)
}
