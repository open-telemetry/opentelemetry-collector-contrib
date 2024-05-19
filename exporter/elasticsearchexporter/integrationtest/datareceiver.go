// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package integrationtest // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/integrationtest"

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/elastic/go-docappender/v2/docappendertest"
	"github.com/gorilla/mux"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
)

type esDataReceiver struct {
	testbed.DataReceiverBase
	receiver receiver.Logs
	endpoint string
}

func newElasticsearchDataReceiver(t testing.TB) *esDataReceiver {
	return &esDataReceiver{
		DataReceiverBase: testbed.DataReceiverBase{},
		endpoint:         fmt.Sprintf("http://%s:%d", testbed.DefaultHost, testutil.GetAvailablePort(t)),
	}
}

func (es *esDataReceiver) Start(_ consumer.Traces, _ consumer.Metrics, lc consumer.Logs) error {
	factory := receiver.NewFactory(
		component.MustNewType("mockelasticsearch"),
		createDefaultConfig,
		receiver.WithLogs(createLogsReceiver, component.StabilityLevelDevelopment),
	)
	cfg := factory.CreateDefaultConfig().(*config)
	cfg.ESEndpoint = es.endpoint

	var err error
	set := receivertest.NewNopCreateSettings()
	// Use an actual logger to log errors.
	set.Logger = zap.Must(zap.NewDevelopment())
	es.receiver, err = factory.CreateLogsReceiver(context.Background(), set, cfg, lc)
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
	cfgFormat := `
  elasticsearch:
    endpoints: [%s]
    flush:
      interval: 1s
    sending_queue:
      enabled: true
    retry:
      enabled: true
      max_requests: 10000
`
	return fmt.Sprintf(cfgFormat, es.endpoint)
}

func (es *esDataReceiver) ProtocolName() string {
	return "elasticsearch"
}

type config struct {
	ESEndpoint string
}

func createDefaultConfig() component.Config {
	return &config{
		ESEndpoint: "127.0.0.1:9200",
	}
}

func createLogsReceiver(
	_ context.Context,
	params receiver.CreateSettings,
	rawCfg component.Config,
	next consumer.Logs,
) (receiver.Logs, error) {
	cfg := rawCfg.(*config)
	return newMockESReceiver(params, cfg, next)
}

type mockESReceiver struct {
	server *http.Server
	params receiver.CreateSettings
}

func newMockESReceiver(params receiver.CreateSettings, cfg *config, next consumer.Logs) (receiver.Logs, error) {
	emptyLogs := plog.NewLogs()
	emptyLogs.ResourceLogs().AppendEmpty().
		ScopeLogs().AppendEmpty().
		LogRecords().AppendEmpty()

	r := mux.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Elastic-Product", "Elasticsearch")
			next.ServeHTTP(w, r)
		})
	})
	r.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintln(w, `{"version":{"number":"1.2.3"}}`)
	})
	r.HandleFunc("/_bulk", func(w http.ResponseWriter, r *http.Request) {
		_, response := docappendertest.DecodeBulkRequest(r)
		for _, itemMap := range response.Items {
			for k, item := range itemMap {
				// Ideally bulk request should be converted to log record
				// however, since we only assert count for now there is no
				// need to do the actual translation. We use a pre-initialized
				// empty plog.Logs to reduce allocation impact on tests and
				// benchmarks due to this.
				if err := next.ConsumeLogs(context.Background(), emptyLogs); err != nil {
					response.HasErrors = true
					item.Status = http.StatusTooManyRequests
					item.Error.Type = "simulated_es_error"
					item.Error.Reason = err.Error()
				}
				itemMap[k] = item
			}
		}
		if err := json.NewEncoder(w).Encode(response); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
		}
	})

	esURL, err := url.Parse(cfg.ESEndpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Elasticsearch endpoint: %w", err)
	}
	return &mockESReceiver{
		server: &http.Server{
			Addr:              esURL.Host,
			Handler:           r,
			ReadHeaderTimeout: 20 * time.Second,
		},
		params: params,
	}, nil
}

func (es *mockESReceiver) Start(_ context.Context, _ component.Host) error {
	go func() {
		if err := es.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			es.params.Logger.Error("failed while running mock ES receiver", zap.Error(err))
		}
	}()
	return nil
}

func (es *mockESReceiver) Shutdown(ctx context.Context) error {
	return es.server.Shutdown(ctx)
}
