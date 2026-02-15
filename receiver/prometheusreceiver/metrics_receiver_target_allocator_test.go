// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusreceiver

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/goccy/go-yaml"
	promTestUtil "github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/prometheus/common/model"
	"github.com/prometheus/common/promslog"
	promConfig "github.com/prometheus/prometheus/config"
	promHTTP "github.com/prometheus/prometheus/discovery/http"
	promTG "github.com/prometheus/prometheus/discovery/targetgroup"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver/internal/targetallocator"
)

const exportedMetrics = `
# HELP test_gauge0 This is my gauge
# TYPE test_gauge0 gauge
test_gauge0{label1="value1",label2="value2"} 10
`

func TestTargetAllocatorProvidesEmptyScrapeConfig(t *testing.T) {
	// Make a prometheus exporter that can serve some metrics.
	mockProm := newMockPrometheus(map[string][]mockPrometheusResponse{
		"/metrics": {
			{
				code: 200,
				data: exportedMetrics,
			},
		},
	})
	t.Cleanup(func() { mockProm.srv.Close() })

	// Fake TargetAllocator to serve discovery and targets.
	tas := newMockTargetAllocator(mockProm.srv.Listener.Addr().String())
	t.Cleanup(func() { tas.srv.Close() })

	promSDConfig := &promHTTP.SDConfig{
		RefreshInterval: model.Duration(45 * time.Second),
		URL:             tas.srv.URL,
	}

	pCfg, err := promConfig.Load("", promslog.NewNopLogger())
	require.NoError(t, err)

	config := &Config{
		PrometheusConfig: (*PromConfig)(pCfg),
		TargetAllocator: configoptional.Some(targetallocator.Config{
			ClientConfig: confighttp.ClientConfig{
				Endpoint: tas.srv.URL,
			},
			CollectorID:  "1",
			HTTPSDConfig: (*targetallocator.PromHTTPSDConfig)(promSDConfig),
			Interval:     60 * time.Second,
		}),
		skipOffsetting: true,
	}

	settings := receivertest.NewNopSettings(metadata.Type)
	warnCh := make(chan struct{}, 1)
	settings.Logger, err = zap.NewDevelopment(zap.Hooks(func(logentry zapcore.Entry) error {
		if logentry.Level >= zapcore.WarnLevel {
			select {
			case warnCh <- struct{}{}:
			default:
			}
		}
		return nil
	}))
	require.NoError(t, err)

	cms := newSignalingSink(1)
	receiver := newTestReceiverSettingsConsumer(t, config, settings, cms)

	select {
	case <-cms.done:
		// Success - at least one scrape was consumed.
	case <-warnCh:
		t.Fatal("Unexpected log message at WARN level or above, see logs")
	case <-time.After(30 * time.Second):
		t.Fatal("Timed out waiting for metrics from target allocator")
	}

	require.NoError(t, promTestUtil.GatherAndCompare(receiver.registry, bytes.NewBufferString(fmt.Sprintf(`
		# TYPE prometheus_target_scrape_pools_failed_total counter
		# HELP prometheus_target_scrape_pools_failed_total Total number of scrape pool creations that failed.
		prometheus_target_scrape_pools_failed_total{receiver="%s"} 0
`, receiver.settings.ID)), "prometheus_target_scrape_pools_failed_total"), "Prometheus scrape manager reports failed scrape pools")

	require.NotEmpty(t, cms.AllMetrics(), "No metrics were scraped even though successful")
}

type mockTargetAllocator struct {
	address string
	srv     *httptest.Server
}

func newMockTargetAllocator(address string) *mockTargetAllocator {
	s := &mockTargetAllocator{
		address: address,
	}
	srv := httptest.NewServer(s)
	s.srv = srv
	return s
}

func (mp *mockTargetAllocator) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	if strings.HasSuffix(req.URL.Path, "/scrape_configs") {
		job := make(map[string]any)
		job["job_name"] = "test"
		// Do not set any fields in the scrape config to verify that we have sane defaults.

		result := make(map[string]any)
		result["test"] = job

		data, err := yaml.Marshal(&result)
		if err != nil {
			return
		}

		_, _ = rw.Write(data)
		return
	}

	response := []*promTG.Group{
		{
			Targets: []model.LabelSet{
				{
					model.AddressLabel: model.LabelValue(mp.address),
					model.SchemeLabel:  "http",
				},
			},
		},
	}

	data, err := json.Marshal(&response)
	if err != nil {
		return
	}
	rw.Header().Set("Content-Type", "application/json")
	_, _ = rw.Write(data)
}
