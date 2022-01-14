package elasticsearchreceiver

import (
	"context"
	"encoding/json"
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/scrapertest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/scrapertest/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/elasticsearchreceiver/internal/mocks"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/elasticsearchreceiver/internal/model"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/receiver/scrapererror"
	"go.uber.org/zap"
)

const expectedMetricsPath = "./testdata/expected_metrics/expected.json"

func TestScraper(t *testing.T) {
	t.Parallel()

	sc := newElasticSearchScraper(zap.NewNop(), createDefaultConfig().(*Config))

	err := sc.start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)

	mockClient := mocks.MockElasticsearchClient{}
	mockClient.On("ClusterHealth", mock.Anything).Return(clusterHealth(t), nil)
	mockClient.On("NodeStats", mock.Anything, []string{"_all"}).Return(nodeStats(t), nil)

	sc.client = &mockClient

	expectedMetrics, err := golden.ReadMetrics(expectedMetricsPath)
	require.NoError(t, err)

	actualMetrics, err := sc.scrape(context.Background())
	require.NoError(t, err)

	expectedMetricsSlice := expectedMetrics.ResourceMetrics().At(0).InstrumentationLibraryMetrics().At(0).Metrics()
	actualMetricsSlice := actualMetrics.ResourceMetrics().At(0).InstrumentationLibraryMetrics().At(0).Metrics()

	err = scrapertest.CompareMetricSlices(expectedMetricsSlice, actualMetricsSlice)
	require.NoError(t, err)
}

func TestScraperFailedStart(t *testing.T) {
	t.Parallel()

	conf := createDefaultConfig().(*Config)

	conf.HTTPClientSettings = confighttp.HTTPClientSettings{
		Endpoint: "localhost:9200",
		TLSSetting: configtls.TLSClientSetting{
			TLSSetting: configtls.TLSSetting{
				CAFile: "/non/existent",
			},
		},
	}

	conf.Username = "dev"
	conf.Password = "dev"

	sc := newElasticSearchScraper(zap.NewNop(), conf)

	err := sc.start(context.Background(), componenttest.NewNopHost())
	require.Error(t, err)
}

func TestScrapingError(t *testing.T) {
	testCases := []struct {
		desc string
		run  func(t *testing.T)
	}{
		{
			desc: "Node stats fails, but cluster health succeeds",
			run: func(t *testing.T) {
				t.Parallel()

				err404 := errors.New("expected status 200 but got 404")

				mockClient := mocks.MockElasticsearchClient{}
				mockClient.On("NodeStats", mock.Anything, []string{"_all"}).Return(nil, err404)
				mockClient.On("ClusterHealth", mock.Anything).Return(clusterHealth(t), nil)

				sc := newElasticSearchScraper(zap.NewNop(), createDefaultConfig().(*Config))
				err := sc.start(context.Background(), componenttest.NewNopHost())
				require.NoError(t, err)

				sc.client = &mockClient

				_, err = sc.scrape(context.Background())
				require.True(t, scrapererror.IsPartialScrapeError(err))
				require.Equal(t, err.Error(), err404.Error())

			},
		},
		{
			desc: "Cluster health fails, but node stats succeeds",
			run: func(t *testing.T) {
				t.Parallel()

				err404 := errors.New("expected status 200 but got 404")

				mockClient := mocks.MockElasticsearchClient{}
				mockClient.On("NodeStats", mock.Anything, []string{"_all"}).Return(nodeStats(t), nil)
				mockClient.On("ClusterHealth", mock.Anything).Return(nil, err404)

				sc := newElasticSearchScraper(zap.NewNop(), createDefaultConfig().(*Config))
				err := sc.start(context.Background(), componenttest.NewNopHost())
				require.NoError(t, err)

				sc.client = &mockClient

				_, err = sc.scrape(context.Background())
				require.True(t, scrapererror.IsPartialScrapeError(err))
				require.Equal(t, err.Error(), err404.Error())

			},
		},
		{
			desc: "Both node stats and cluster health fails",
			run: func(t *testing.T) {
				t.Parallel()

				err404 := errors.New("expected status 200 but got 404")
				err500 := errors.New("expected status 200 but got 500")

				mockClient := mocks.MockElasticsearchClient{}
				mockClient.On("NodeStats", mock.Anything, []string{"_all"}).Return(nil, err500)
				mockClient.On("ClusterHealth", mock.Anything).Return(nil, err404)

				sc := newElasticSearchScraper(zap.NewNop(), createDefaultConfig().(*Config))
				err := sc.start(context.Background(), componenttest.NewNopHost())
				require.NoError(t, err)

				sc.client = &mockClient

				m, err := sc.scrape(context.Background())
				require.Contains(t, err.Error(), err404.Error())
				require.Contains(t, err.Error(), err500.Error())

				require.Equal(t, m.DataPointCount(), 0)
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.desc, testCase.run)
	}
}

func clusterHealth(t *testing.T) *model.ClusterHealth {
	healthJson, err := ioutil.ReadFile("./testdata/sample_payloads/health.json")
	require.NoError(t, err)

	clusterHealth := model.ClusterHealth{}
	require.NoError(t, json.Unmarshal(healthJson, &clusterHealth))

	return &clusterHealth
}

func nodeStats(t *testing.T) *model.NodeStats {
	nodeJson, err := ioutil.ReadFile("./testdata/sample_payloads/nodes_linux.json")
	require.NoError(t, err)

	nodeStats := model.NodeStats{}
	require.NoError(t, json.Unmarshal(nodeJson, &nodeStats))
	return &nodeStats
}

// TestWriteGolden writes golden metrics to testdata; It should only be used to generate golden metrics when changes to metrics are made.
func TestWriteGolden(t *testing.T) {
	t.SkipNow()

	sc := newElasticSearchScraper(zap.NewNop(), createDefaultConfig().(*Config))

	err := sc.start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)

	mockClient := mocks.MockElasticsearchClient{}
	mockClient.On("ClusterHealth", mock.Anything).Return(clusterHealth(t), nil)
	mockClient.On("NodeStats", mock.Anything, mock.Anything).Return(nodeStats(t), nil)

	sc.client = &mockClient

	actualMetrics, err := sc.scrape(context.Background())
	require.NoError(t, err)

	expectedDir := filepath.Dir(expectedMetricsPath)
	err = os.MkdirAll(expectedDir, 0777)
	require.NoError(t, err)

	err = golden.WriteMetrics(expectedMetricsPath, actualMetrics)
	require.NoError(t, err)
}
