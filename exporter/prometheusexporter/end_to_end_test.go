// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusexporter

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"gopkg.in/yaml.v2"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusexporter/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver"
)

func TestEndToEndSummarySupport(t *testing.T) {
	if testing.Short() {
		t.Skip("This test can take a couple of seconds")
	}

	//nolint:gosec // the following triggers G101: Potential hardcoded credentials
	dropWizardResponse := `
# HELP jvm_memory_pool_bytes_used Used bytes of a given JVM memory pool.
# TYPE jvm_memory_pool_bytes_used gauge
jvm_memory_pool_bytes_used{pool="CodeHeap 'non-nmethods'",} 1277952.0
jvm_memory_pool_bytes_used{pool="Metaspace",} 2.6218176E7
jvm_memory_pool_bytes_used{pool="CodeHeap 'profiled nmethods'",} 6871168.0
jvm_memory_pool_bytes_used{pool="Compressed Class Space",} 2751312.0
jvm_memory_pool_bytes_used{pool="G1 Eden Space",} 4.4040192E7
jvm_memory_pool_bytes_used{pool="G1 Old Gen",} 4385408.0
jvm_memory_pool_bytes_used{pool="G1 Survivor Space",} 8388608.0
jvm_memory_pool_bytes_used{pool="CodeHeap 'non-profiled nmethods'",} 2869376.0
# HELP jvm_info JVM version info
# TYPE jvm_info gauge
jvm_info{version="9.0.4+11",vendor="Oracle Corporation",} 1.0
# HELP jvm_gc_collection_seconds Time spent in a given JVM garbage collector in seconds.
# TYPE jvm_gc_collection_seconds summary
jvm_gc_collection_seconds_count{gc="G1 Young Generation",} 9.0
jvm_gc_collection_seconds_sum{gc="G1 Young Generation",} 0.229
jvm_gc_collection_seconds_count{gc="G1 Old Generation",} 0.0
jvm_gc_collection_seconds_sum{gc="G1 Old Generation",} 0.0`

	server := NewE2ETestServer(t, dropWizardResponse)
	defer server.Close()

	exporterCfg := &Config{
		Namespace: "test",
		ServerConfig: confighttp.ServerConfig{
			Endpoint: "localhost:8787",
		},
		SendTimestamps:   true,
		MetricExpiration: 2 * time.Hour,
	}

	wantLineRegexps := []string{
		`. HELP test_jvm_gc_collection_seconds Time spent in a given JVM garbage collector in seconds.`,
		`. TYPE test_jvm_gc_collection_seconds summary`,
		`test_jvm_gc_collection_seconds_sum.gc="G1 Old Generation",instance="127.0.0.1:.*",job="otel-collector". 0.*`,
		`test_jvm_gc_collection_seconds_count.gc="G1 Old Generation",instance="127.0.0.1:.*",job="otel-collector". 0.*`,
		`test_jvm_gc_collection_seconds_sum.gc="G1 Young Generation",instance="127.0.0.1:.*",job="otel-collector". 0.*`,
		`test_jvm_gc_collection_seconds_count.gc="G1 Young Generation",instance="127.0.0.1:.*",job="otel-collector". 9.*`,
		`. HELP test_jvm_info JVM version info`,
		`. TYPE test_jvm_info gauge`,
		`test_jvm_info.instance="127.0.0.1:.*",job="otel-collector",vendor="Oracle Corporation",version="9.0.4.11". 1.*`,
		`. HELP test_jvm_memory_pool_bytes_used Used bytes of a given JVM memory pool.`,
		`. TYPE test_jvm_memory_pool_bytes_used gauge`,
		`test_jvm_memory_pool_bytes_used.instance="127.0.0.1:.*",job="otel-collector",pool="CodeHeap 'non.nmethods'". 1.277952e.06.*`,
		`test_jvm_memory_pool_bytes_used.instance="127.0.0.1:.*",job="otel-collector",pool="CodeHeap 'non.profiled nmethods'". 2.869376e.06.*`,
		`test_jvm_memory_pool_bytes_used.instance="127.0.0.1:.*",job="otel-collector",pool="CodeHeap 'profiled nmethods'". 6.871168e.06.*`,
		`test_jvm_memory_pool_bytes_used.instance="127.0.0.1:.*",job="otel-collector",pool="Compressed Class Space". 2.751312e.06.*`,
		`test_jvm_memory_pool_bytes_used.instance="127.0.0.1:.*",job="otel-collector",pool="G1 Eden Space". 4.4040192e.07.*`,
		`test_jvm_memory_pool_bytes_used.instance="127.0.0.1:.*",job="otel-collector",pool="G1 Old Gen". 4.385408e.06.*`,
		`test_jvm_memory_pool_bytes_used.instance="127.0.0.1:.*",job="otel-collector",pool="G1 Survivor Space". 8.388608e.06.*`,
		`test_jvm_memory_pool_bytes_used.instance="127.0.0.1:.*",job="otel-collector",pool="Metaspace". 2.6218176e.07.*`,
		`. HELP test_scrape_duration_seconds Duration of the scrape`,
		`. TYPE test_scrape_duration_seconds gauge`,
		`test_scrape_duration_seconds.instance="127.0.0.1:.*",job="otel-collector". [0-9.e-]+ [0-9]+`,
		`. HELP test_scrape_samples_post_metric_relabeling The number of samples remaining after metric relabeling was applied`,
		`. TYPE test_scrape_samples_post_metric_relabeling gauge`,
		`test_scrape_samples_post_metric_relabeling.instance="127.0.0.1:.*",job="otel-collector". 13 .*`,
		`. HELP test_scrape_samples_scraped The number of samples the target exposed`,
		`. TYPE test_scrape_samples_scraped gauge`,
		`test_scrape_samples_scraped.instance="127.0.0.1:.*",job="otel-collector". 13 .*`,
		`. HELP test_scrape_series_added The approximate number of new series in this scrape`,
		`. TYPE test_scrape_series_added gauge`,
		`test_scrape_series_added.instance="127.0.0.1:.*",job="otel-collector". 13 .*`,
		`. HELP test_up The scraping was successful`,
		`. TYPE test_up gauge`,
		`test_up.instance="127.0.0.1:.*",job="otel-collector". 1 .*`,
		`. HELP test_target_info Target metadata`,
		`. TYPE test_target_info gauge`,
		`test_target_info.http_scheme=\"http\",instance="127.0.0.1:.*",job="otel-collector",net_host_port=".*,server_port=".*",url_scheme="http". 1`,
	}

	server.RunTest(t, exporterCfg, wantLineRegexps)
}

func TestUTF8EscapingWithSuffixes(t *testing.T) {
	if testing.Short() {
		t.Skip("This test can take a couple of seconds")
	}

	// Declare a dotted metric and label name.
	rawResponse := `
# HELP "my.metric" an escaped metric name.
# TYPE "my.metric" counter
{"my.metric", "my.label"="my.value"} 20.0
`

	server := NewE2ETestServer(t, rawResponse)
	defer server.Close()

	exporterCfg := &Config{
		Namespace: "test",
		ServerConfig: confighttp.ServerConfig{
			Endpoint: "localhost:8787",
		},
		SendTimestamps:    true,
		MetricExpiration:  2 * time.Hour,
		AddMetricSuffixes: true,
	}

	// Confirm that metric and label names are escaped.
	want := []string{
		`# HELP test_my_metric_total an escaped metric name.`,
		`# TYPE test_my_metric_total counter`,
		`test_my_metric_total{instance="127.0.0.1:[0-9]*",job="otel-collector",my_label="my.value"} 20 .*`,
		`# HELP test_scrape_duration_seconds Duration of the scrape`,
		`# TYPE test_scrape_duration_seconds gauge`,
		`test_scrape_duration_seconds{instance="127.0.0.1:[0-9]*",job="otel-collector"} .* .*`,
		`# HELP test_scrape_samples_post_metric_relabeling The number of samples remaining after metric relabeling was applied`,
		`# TYPE test_scrape_samples_post_metric_relabeling gauge`,
		`test_scrape_samples_post_metric_relabeling{instance="127.0.0.1:[0-9]*",job="otel-collector"} 1 .*`,
		`# HELP test_scrape_samples_scraped The number of samples the target exposed`,
		`# TYPE test_scrape_samples_scraped gauge`,
		`test_scrape_samples_scraped{instance="127.0.0.1:[0-9]*",job="otel-collector"} 1 .*`,
		`# HELP test_scrape_series_added The approximate number of new series in this scrape`,
		`# TYPE test_scrape_series_added gauge`,
		`test_scrape_series_added{instance="127.0.0.1:[0-9]*",job="otel-collector"} 1 .*`,
		`# HELP test_target_info Target metadata`,
		`# TYPE test_target_info gauge`,
		`test_target_info{http_scheme="http",instance="127.0.0.1:[0-9]*",job="otel-collector",net_host_port="[0-9]*",server_port="[0-9]*",url_scheme="http"} 1`,
		`# HELP test_up The scraping was successful`,
		`# TYPE test_up gauge`,
		`test_up{instance="127.0.0.1:[0-9]*",job="otel-collector"} 1 .*`,
	}

	server.RunTest(t, exporterCfg, want)
}

func TestUTF8EscapingNoSuffixes(t *testing.T) {
	if testing.Short() {
		t.Skip("This test can take a couple of seconds")
	}

	// Declare a dotted metric and label name.
	rawResponse := `
# HELP "my.metric" an escaped metric name.
# TYPE "my.metric" counter
{"my.metric", "my.label"="my.value"} 20.0
`

	server := NewE2ETestServer(t, rawResponse)
	defer server.Close()

	exporterCfg := &Config{
		Namespace: "test",
		ServerConfig: confighttp.ServerConfig{
			Endpoint: "localhost:8787",
		},
		SendTimestamps:    true,
		MetricExpiration:  2 * time.Hour,
		EnableOpenMetrics: true,
		AddMetricSuffixes: false,
	}

	// Confirm that metric and label names are escaped.
	want := []string{
		`# HELP test_my_metric an escaped metric name.`,
		`# TYPE test_my_metric counter`,
		`test_my_metric{instance="127.0.0.1:[0-9]*",job="otel-collector",my_label="my.value"} 20 .*`,
		`# HELP test_scrape_duration_seconds Duration of the scrape`,
		`# TYPE test_scrape_duration_seconds gauge`,
		`test_scrape_duration_seconds{instance="127.0.0.1:[0-9]*",job="otel-collector"} .* .*`,
		`# HELP test_scrape_samples_post_metric_relabeling The number of samples remaining after metric relabeling was applied`,
		`# TYPE test_scrape_samples_post_metric_relabeling gauge`,
		`test_scrape_samples_post_metric_relabeling{instance="127.0.0.1:[0-9]*",job="otel-collector"} 1 .*`,
		`# HELP test_scrape_samples_scraped The number of samples the target exposed`,
		`# TYPE test_scrape_samples_scraped gauge`,
		`test_scrape_samples_scraped{instance="127.0.0.1:[0-9]*",job="otel-collector"} 1 .*`,
		`# HELP test_scrape_series_added The approximate number of new series in this scrape`,
		`# TYPE test_scrape_series_added gauge`,
		`test_scrape_series_added{instance="127.0.0.1:[0-9]*",job="otel-collector"} 1 .*`,
		`# HELP test_target_info Target metadata`,
		`# TYPE test_target_info gauge`,
		`test_target_info{http_scheme="http",instance="127.0.0.1:[0-9]*",job="otel-collector",net_host_port="[0-9]*",server_port="[0-9]*",url_scheme="http"} 1`,
		`# HELP test_up The scraping was successful`,
		`# TYPE test_up gauge`,
		`test_up{instance="127.0.0.1:[0-9]*",job="otel-collector"} 1 .*`,
	}

	server.RunTest(t, exporterCfg, want)
}

type e2eTestServer struct {
	*httptest.Server
	scrapeConfig       string
	currentScrapeIndex int
	wg                 sync.WaitGroup
}

func NewE2ETestServer(t *testing.T, injectResponse string) *e2eTestServer {
	// 1. Create the Prometheus scrape endpoint.
	var server e2eTestServer
	server.Server = httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, _ *http.Request) {
		// Serve back the metrics as if they were from DropWizard.
		_, err := rw.Write([]byte(injectResponse))
		assert.NoError(t, err)
		server.currentScrapeIndex++
		if server.currentScrapeIndex == 8 { // We shall let the Prometheus receiver scrape the DropWizard mock server, at least 8 times.
			server.wg.Done() // done scraping response 8 times
		}
	}))

	srvURL, err := url.Parse(server.URL)
	require.NoError(t, err)

	server.scrapeConfig = fmt.Sprintf(`
        global:
          scrape_interval: 2ms

        scrape_configs:
            - job_name: 'otel-collector'
              scrape_interval: 50ms
              scrape_timeout: 50ms
              static_configs:
                - targets: ['%s']
        `, srvURL.Host)

	return &server
}

func (s *e2eTestServer) RunTest(t *testing.T, exporterConfig *Config, wantLineRegexps []string) {
	s.wg.Add(1) // scrape one endpoint
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	exporterFactory := NewFactory()
	set := exportertest.NewNopSettings(metadata.Type)
	exporter, err := exporterFactory.CreateMetrics(ctx, set, exporterConfig)
	require.NoError(t, err)
	require.NoError(t, exporter.Start(ctx, nil), "Failed to start the Prometheus exporter")
	t.Cleanup(func() { require.NoError(t, exporter.Shutdown(ctx)) })

	// 3. Create the Prometheus receiver scraping from the DropWizard mock server and
	// it'll feed scraped and converted metrics then pass them to the Prometheus exporter.
	yamlConfig := []byte(s.scrapeConfig)
	receiverConfig := new(prometheusreceiver.PromConfig)
	require.NoError(t, yaml.Unmarshal(yamlConfig, receiverConfig))

	receiverFactory := prometheusreceiver.NewFactory()
	receiverCreateSet := receivertest.NewNopSettings(metadata.Type)
	rcvCfg := &prometheusreceiver.Config{
		PrometheusConfig: receiverConfig,
	}
	// 3.5 Create the Prometheus receiver and pass in the previously created Prometheus exporter.
	prometheusReceiver, err := receiverFactory.CreateMetrics(ctx, receiverCreateSet, rcvCfg, exporter)
	require.NoError(t, err)
	require.NoError(t, prometheusReceiver.Start(ctx, nil), "Failed to start the Prometheus receiver")
	t.Cleanup(func() { require.NoError(t, prometheusReceiver.Shutdown(ctx)) })

	// 4. Scrape from the Prometheus receiver to ensure that we export summary metrics
	s.wg.Wait()

	req, err := http.NewRequest("GET", "http://"+exporterConfig.Endpoint+"/metrics", nil)
	require.NoError(t, err, "Failed to construct request")
	req.Header.Add("Accept", "text/plain; version=1.0.0; charset=utf-8; escaping=allow-utf-8")
	client := &http.Client{}
	res, err := client.Do(req)
	require.NoError(t, err, "Failed to scrape from the exporter")
	defer res.Body.Close()
	prometheusExporterScrape, err := io.ReadAll(res.Body)
	require.NoError(t, err)

	// 5. Verify that we have the summary metrics and that their values make
	// sense. Perform a complete line by line prefix verification to ensure we
	// extract back the inputs we'd expect after scraping Prometheus.
	for _, wantLineRegexp := range wantLineRegexps {
		reg := regexp.MustCompile(wantLineRegexp)
		prometheusExporterScrape = reg.ReplaceAll(prometheusExporterScrape, []byte(""))
	}
	// After this replacement, there should ONLY be newlines present.
	prometheusExporterScrape = bytes.ReplaceAll(prometheusExporterScrape, []byte("\n"), []byte(""))
	// Now assert that NO output was left over.
	require.Empty(t, prometheusExporterScrape, "Left-over unmatched Prometheus scrape content: %q\n", prometheusExporterScrape)
}
