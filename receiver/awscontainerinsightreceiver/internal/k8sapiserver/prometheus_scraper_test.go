package k8sapiserver

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/simpleprometheusreceiver"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"
)

const renameMetric = `
# HELP http_go_threads Number of OS threads created
# TYPE http_go_threads gauge
http_go_threads 19
# HELP http_connected_total connected clients
# TYPE http_connected_total counter
http_connected_total{method="post",port="6380"} 15.0
# HELP redis_http_requests_total Redis connected clients
# TYPE redis_http_requests_total counter
redis_http_requests_total{method="post",port="6380"} 10.0
redis_http_requests_total{method="post",port="6381"} 12.0
# HELP rpc_duration_total RPC clients
# TYPE rpc_duration_total counter
rpc_duration_total{method="post",port="6380"} 100.0
rpc_duration_total{method="post",port="6381"} 120.0
`

type mockConsumer struct {
	t             *testing.T
	up            *bool
	httpConnected *bool
}

func (m mockConsumer) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{
		MutatesData: false,
	}
}

func (m mockConsumer) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	assert.Equal(m.t, 1, md.ResourceMetrics().Len())

	scopeMetrics := md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()
	for i := 0; i < scopeMetrics.Len(); i++ {
		metric := scopeMetrics.At(i)
		if metric.Name() == "http_connected_total" {
			assert.Equal(m.t, float64(15), metric.Sum().DataPoints().At(0).DoubleValue())
			*m.httpConnected = true
		}
		if metric.Name() == "up" {
			assert.Equal(m.t, float64(1), metric.Gauge().DataPoints().At(0).DoubleValue())
			*m.up = true
		}
	}

	return nil
}
func TestNewPrometheusScraperEndToEnd(t *testing.T) {

	upPtr := false
	httpPtr := false

	consumer := mockConsumer{
		t:             t,
		up:            &upPtr,
		httpConnected: &httpPtr,
	}

	settings := componenttest.NewNopTelemetrySettings()
	settings.Logger, _ = zap.NewDevelopment()

	scraper, err := NewPrometheusScraper(context.TODO(), settings, consumer, componenttest.NewNopHost(), mockClusterNameProvider{})
	assert.NoError(t, err)
	assert.Equal(t, mockClusterNameProvider{}, scraper.clusterNameProvider)

	// build up a new SPR
	spFactory := simpleprometheusreceiver.NewFactory()

	targets := []*testData{
		{
			name: "prometheus_simple",
			pages: []mockPrometheusResponse{
				{code: 200, data: renameMetric},
			},
		},
	}
	mp, cfg, err := setupMockPrometheus(targets...)

	split := strings.Split(mp.srv.URL, "http://")

	spConfig := simpleprometheusreceiver.Config{
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Endpoint: split[1],
			TLSSetting: configtls.TLSClientSetting{
				Insecure:           true,
				InsecureSkipVerify: true,
			},
		},
		MetricsPath:        cfg.ScrapeConfigs[0].MetricsPath,
		CollectionInterval: time.Duration(cfg.ScrapeConfigs[0].ScrapeInterval),
		UseServiceAccount:  false,
	}

	// replace the SPR
	params := receiver.CreateSettings{
		TelemetrySettings: scraper.settings,
	}
	scraper.simplePrometheusReceiver, err = spFactory.CreateMetricsReceiver(scraper.ctx, params, &spConfig, consumer)
	assert.NoError(t, err)
	assert.NotNil(t, mp)
	defer mp.Close()

	scraper.Start()

	t.Cleanup(func() {
		scraper.Shutdown()
	})

	// wait for scrape
	mp.wg.Wait()

	assert.True(t, *consumer.up)
	assert.True(t, *consumer.httpConnected)
}
