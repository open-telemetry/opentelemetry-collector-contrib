// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package elasticsearchreceiver

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.opentelemetry.io/collector/receiver/scrapererror"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/elasticsearchreceiver/internal/mocks"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/elasticsearchreceiver/internal/model"
)

const fullExpectedMetricsPath = "./testdata/expected_metrics/full.yaml"
const skipClusterExpectedMetricsPath = "./testdata/expected_metrics/clusterSkip.yaml"
const noNodesExpectedMetricsPath = "./testdata/expected_metrics/noNodes.yaml"

func TestScraper(t *testing.T) {
	t.Parallel()

	config := createDefaultConfig().(*Config)

	config.Metrics.ElasticsearchNodeOperationsGetCompleted.Enabled = true
	config.Metrics.ElasticsearchNodeOperationsGetTime.Enabled = true
	config.Metrics.ElasticsearchNodeSegmentsMemory.Enabled = true

	config.Metrics.JvmMemoryHeapUtilization.Enabled = true

	config.Metrics.ElasticsearchNodeOperationsCurrent.Enabled = true

	config.Metrics.ElasticsearchIndexOperationsMergeSize.Enabled = true
	config.Metrics.ElasticsearchIndexOperationsMergeDocsCount.Enabled = true
	config.Metrics.ElasticsearchIndexSegmentsCount.Enabled = true
	config.Metrics.ElasticsearchIndexSegmentsSize.Enabled = true
	config.Metrics.ElasticsearchIndexSegmentsMemory.Enabled = true
	config.Metrics.ElasticsearchIndexTranslogOperations.Enabled = true
	config.Metrics.ElasticsearchIndexTranslogSize.Enabled = true
	config.Metrics.ElasticsearchIndexCacheMemoryUsage.Enabled = true
	config.Metrics.ElasticsearchIndexCacheSize.Enabled = true
	config.Metrics.ElasticsearchIndexCacheEvictions.Enabled = true
	config.Metrics.ElasticsearchIndexDocuments.Enabled = true

	config.Metrics.ElasticsearchClusterIndicesCacheEvictions.Enabled = true

	config.Metrics.ElasticsearchNodeCacheSize.Enabled = true
	config.Metrics.ElasticsearchProcessCPUUsage.Enabled = true
	config.Metrics.ElasticsearchProcessCPUTime.Enabled = true
	config.Metrics.ElasticsearchProcessMemoryVirtual.Enabled = true

	sc := newElasticSearchScraper(receivertest.NewNopCreateSettings(), config)

	err := sc.start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)

	mockClient := mocks.MockElasticsearchClient{}
	mockClient.On("ClusterMetadata", mock.Anything).Return(clusterMetadata(t), nil)
	mockClient.On("ClusterHealth", mock.Anything).Return(clusterHealth(t), nil)
	mockClient.On("ClusterStats", mock.Anything, []string{"_all"}).Return(clusterStats(t), nil)
	mockClient.On("Nodes", mock.Anything, []string{"_all"}).Return(nodes(t), nil)
	mockClient.On("NodeStats", mock.Anything, []string{"_all"}).Return(nodeStats(t), nil)
	mockClient.On("IndexStats", mock.Anything, []string{"_all"}).Return(indexStats(t), nil)

	sc.client = &mockClient

	expectedMetrics, err := golden.ReadMetrics(fullExpectedMetricsPath)
	require.NoError(t, err)

	actualMetrics, err := sc.scrape(context.Background())
	require.NoError(t, err)

	require.NoError(t, pmetrictest.CompareMetrics(expectedMetrics, actualMetrics, pmetrictest.IgnoreResourceMetricsOrder(),
		pmetrictest.IgnoreMetricDataPointsOrder(), pmetrictest.IgnoreStartTimestamp(), pmetrictest.IgnoreTimestamp()))
}

func TestScraperSkipClusterMetrics(t *testing.T) {
	t.Parallel()

	conf := createDefaultConfig().(*Config)
	conf.SkipClusterMetrics = true

	sc := newElasticSearchScraper(receivertest.NewNopCreateSettings(), conf)

	err := sc.start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)

	mockClient := mocks.MockElasticsearchClient{}
	mockClient.On("ClusterMetadata", mock.Anything).Return(clusterMetadata(t), nil)
	mockClient.On("ClusterHealth", mock.Anything).Return(clusterHealth(t), nil)
	mockClient.On("ClusterStats", mock.Anything, []string{}).Return(clusterStats(t), nil)
	mockClient.On("Nodes", mock.Anything, []string{"_all"}).Return(nodes(t), nil)
	mockClient.On("NodeStats", mock.Anything, []string{"_all"}).Return(nodeStats(t), nil)
	mockClient.On("IndexStats", mock.Anything, []string{"_all"}).Return(indexStats(t), nil)

	sc.client = &mockClient

	expectedMetrics, err := golden.ReadMetrics(skipClusterExpectedMetricsPath)
	require.NoError(t, err)

	actualMetrics, err := sc.scrape(context.Background())
	require.NoError(t, err)

	require.NoError(t, pmetrictest.CompareMetrics(expectedMetrics, actualMetrics, pmetrictest.IgnoreResourceMetricsOrder(),
		pmetrictest.IgnoreMetricDataPointsOrder(), pmetrictest.IgnoreStartTimestamp(), pmetrictest.IgnoreTimestamp()))
}

func TestScraperNoNodesMetrics(t *testing.T) {
	t.Parallel()

	conf := createDefaultConfig().(*Config)
	conf.Nodes = []string{}

	sc := newElasticSearchScraper(receivertest.NewNopCreateSettings(), conf)

	err := sc.start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)

	mockClient := mocks.MockElasticsearchClient{}
	mockClient.On("ClusterMetadata", mock.Anything).Return(clusterMetadata(t), nil)
	mockClient.On("ClusterHealth", mock.Anything).Return(clusterHealth(t), nil)
	mockClient.On("ClusterStats", mock.Anything, []string{}).Return(clusterStats(t), nil)
	mockClient.On("Nodes", mock.Anything, []string{"_all"}).Return(nodes(t), nil)
	mockClient.On("NodeStats", mock.Anything, []string{}).Return(nodeStats(t), nil)
	mockClient.On("IndexStats", mock.Anything, []string{"_all"}).Return(indexStats(t), nil)

	sc.client = &mockClient

	expectedMetrics, err := golden.ReadMetrics(noNodesExpectedMetricsPath)
	require.NoError(t, err)

	actualMetrics, err := sc.scrape(context.Background())
	require.NoError(t, err)

	require.NoError(t, pmetrictest.CompareMetrics(expectedMetrics, actualMetrics, pmetrictest.IgnoreResourceMetricsOrder(),
		pmetrictest.IgnoreMetricDataPointsOrder(), pmetrictest.IgnoreStartTimestamp(), pmetrictest.IgnoreTimestamp()))
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

	sc := newElasticSearchScraper(receivertest.NewNopCreateSettings(), conf)

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
				mockClient.On("ClusterMetadata", mock.Anything).Return(clusterMetadata(t), nil)
				mockClient.On("Nodes", mock.Anything, []string{"_all"}).Return(nodes(t), nil)
				mockClient.On("NodeStats", mock.Anything, []string{"_all"}).Return(nil, err404)
				mockClient.On("ClusterHealth", mock.Anything).Return(clusterHealth(t), nil)
				mockClient.On("ClusterStats", mock.Anything, []string{"_all"}).Return(clusterStats(t), nil)
				mockClient.On("IndexStats", mock.Anything, []string{"_all"}).Return(indexStats(t), nil)

				sc := newElasticSearchScraper(receivertest.NewNopCreateSettings(), createDefaultConfig().(*Config))
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
				mockClient.On("ClusterMetadata", mock.Anything).Return(clusterMetadata(t), nil)
				mockClient.On("Nodes", mock.Anything, []string{"_all"}).Return(nodes(t), nil)
				mockClient.On("NodeStats", mock.Anything, []string{"_all"}).Return(nodeStats(t), nil)
				mockClient.On("ClusterHealth", mock.Anything).Return(nil, err404)
				mockClient.On("ClusterStats", mock.Anything, []string{"_all"}).Return(clusterStats(t), nil)
				mockClient.On("IndexStats", mock.Anything, []string{"_all"}).Return(indexStats(t), nil)

				sc := newElasticSearchScraper(receivertest.NewNopCreateSettings(), createDefaultConfig().(*Config))
				err := sc.start(context.Background(), componenttest.NewNopHost())
				require.NoError(t, err)

				sc.client = &mockClient

				_, err = sc.scrape(context.Background())
				require.True(t, scrapererror.IsPartialScrapeError(err))
				require.Equal(t, err.Error(), err404.Error())

			},
		},
		{
			desc: "Node stats, index stats, cluster stats and cluster health fails",
			run: func(t *testing.T) {
				t.Parallel()

				err404 := errors.New("expected status 200 but got 404")
				err500 := errors.New("expected status 200 but got 500")

				mockClient := mocks.MockElasticsearchClient{}
				mockClient.On("ClusterMetadata", mock.Anything).Return(clusterMetadata(t), nil)
				mockClient.On("Nodes", mock.Anything, []string{"_all"}).Return(nodes(t), nil)
				mockClient.On("NodeStats", mock.Anything, []string{"_all"}).Return(nil, err500)
				mockClient.On("ClusterHealth", mock.Anything).Return(nil, err404)
				mockClient.On("ClusterStats", mock.Anything, []string{"_all"}).Return(nil, err404)
				mockClient.On("IndexStats", mock.Anything, []string{"_all"}).Return(nil, err500)

				sc := newElasticSearchScraper(receivertest.NewNopCreateSettings(), createDefaultConfig().(*Config))
				err := sc.start(context.Background(), componenttest.NewNopHost())
				require.NoError(t, err)

				sc.client = &mockClient

				m, err := sc.scrape(context.Background())
				require.Contains(t, err.Error(), err404.Error())
				require.Contains(t, err.Error(), err500.Error())

				require.Equal(t, m.DataPointCount(), 0)
			},
		},
		{
			desc: "ClusterMetadata is invalid, node stats and cluster health succeed",
			run: func(t *testing.T) {
				t.Parallel()

				err404 := errors.New("expected status 200 but got 404")

				mockClient := mocks.MockElasticsearchClient{}
				mockClient.On("ClusterMetadata", mock.Anything).Return(nil, err404)
				mockClient.On("Nodes", mock.Anything, []string{"_all"}).Return(nodes(t), nil)
				mockClient.On("NodeStats", mock.Anything, []string{"_all"}).Return(nodeStats(t), nil)
				mockClient.On("ClusterHealth", mock.Anything).Return(clusterHealth(t), nil)
				mockClient.On("ClusterStats", mock.Anything, []string{"_all"}).Return(clusterStats(t), nil)
				mockClient.On("IndexStats", mock.Anything, []string{"_all"}).Return(indexStats(t), nil)

				sc := newElasticSearchScraper(receivertest.NewNopCreateSettings(), createDefaultConfig().(*Config))
				err := sc.start(context.Background(), componenttest.NewNopHost())
				require.NoError(t, err)

				sc.client = &mockClient

				_, err = sc.scrape(context.Background())
				require.True(t, scrapererror.IsPartialScrapeError(err))
				require.Contains(t, err.Error(), err404.Error())
			},
		},
		{
			desc: "ClusterMetadata, node stats, index stats, cluster stats and cluster health fail",
			run: func(t *testing.T) {
				t.Parallel()

				err404 := errors.New("expected status 200 but got 404")
				err500 := errors.New("expected status 200 but got 500")

				mockClient := mocks.MockElasticsearchClient{}
				mockClient.On("ClusterMetadata", mock.Anything).Return(nil, err404)
				mockClient.On("Nodes", mock.Anything, []string{"_all"}).Return(nodes(t), nil)
				mockClient.On("NodeStats", mock.Anything, []string{"_all"}).Return(nil, err500)
				mockClient.On("ClusterHealth", mock.Anything).Return(nil, err404)
				mockClient.On("IndexStats", mock.Anything, []string{"_all"}).Return(nil, err500)
				mockClient.On("ClusterStats", mock.Anything, []string{"_all"}).Return(nil, err500)

				sc := newElasticSearchScraper(receivertest.NewNopCreateSettings(), createDefaultConfig().(*Config))
				err := sc.start(context.Background(), componenttest.NewNopHost())
				require.NoError(t, err)

				sc.client = &mockClient

				m, err := sc.scrape(context.Background())
				require.Contains(t, err.Error(), err404.Error())
				require.Contains(t, err.Error(), err500.Error())

				require.Equal(t, m.DataPointCount(), 0)
			},
		},
		{
			desc: "Cluster health status is invalid",
			run: func(t *testing.T) {
				t.Parallel()

				ch := clusterHealth(t)
				ch.Status = "pink"

				mockClient := mocks.MockElasticsearchClient{}
				mockClient.On("ClusterMetadata", mock.Anything).Return(clusterMetadata(t), nil)
				mockClient.On("Nodes", mock.Anything, []string{"_all"}).Return(nodes(t), nil)
				mockClient.On("NodeStats", mock.Anything, []string{"_all"}).Return(nodeStats(t), nil)
				mockClient.On("ClusterHealth", mock.Anything).Return(ch, nil)
				mockClient.On("ClusterStats", mock.Anything, []string{"_all"}).Return(clusterStats(t), nil)
				mockClient.On("IndexStats", mock.Anything, []string{"_all"}).Return(indexStats(t), nil)

				sc := newElasticSearchScraper(receivertest.NewNopCreateSettings(), createDefaultConfig().(*Config))
				err := sc.start(context.Background(), componenttest.NewNopHost())
				require.NoError(t, err)

				sc.client = &mockClient

				_, err = sc.scrape(context.Background())
				require.True(t, scrapererror.IsPartialScrapeError(err))
				require.Contains(t, err.Error(), errUnknownClusterStatus.Error())
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.desc, testCase.run)
	}
}

func clusterHealth(t *testing.T) *model.ClusterHealth {
	healthJSON, err := os.ReadFile("./testdata/sample_payloads/health.json")
	require.NoError(t, err)

	clusterHealth := model.ClusterHealth{}
	require.NoError(t, json.Unmarshal(healthJSON, &clusterHealth))

	return &clusterHealth
}

func clusterStats(t *testing.T) *model.ClusterStats {
	statsJSON, err := os.ReadFile("./testdata/sample_payloads/cluster.json")
	require.NoError(t, err)

	clusterStats := model.ClusterStats{}
	require.NoError(t, json.Unmarshal(statsJSON, &clusterStats))

	return &clusterStats
}

func nodes(t *testing.T) *model.Nodes {
	nodeJSON, err := os.ReadFile("./testdata/sample_payloads/nodes_linux.json")
	require.NoError(t, err)

	nodes := model.Nodes{}
	require.NoError(t, json.Unmarshal(nodeJSON, &nodes))
	return &nodes
}

func nodeStats(t *testing.T) *model.NodeStats {
	nodeJSON, err := os.ReadFile("./testdata/sample_payloads/nodes_stats_linux.json")
	require.NoError(t, err)

	nodeStats := model.NodeStats{}
	require.NoError(t, json.Unmarshal(nodeJSON, &nodeStats))
	return &nodeStats
}

func indexStats(t *testing.T) *model.IndexStats {
	indexJSON, err := os.ReadFile("./testdata/sample_payloads/indices.json")
	require.NoError(t, err)

	indexStats := model.IndexStats{}
	require.NoError(t, json.Unmarshal(indexJSON, &indexStats))
	return &indexStats
}

func clusterMetadata(t *testing.T) *model.ClusterMetadataResponse {
	metadataJSON, err := os.ReadFile("./testdata/sample_payloads/metadata.json")
	require.NoError(t, err)

	metadataResponse := model.ClusterMetadataResponse{}
	require.NoError(t, json.Unmarshal(metadataJSON, &metadataResponse))
	return &metadataResponse
}
