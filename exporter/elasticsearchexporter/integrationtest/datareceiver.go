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

	"github.com/elastic/go-docappender/v2/docappendertest"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/sharedcomponent"
	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
)

const (
	// TestLogsIndex is used by the mock ES data receiver to indentify log events.
	// Exporter LogsIndex configuration must be configured with TestLogsIndex for
	// the data receiver to work properly
	TestLogsIndex = "logs-test-idx"

	// TestTracesIndex is used by the mock ES data receiver to indentify trace
	// events. Exporter TracesIndex configuration must be configured with
	// TestTracesIndex for the data receiver to work properly
	TestTracesIndex = "traces-test-idx"
)

type esDataReceiver struct {
	testbed.DataReceiverBase
	receiver          receiver.Logs
	endpoint          string
	decodeBulkRequest bool
	batcherEnabled    *bool
	t                 testing.TB
}

type dataReceiverOption func(*esDataReceiver)

func newElasticsearchDataReceiver(t testing.TB, opts ...dataReceiverOption) *esDataReceiver {
	r := &esDataReceiver{
		DataReceiverBase:  testbed.DataReceiverBase{},
		endpoint:          fmt.Sprintf("http://%s:%d", testbed.DefaultHost, testutil.GetAvailablePort(t)),
		decodeBulkRequest: true,
		t:                 t,
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

func withDecodeBulkRequest(decode bool) dataReceiverOption {
	return func(r *esDataReceiver) {
		r.decodeBulkRequest = decode
	}
}

func withBatcherEnabled(enabled bool) dataReceiverOption {
	return func(r *esDataReceiver) {
		r.batcherEnabled = &enabled
	}
}

func (es *esDataReceiver) Start(tc consumer.Traces, _ consumer.Metrics, lc consumer.Logs) error {
	factory := receiver.NewFactory(
		component.MustNewType("mockelasticsearch"),
		createDefaultConfig,
		receiver.WithLogs(createLogsReceiver, component.StabilityLevelDevelopment),
		receiver.WithTraces(createTracesReceiver, component.StabilityLevelDevelopment),
	)
	esURL, err := url.Parse(es.endpoint)
	if err != nil {
		return fmt.Errorf("invalid ES URL specified %s: %w", es.endpoint, err)
	}
	cfg := factory.CreateDefaultConfig().(*config)
	cfg.ServerConfig.Endpoint = esURL.Host
	cfg.DecodeBulkRequests = es.decodeBulkRequest

	set := receivertest.NewNopSettings()
	// Use an actual logger to log errors.
	set.Logger = zap.Must(zap.NewDevelopment())
	logsReceiver, err := factory.CreateLogsReceiver(context.Background(), set, cfg, lc)
	if err != nil {
		return fmt.Errorf("failed to create logs receiver: %w", err)
	}
	tracesReceiver, err := factory.CreateTracesReceiver(context.Background(), set, cfg, tc)
	if err != nil {
		return fmt.Errorf("failed to create traces receiver: %w", err)
	}

	// Since we use SharedComponent both receivers should be same
	require.Same(es.t, logsReceiver, tracesReceiver)
	es.receiver = logsReceiver

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
	cfgFormat := fmt.Sprintf(`
  elasticsearch:
    endpoints: [%s]
    logs_index: %s
    traces_index: %s
    sending_queue:
      enabled: true
    retry:
      enabled: true
      initial_interval: 100ms
      max_interval: 1s
      max_requests: 10000`,
		es.endpoint, TestLogsIndex, TestTracesIndex,
	)

	if es.batcherEnabled == nil {
		cfgFormat += `
    flush:
      interval: 1s`
	} else {
		cfgFormat += fmt.Sprintf(`
    batcher:
      flush_timeout: 1s
      enabled: %v`,
			*es.batcherEnabled,
		)
	}
	return cfgFormat + "\n"
}

func (es *esDataReceiver) ProtocolName() string {
	return "elasticsearch"
}

type config struct {
	confighttp.ServerConfig

	// DecodeBulkRequests controls decoding of the bulk request in the mock
	// ES receiver. Decoding requests would consume resources and might
	// pollute the benchmark results. Note that if decode bulk request is
	// set to false then the consumers will not consume any events and the
	// bulk request will always return http.StatusOK.
	DecodeBulkRequests bool
}

func createDefaultConfig() component.Config {
	return &config{
		ServerConfig: confighttp.ServerConfig{
			Endpoint: "127.0.0.1:9200",
		},
		DecodeBulkRequests: true,
	}
}

func createLogsReceiver(
	_ context.Context,
	params receiver.Settings,
	rawCfg component.Config,
	next consumer.Logs,
) (receiver.Logs, error) {
	receiver := receivers.GetOrAdd(rawCfg, func() component.Component {
		return newMockESReceiver(params, rawCfg.(*config))
	})
	receiver.Unwrap().(*mockESReceiver).logsConsumer = next
	return receiver, nil
}

func createTracesReceiver(
	_ context.Context,
	params receiver.Settings,
	rawCfg component.Config,
	next consumer.Traces,
) (receiver.Traces, error) {
	receiver := receivers.GetOrAdd(rawCfg, func() component.Component {
		return newMockESReceiver(params, rawCfg.(*config))
	})
	receiver.Unwrap().(*mockESReceiver).tracesConsumer = next
	return receiver, nil
}

type mockESReceiver struct {
	params receiver.Settings
	config *config

	tracesConsumer consumer.Traces
	logsConsumer   consumer.Logs

	server *http.Server
}

func newMockESReceiver(params receiver.Settings, cfg *config) receiver.Logs {
	return &mockESReceiver{
		params: params,
		config: cfg,
	}
}

func (es *mockESReceiver) Start(ctx context.Context, host component.Host) error {
	if es.server != nil {
		return nil
	}

	ln, err := es.config.ToListener(ctx)
	if err != nil {
		return fmt.Errorf("failed to bind to address %s: %w", es.config.Endpoint, err)
	}

	// Ideally bulk request items should be converted to the corresponding event record
	// however, since we only assert count for now there is no need to do the actual
	// translation. Instead we use a pre-initialized empty logs and traces model to
	// reduce allocation impact on tests and benchmarks.
	emptyLogs := plog.NewLogs()
	emptyLogs.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
	emptyTrace := ptrace.NewTraces()
	emptyTrace.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()

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
		if !es.config.DecodeBulkRequests {
			fmt.Fprintln(w, "{}")
			return
		}
		_, response := docappendertest.DecodeBulkRequest(r)
		for _, itemMap := range response.Items {
			for k, item := range itemMap {
				var consumeErr error
				switch item.Index {
				case TestLogsIndex:
					consumeErr = es.logsConsumer.ConsumeLogs(context.Background(), emptyLogs)
				case TestTracesIndex:
					consumeErr = es.tracesConsumer.ConsumeTraces(context.Background(), emptyTrace)
				}
				if consumeErr != nil {
					response.HasErrors = true
					item.Status = http.StatusTooManyRequests
					item.Error.Type = "simulated_es_error"
					item.Error.Reason = consumeErr.Error()
				}
				itemMap[k] = item
			}
		}
		if jsonErr := json.NewEncoder(w).Encode(response); jsonErr != nil {
			w.WriteHeader(http.StatusInternalServerError)
		}
	})

	es.server, err = es.config.ToServer(ctx, host, es.params.TelemetrySettings, r)
	if err != nil {
		return fmt.Errorf("failed to create mock ES server: %w", err)
	}

	go func() {
		if err := es.server.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			es.params.ReportStatus(component.NewFatalErrorEvent(err))
		}
	}()
	return nil
}

func (es *mockESReceiver) Shutdown(ctx context.Context) error {
	if es.server == nil {
		return nil
	}
	return es.server.Shutdown(ctx)
}

// mockESReceiver serves both, traces and logs. Shared component allows for a single
// instance of mockESReceiver to serve all supported event types.
var receivers = sharedcomponent.NewSharedComponents()
