// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusreceiver

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/goccy/go-yaml"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	"github.com/prometheus/common/promslog"
	"github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/discovery"
	"github.com/prometheus/prometheus/model/histogram"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/scrape"
	"github.com/prometheus/prometheus/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// UPSTREAM PROMETHEUS BUG TEST: Verify opts.Metadata provided by AppenderV2
// =============================================================================
//
// This test uses Prometheus's actual scraping code to verify what metadata
// is provided in AppendV2Options during scraping. This documents the upstream
// bug where opts.Metadata.Type and opts.MetricFamilyName are empty.
//
// Related: https://github.com/prometheus/prometheus/pull/17872
// =============================================================================

// capturedAppendCall records the parameters from a single Append call
type capturedAppendCall struct {
	labels           labels.Labels
	metricFamilyName string
	metadataType     model.MetricType
	metadataHelp     string
	metadataUnit     string
}

// capturingAppendableV2 is an AppendableV2 that captures all Append calls for inspection
type capturingAppendableV2 struct {
	mu       sync.Mutex
	captured []capturedAppendCall
	done     chan struct{}
	minCalls int
}

func newCapturingAppendableV2(minCalls int) *capturingAppendableV2 {
	return &capturingAppendableV2{
		captured: make([]capturedAppendCall, 0),
		done:     make(chan struct{}),
		minCalls: minCalls,
	}
}

func (c *capturingAppendableV2) AppenderV2(context.Context) storage.AppenderV2 {
	return &capturingAppenderV2{parent: c}
}

func (c *capturingAppendableV2) getCaptured() []capturedAppendCall {
	c.mu.Lock()
	defer c.mu.Unlock()
	result := make([]capturedAppendCall, len(c.captured))
	copy(result, c.captured)
	return result
}

// capturingAppenderV2 captures Append calls
type capturingAppenderV2 struct {
	parent *capturingAppendableV2
}

func (a *capturingAppenderV2) Append(
	_ storage.SeriesRef,
	ls labels.Labels,
	_, _ int64,
	_ float64,
	_ *histogram.Histogram,
	_ *histogram.FloatHistogram,
	opts storage.AppendV2Options,
) (storage.SeriesRef, error) {
	a.parent.mu.Lock()
	a.parent.captured = append(a.parent.captured, capturedAppendCall{
		labels:           ls.Copy(),
		metricFamilyName: opts.MetricFamilyName,
		metadataType:     opts.Metadata.Type,
		metadataHelp:     opts.Metadata.Help,
		metadataUnit:     opts.Metadata.Unit,
	})
	count := len(a.parent.captured)
	a.parent.mu.Unlock()

	// Signal when we have enough calls
	if count >= a.parent.minCalls {
		select {
		case <-a.parent.done:
			// Already closed
		default:
			close(a.parent.done)
		}
	}
	return 0, nil
}

func (*capturingAppenderV2) Commit() error {
	return nil
}

func (*capturingAppenderV2) Rollback() error {
	return nil
}

// TestAppenderV2_MetadataFromPrometheus verifies what metadata Prometheus actually
// provides when calling AppenderV2.Append() during scraping.
func TestAppenderV2_MetadataFromPrometheus(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Set up a mock server that returns Prometheus metrics
	metricsPage := `
# HELP test_counter_total A test counter metric
# TYPE test_counter_total counter
test_counter_total{label="value"} 100

# HELP test_gauge A test gauge metric
# TYPE test_gauge gauge
test_gauge 42.5

# HELP test_histogram A test histogram metric
# TYPE test_histogram histogram
test_histogram_bucket{le="0.5"} 10
test_histogram_bucket{le="1"} 20
test_histogram_bucket{le="+Inf"} 30
test_histogram_sum 25
test_histogram_count 30
`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprint(w, metricsPage)
	}))
	defer server.Close()

	serverURL, err := url.Parse(server.URL)
	require.NoError(t, err)

	// Create Prometheus scrape config
	cfgMap := map[string]any{
		"scrape_configs": []map[string]any{
			{
				"job_name":        "test",
				"scrape_interval": "100ms",
				"scrape_timeout":  "50ms",
				"static_configs": []map[string]any{
					{"targets": []string{serverURL.Host}},
				},
			},
		},
	}
	cfgBytes, err := yaml.Marshal(cfgMap)
	require.NoError(t, err)

	promCfg, err := config.Load(string(cfgBytes), promslog.NewNopLogger())
	require.NoError(t, err)

	// Create capturing appendable - we expect at least 8 calls:
	// test_counter_total, test_gauge, 5 histogram samples, plus up metric
	capturing := newCapturingAppendableV2(8)

	ctx := t.Context()

	logger := promslog.NewNopLogger()

	// Create a prometheus registry for metrics
	reg := prometheus.NewRegistry()

	// Create discovery manager
	refreshMetrics := discovery.NewRefreshMetrics(reg)
	sdMetrics, err := discovery.RegisterSDMetrics(reg, refreshMetrics)
	require.NoError(t, err)
	discoveryManager := discovery.NewManager(ctx, logger, reg, sdMetrics)
	require.NotNil(t, discoveryManager)

	go func() {
		_ = discoveryManager.Run()
	}()

	// Create scrape manager with our capturing appendable
	// The scrape manager signature: NewManager(opts, logger, notifierV1, store, storeV2, registerer)
	scrapeManager, err := scrape.NewManager(
		&scrape.Options{PassMetadataInContext: true},
		logger,
		nil,       // Notifier V1
		nil,       // Appendable (V1) - nil since we're testing V2
		capturing, // AppendableV2 (V2) - our capturing implementation
		reg,       // Registerer
	)
	require.NoError(t, err)

	// Apply scrape config
	err = scrapeManager.ApplyConfig(promCfg)
	require.NoError(t, err)

	// Apply discovery config
	c := make(map[string]discovery.Configs)
	for _, v := range promCfg.ScrapeConfigs {
		c[v.JobName] = v.ServiceDiscoveryConfigs
	}
	err = discoveryManager.ApplyConfig(c)
	require.NoError(t, err)

	// Run scrape manager with discovery manager's sync channel
	go func() {
		err := scrapeManager.Run(discoveryManager.SyncCh())
		if err != nil && ctx.Err() == nil {
			t.Logf("scrape manager error: %v", err)
		}
	}()

	// Wait for captures or timeout
	select {
	case <-capturing.done:
		// Got enough captures
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for scrape")
	}

	// Stop scrape manager
	scrapeManager.Stop()

	// Verify what metadata Prometheus provided
	captured := capturing.getCaptured()
	require.NotEmpty(t, captured, "expected at least some Append calls")

	// Find the counter metric
	var counterCall *capturedAppendCall
	for i := range captured {
		if captured[i].labels.Get(model.MetricNameLabel) == "test_counter_total" {
			counterCall = &captured[i]
			break
		}
	}

	// =========================================================================
	// THIS IS THE ACTUAL BUG TEST
	// =========================================================================
	// When the upstream Prometheus bug is fixed, these assertions should change:
	// - metadataType should be "counter", not ""
	// - metricFamilyName should be "test_counter", not ""
	//
	// Current behavior (bug):
	require.NotNil(t, counterCall, "expected to capture test_counter_total metric")
	assert.Empty(t, counterCall.metadataType,
		"BUG: Prometheus should provide metadata.Type='counter' but provides empty string.")
	assert.Empty(t, counterCall.metricFamilyName,
		"BUG: Prometheus should provide MetricFamilyName='test_counter' but provides empty string.")

	// Log all captured calls for visibility
	t.Logf("=== All captured Append calls (%d total) ===", len(captured))
	for i, call := range captured {
		metricName := call.labels.Get(model.MetricNameLabel)
		t.Logf("  [%d] metric=%q labels=%s type=%q familyName=%q help=%q unit=%q",
			i, metricName, call.labels.String(), call.metadataType, call.metricFamilyName, call.metadataHelp, call.metadataUnit)
	}
}
