// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package elasticsearchreceiver

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.opentelemetry.io/collector/scraper/scrapererror"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/elasticsearchreceiver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/elasticsearchreceiver/internal/mocks"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/elasticsearchreceiver/internal/model"
)

const (
	fullLinuxExpectedMetricsPath   = "./testdata/expected_metrics/full_linux.yaml"
	fullOtherExpectedMetricsPath   = "./testdata/expected_metrics/full_other.yaml"
	skipClusterExpectedMetricsPath = "./testdata/expected_metrics/clusterSkip.yaml"
	noNodesExpectedMetricsPath     = "./testdata/expected_metrics/noNodes.yaml"
)

func TestScraper(t *testing.T) {
	t.Parallel()

	config := createDefaultConfig().(*Config)

	config.MetricsBuilderConfig.Metrics.ElasticsearchNodeOperationsGetCompleted.Enabled = true
	config.MetricsBuilderConfig.Metrics.ElasticsearchNodeOperationsGetTime.Enabled = true
	config.MetricsBuilderConfig.Metrics.ElasticsearchNodeSegmentsMemory.Enabled = true

	config.MetricsBuilderConfig.Metrics.JvmMemoryHeapUtilization.Enabled = true

	config.MetricsBuilderConfig.Metrics.ElasticsearchNodeOperationsCurrent.Enabled = true

	config.MetricsBuilderConfig.Metrics.ElasticsearchIndexOperationsMergeSize.Enabled = true
	config.MetricsBuilderConfig.Metrics.ElasticsearchIndexOperationsMergeDocsCount.Enabled = true
	config.MetricsBuilderConfig.Metrics.ElasticsearchIndexOperationsMergeCurrent.Enabled = true
	config.MetricsBuilderConfig.Metrics.ElasticsearchIndexSegmentsCount.Enabled = true
	config.MetricsBuilderConfig.Metrics.ElasticsearchIndexSegmentsSize.Enabled = true
	config.MetricsBuilderConfig.Metrics.ElasticsearchIndexSegmentsMemory.Enabled = true
	config.MetricsBuilderConfig.Metrics.ElasticsearchIndexTranslogOperations.Enabled = true
	config.MetricsBuilderConfig.Metrics.ElasticsearchIndexTranslogSize.Enabled = true
	config.MetricsBuilderConfig.Metrics.ElasticsearchIndexCacheMemoryUsage.Enabled = true
	config.MetricsBuilderConfig.Metrics.ElasticsearchIndexCacheSize.Enabled = true
	config.MetricsBuilderConfig.Metrics.ElasticsearchIndexCacheEvictions.Enabled = true
	config.MetricsBuilderConfig.Metrics.ElasticsearchIndexDocuments.Enabled = true

	config.MetricsBuilderConfig.Metrics.ElasticsearchClusterIndicesCacheEvictions.Enabled = true

	config.MetricsBuilderConfig.Metrics.ElasticsearchNodeCacheSize.Enabled = true
	config.MetricsBuilderConfig.Metrics.ElasticsearchProcessCPUUsage.Enabled = true
	config.MetricsBuilderConfig.Metrics.ElasticsearchProcessCPUTime.Enabled = true
	config.MetricsBuilderConfig.Metrics.ElasticsearchProcessMemoryVirtual.Enabled = true

	sc := newElasticSearchScraper(receivertest.NewNopSettings(metadata.Type), config)

	err := sc.start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)

	mockClient := mocks.MockElasticsearchClient{}
	mockClient.On("ClusterMetadata", mock.Anything).Return(clusterMetadata(t), nil)
	mockClient.On("ClusterHealth", mock.Anything).Return(clusterHealth(t), nil)
	mockClient.On("ClusterStats", mock.Anything, []string{"_all"}).Return(clusterStats(t), nil)
	mockClient.On("Nodes", mock.Anything, []string{"_all"}).Return(nodes(t), nil)
	mockClient.On("NodeStats", mock.Anything, []string{"_all"}).Return(nodeStatsLinux(t), nil)
	mockClient.On("IndexStats", mock.Anything, []string{"_all"}).Return(indexStats(t), nil)

	sc.client = &mockClient

	expectedMetrics, err := golden.ReadMetrics(fullLinuxExpectedMetricsPath)
	require.NoError(t, err)

	actualMetrics, err := sc.scrape(t.Context())
	require.NoError(t, err)

	require.NoError(t, pmetrictest.CompareMetrics(expectedMetrics, actualMetrics, pmetrictest.IgnoreResourceMetricsOrder(),
		pmetrictest.IgnoreMetricDataPointsOrder(), pmetrictest.IgnoreStartTimestamp(), pmetrictest.IgnoreTimestamp()))
}

// TestScraperClusterUUID verifies that when the opt-in elasticsearch.cluster.uuid resource
// attribute is enabled, every emitted resource carries the cluster UUID from the metadata endpoint.
func TestScraperClusterUUID(t *testing.T) {
	t.Parallel()

	config := createDefaultConfig().(*Config)
	config.MetricsBuilderConfig.ResourceAttributes.ElasticsearchClusterUUID.Enabled = true

	sc := newElasticSearchScraper(receivertest.NewNopSettings(metadata.Type), config)
	require.NoError(t, sc.start(t.Context(), componenttest.NewNopHost()))

	mockClient := mocks.MockElasticsearchClient{}
	mockClient.On("ClusterMetadata", mock.Anything).Return(clusterMetadata(t), nil)
	mockClient.On("ClusterHealth", mock.Anything).Return(clusterHealth(t), nil)
	mockClient.On("ClusterStats", mock.Anything, []string{"_all"}).Return(clusterStats(t), nil)
	mockClient.On("Nodes", mock.Anything, []string{"_all"}).Return(nodes(t), nil)
	mockClient.On("NodeStats", mock.Anything, []string{"_all"}).Return(nodeStatsLinux(t), nil)
	mockClient.On("IndexStats", mock.Anything, []string{"_all"}).Return(indexStats(t), nil)
	sc.client = &mockClient

	expectedUUID := clusterMetadata(t).ClusterUUID
	require.NotEmpty(t, expectedUUID)

	actualMetrics, err := sc.scrape(t.Context())
	require.NoError(t, err)

	resourceMetrics := actualMetrics.ResourceMetrics()
	require.Positive(t, resourceMetrics.Len())
	for i := 0; i < resourceMetrics.Len(); i++ {
		uuid, ok := resourceMetrics.At(i).Resource().Attributes().Get("elasticsearch.cluster.uuid")
		require.True(t, ok, "elasticsearch.cluster.uuid attribute missing on resource %d", i)
		require.Equal(t, expectedUUID, uuid.Str())
	}
}

func TestScraperNoIOStats(t *testing.T) {
	t.Parallel()

	config := createDefaultConfig().(*Config)

	config.MetricsBuilderConfig.Metrics.ElasticsearchNodeOperationsGetCompleted.Enabled = true
	config.MetricsBuilderConfig.Metrics.ElasticsearchNodeOperationsGetTime.Enabled = true
	config.MetricsBuilderConfig.Metrics.ElasticsearchNodeSegmentsMemory.Enabled = true

	config.MetricsBuilderConfig.Metrics.JvmMemoryHeapUtilization.Enabled = true

	config.MetricsBuilderConfig.Metrics.ElasticsearchNodeOperationsCurrent.Enabled = true

	config.MetricsBuilderConfig.Metrics.ElasticsearchIndexOperationsMergeSize.Enabled = true
	config.MetricsBuilderConfig.Metrics.ElasticsearchIndexOperationsMergeDocsCount.Enabled = true
	config.MetricsBuilderConfig.Metrics.ElasticsearchIndexOperationsMergeCurrent.Enabled = true
	config.MetricsBuilderConfig.Metrics.ElasticsearchIndexSegmentsCount.Enabled = true
	config.MetricsBuilderConfig.Metrics.ElasticsearchIndexSegmentsSize.Enabled = true
	config.MetricsBuilderConfig.Metrics.ElasticsearchIndexSegmentsMemory.Enabled = true
	config.MetricsBuilderConfig.Metrics.ElasticsearchIndexTranslogOperations.Enabled = true
	config.MetricsBuilderConfig.Metrics.ElasticsearchIndexTranslogSize.Enabled = true
	config.MetricsBuilderConfig.Metrics.ElasticsearchIndexCacheMemoryUsage.Enabled = true
	config.MetricsBuilderConfig.Metrics.ElasticsearchIndexCacheSize.Enabled = true
	config.MetricsBuilderConfig.Metrics.ElasticsearchIndexCacheEvictions.Enabled = true
	config.MetricsBuilderConfig.Metrics.ElasticsearchIndexDocuments.Enabled = true

	config.MetricsBuilderConfig.Metrics.ElasticsearchClusterIndicesCacheEvictions.Enabled = true

	config.MetricsBuilderConfig.Metrics.ElasticsearchNodeCacheSize.Enabled = true
	config.MetricsBuilderConfig.Metrics.ElasticsearchProcessCPUUsage.Enabled = true
	config.MetricsBuilderConfig.Metrics.ElasticsearchProcessCPUTime.Enabled = true
	config.MetricsBuilderConfig.Metrics.ElasticsearchProcessMemoryVirtual.Enabled = true

	sc := newElasticSearchScraper(receivertest.NewNopSettings(metadata.Type), config)

	err := sc.start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)

	mockClient := mocks.MockElasticsearchClient{}
	mockClient.On("ClusterMetadata", mock.Anything).Return(clusterMetadata(t), nil)
	mockClient.On("ClusterHealth", mock.Anything).Return(clusterHealth(t), nil)
	mockClient.On("ClusterStats", mock.Anything, []string{"_all"}).Return(clusterStats(t), nil)
	mockClient.On("Nodes", mock.Anything, []string{"_all"}).Return(nodes(t), nil)
	mockClient.On("NodeStats", mock.Anything, []string{"_all"}).Return(nodeStatsOther(t), nil)
	mockClient.On("IndexStats", mock.Anything, []string{"_all"}).Return(indexStats(t), nil)

	sc.client = &mockClient

	expectedMetrics, err := golden.ReadMetrics(fullOtherExpectedMetricsPath)
	require.NoError(t, err)

	actualMetrics, err := sc.scrape(t.Context())
	require.NoError(t, err)

	require.NoError(t, pmetrictest.CompareMetrics(expectedMetrics, actualMetrics, pmetrictest.IgnoreResourceMetricsOrder(),
		pmetrictest.IgnoreMetricDataPointsOrder(), pmetrictest.IgnoreStartTimestamp(), pmetrictest.IgnoreTimestamp()))
}

func TestScraperSkipClusterMetrics(t *testing.T) {
	t.Parallel()

	conf := createDefaultConfig().(*Config)
	conf.SkipClusterMetrics = true

	sc := newElasticSearchScraper(receivertest.NewNopSettings(metadata.Type), conf)

	err := sc.start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)

	mockClient := mocks.MockElasticsearchClient{}
	mockClient.On("ClusterMetadata", mock.Anything).Return(clusterMetadata(t), nil)
	mockClient.On("ClusterHealth", mock.Anything).Return(clusterHealth(t), nil)
	mockClient.On("ClusterStats", mock.Anything, []string{}).Return(clusterStats(t), nil)
	mockClient.On("Nodes", mock.Anything, []string{"_all"}).Return(nodes(t), nil)
	mockClient.On("NodeStats", mock.Anything, []string{"_all"}).Return(nodeStatsLinux(t), nil)
	mockClient.On("IndexStats", mock.Anything, []string{"_all"}).Return(indexStats(t), nil)

	sc.client = &mockClient

	expectedMetrics, err := golden.ReadMetrics(skipClusterExpectedMetricsPath)
	require.NoError(t, err)

	actualMetrics, err := sc.scrape(t.Context())
	require.NoError(t, err)

	require.NoError(t, pmetrictest.CompareMetrics(expectedMetrics, actualMetrics, pmetrictest.IgnoreResourceMetricsOrder(),
		pmetrictest.IgnoreMetricDataPointsOrder(), pmetrictest.IgnoreStartTimestamp(), pmetrictest.IgnoreTimestamp()))
}

func TestScraperNoNodesMetrics(t *testing.T) {
	t.Parallel()

	conf := createDefaultConfig().(*Config)
	conf.Nodes = []string{}

	sc := newElasticSearchScraper(receivertest.NewNopSettings(metadata.Type), conf)

	err := sc.start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)

	mockClient := mocks.MockElasticsearchClient{}
	mockClient.On("ClusterMetadata", mock.Anything).Return(clusterMetadata(t), nil)
	mockClient.On("ClusterHealth", mock.Anything).Return(clusterHealth(t), nil)
	mockClient.On("ClusterStats", mock.Anything, []string{}).Return(clusterStats(t), nil)
	mockClient.On("Nodes", mock.Anything, []string{"_all"}).Return(nodes(t), nil)
	mockClient.On("NodeStats", mock.Anything, []string{}).Return(nodeStatsLinux(t), nil)
	mockClient.On("IndexStats", mock.Anything, []string{"_all"}).Return(indexStats(t), nil)

	sc.client = &mockClient

	expectedMetrics, err := golden.ReadMetrics(noNodesExpectedMetricsPath)
	require.NoError(t, err)

	actualMetrics, err := sc.scrape(t.Context())
	require.NoError(t, err)

	require.NoError(t, pmetrictest.CompareMetrics(expectedMetrics, actualMetrics, pmetrictest.IgnoreResourceMetricsOrder(),
		pmetrictest.IgnoreMetricDataPointsOrder(), pmetrictest.IgnoreStartTimestamp(), pmetrictest.IgnoreTimestamp()))
}

func TestScraperClusterStatsMasterOnly(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		desc          string
		localNodeID   string
		masterNodeID  string
		expectFetched bool
		expectPartial bool
	}{
		{
			desc:          "local node is master",
			localNodeID:   "szaFXm55RIeu8X-PTv5unQ",
			masterNodeID:  "szaFXm55RIeu8X-PTv5unQ",
			expectFetched: true,
		},
		{
			desc:          "local node is not master",
			localNodeID:   "szaFXm55RIeu8X-PTv5unQ",
			masterNodeID:  "someOtherNodeId",
			expectFetched: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			t.Parallel()

			conf := createDefaultConfig().(*Config)
			conf.ClusterStatsMasterOnly = true

			sc := newElasticSearchScraper(receivertest.NewNopSettings(metadata.Type), conf)
			err := sc.start(t.Context(), componenttest.NewNopHost())
			require.NoError(t, err)

			localNode := nodes(t)
			localNode.Nodes = map[string]model.NodeInfo{tc.localNodeID: localNode.Nodes["szaFXm55RIeu8X-PTv5unQ"]}

			mockClient := mocks.MockElasticsearchClient{}
			mockClient.On("ClusterMetadata", mock.Anything).Return(clusterMetadata(t), nil)
			mockClient.On("ClusterHealth", mock.Anything).Return(clusterHealth(t), nil)
			mockClient.On("Nodes", mock.Anything, []string{"_all"}).Return(nodes(t), nil)
			mockClient.On("Nodes", mock.Anything, []string{"_local"}).Return(localNode, nil)
			mockClient.On("MasterNode", mock.Anything).Return(&model.MasterNodeResponse{MasterNode: tc.masterNodeID}, nil)
			mockClient.On("NodeStats", mock.Anything, []string{"_all"}).Return(nodeStatsLinux(t), nil)
			mockClient.On("IndexStats", mock.Anything, []string{"_all"}).Return(indexStats(t), nil)
			if tc.expectFetched {
				mockClient.On("ClusterStats", mock.Anything, []string{"_all"}).Return(clusterStats(t), nil)
			}

			sc.client = &mockClient

			_, err = sc.scrape(t.Context())
			require.NoError(t, err)

			if tc.expectFetched {
				mockClient.AssertCalled(t, "ClusterStats", mock.Anything, []string{"_all"})
			} else {
				mockClient.AssertNotCalled(t, "ClusterStats", mock.Anything, mock.Anything)
			}
		})
	}
}

func TestScraperClusterStatsMasterOnlyDetectionError(t *testing.T) {
	t.Parallel()

	errBoom := errors.New("boom")

	conf := createDefaultConfig().(*Config)
	conf.ClusterStatsMasterOnly = true

	sc := newElasticSearchScraper(receivertest.NewNopSettings(metadata.Type), conf)
	err := sc.start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)

	mockClient := mocks.MockElasticsearchClient{}
	mockClient.On("ClusterMetadata", mock.Anything).Return(clusterMetadata(t), nil)
	mockClient.On("ClusterHealth", mock.Anything).Return(clusterHealth(t), nil)
	mockClient.On("Nodes", mock.Anything, []string{"_all"}).Return(nodes(t), nil)
	mockClient.On("Nodes", mock.Anything, []string{"_local"}).Return(nil, errBoom)
	mockClient.On("NodeStats", mock.Anything, []string{"_all"}).Return(nodeStatsLinux(t), nil)
	mockClient.On("IndexStats", mock.Anything, []string{"_all"}).Return(indexStats(t), nil)

	sc.client = &mockClient

	_, err = sc.scrape(t.Context())
	require.True(t, scrapererror.IsPartialScrapeError(err))
	require.ErrorContains(t, err, errBoom.Error())

	mockClient.AssertNotCalled(t, "ClusterStats", mock.Anything, mock.Anything)
}

func TestScraperClusterStatsMasterOnlyIgnoresNodeFilter(t *testing.T) {
	t.Parallel()

	conf := createDefaultConfig().(*Config)
	conf.Nodes = []string{"_local"}
	conf.ClusterStatsMasterOnly = true

	sc := newElasticSearchScraper(receivertest.NewNopSettings(metadata.Type), conf)
	err := sc.start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)

	localNodeID := "szaFXm55RIeu8X-PTv5unQ"
	localNode := nodes(t)
	localNode.Nodes = map[string]model.NodeInfo{localNodeID: localNode.Nodes[localNodeID]}

	mockClient := mocks.MockElasticsearchClient{}
	mockClient.On("ClusterMetadata", mock.Anything).Return(clusterMetadata(t), nil)
	mockClient.On("ClusterHealth", mock.Anything).Return(clusterHealth(t), nil)
	mockClient.On("Nodes", mock.Anything, []string{"_local"}).Return(localNode, nil)
	mockClient.On("MasterNode", mock.Anything).Return(&model.MasterNodeResponse{MasterNode: localNodeID}, nil)
	mockClient.On("NodeStats", mock.Anything, []string{"_local"}).Return(nodeStatsLinux(t), nil)
	mockClient.On("IndexStats", mock.Anything, []string{"_all"}).Return(indexStats(t), nil)
	mockClient.On("ClusterStats", mock.Anything, []string{"_all"}).Return(clusterStats(t), nil)

	sc.client = &mockClient

	_, err = sc.scrape(t.Context())
	require.NoError(t, err)

	// Even though the receiver is configured with nodes: ["_local"] (as recommended for
	// per-node NodeStats attribution), ClusterStats must still be fetched with "_all" so the
	// elected master reports true cluster-wide aggregates rather than narrowing to itself.
	mockClient.AssertCalled(t, "ClusterStats", mock.Anything, []string{"_all"})
	mockClient.AssertNotCalled(t, "ClusterStats", mock.Anything, []string{"_local"})
}

func TestScraperIndexStatsMasterOnly(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		desc          string
		localNodeID   string
		masterNodeID  string
		expectFetched bool
	}{
		{
			desc:          "local node is master",
			localNodeID:   "szaFXm55RIeu8X-PTv5unQ",
			masterNodeID:  "szaFXm55RIeu8X-PTv5unQ",
			expectFetched: true,
		},
		{
			desc:          "local node is not master",
			localNodeID:   "szaFXm55RIeu8X-PTv5unQ",
			masterNodeID:  "someOtherNodeId",
			expectFetched: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			t.Parallel()

			conf := createDefaultConfig().(*Config)
			conf.IndexStatsMasterOnly = true

			sc := newElasticSearchScraper(receivertest.NewNopSettings(metadata.Type), conf)
			err := sc.start(t.Context(), componenttest.NewNopHost())
			require.NoError(t, err)

			localNode := nodes(t)
			localNode.Nodes = map[string]model.NodeInfo{tc.localNodeID: localNode.Nodes["szaFXm55RIeu8X-PTv5unQ"]}

			mockClient := mocks.MockElasticsearchClient{}
			mockClient.On("ClusterMetadata", mock.Anything).Return(clusterMetadata(t), nil)
			mockClient.On("ClusterHealth", mock.Anything).Return(clusterHealth(t), nil)
			mockClient.On("ClusterStats", mock.Anything, []string{"_all"}).Return(clusterStats(t), nil)
			mockClient.On("Nodes", mock.Anything, []string{"_all"}).Return(nodes(t), nil)
			mockClient.On("Nodes", mock.Anything, []string{"_local"}).Return(localNode, nil)
			mockClient.On("MasterNode", mock.Anything).Return(&model.MasterNodeResponse{MasterNode: tc.masterNodeID}, nil)
			mockClient.On("NodeStats", mock.Anything, []string{"_all"}).Return(nodeStatsLinux(t), nil)
			if tc.expectFetched {
				mockClient.On("IndexStats", mock.Anything, []string{"_all"}).Return(indexStats(t), nil)
			}

			sc.client = &mockClient

			_, err = sc.scrape(t.Context())
			require.NoError(t, err)

			if tc.expectFetched {
				mockClient.AssertCalled(t, "IndexStats", mock.Anything, []string{"_all"})
			} else {
				mockClient.AssertNotCalled(t, "IndexStats", mock.Anything, mock.Anything)
			}
		})
	}
}

func TestScraperFailedStart(t *testing.T) {
	t.Parallel()

	conf := createDefaultConfig().(*Config)

	clientConfig := confighttp.NewDefaultClientConfig()
	// TODO: See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/49316.
	clientConfig.MaxIdleConns = 0    //nolint:staticcheck // SA1019: see TODO above
	clientConfig.IdleConnTimeout = 0 //nolint:staticcheck // SA1019: see TODO above
	clientConfig.ForceAttemptHTTP2 = false
	clientConfig.Endpoint = "localhost:9200"
	clientConfig.TLS = configtls.ClientConfig{
		Config: configtls.Config{
			CAFile: "/non/existent",
		},
	}
	conf.ClientConfig = clientConfig

	conf.Username = "dev"
	conf.Password = "dev"

	sc := newElasticSearchScraper(receivertest.NewNopSettings(metadata.Type), conf)

	err := sc.start(t.Context(), componenttest.NewNopHost())
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

				sc := newElasticSearchScraper(receivertest.NewNopSettings(metadata.Type), createDefaultConfig().(*Config))
				err := sc.start(t.Context(), componenttest.NewNopHost())
				require.NoError(t, err)

				sc.client = &mockClient

				_, err = sc.scrape(t.Context())
				require.True(t, scrapererror.IsPartialScrapeError(err))
				require.EqualError(t, err, err404.Error())
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
				mockClient.On("NodeStats", mock.Anything, []string{"_all"}).Return(nodeStatsLinux(t), nil)
				mockClient.On("ClusterHealth", mock.Anything).Return(nil, err404)
				mockClient.On("ClusterStats", mock.Anything, []string{"_all"}).Return(clusterStats(t), nil)
				mockClient.On("IndexStats", mock.Anything, []string{"_all"}).Return(indexStats(t), nil)

				sc := newElasticSearchScraper(receivertest.NewNopSettings(metadata.Type), createDefaultConfig().(*Config))
				err := sc.start(t.Context(), componenttest.NewNopHost())
				require.NoError(t, err)

				sc.client = &mockClient

				_, err = sc.scrape(t.Context())
				require.True(t, scrapererror.IsPartialScrapeError(err))
				require.EqualError(t, err, err404.Error())
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

				sc := newElasticSearchScraper(receivertest.NewNopSettings(metadata.Type), createDefaultConfig().(*Config))
				err := sc.start(t.Context(), componenttest.NewNopHost())
				require.NoError(t, err)

				sc.client = &mockClient

				m, err := sc.scrape(t.Context())
				require.ErrorContains(t, err, err404.Error())
				require.ErrorContains(t, err, err500.Error())

				require.Equal(t, 0, m.DataPointCount())
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
				mockClient.On("NodeStats", mock.Anything, []string{"_all"}).Return(nodeStatsLinux(t), nil)
				mockClient.On("ClusterHealth", mock.Anything).Return(clusterHealth(t), nil)
				mockClient.On("ClusterStats", mock.Anything, []string{"_all"}).Return(clusterStats(t), nil)
				mockClient.On("IndexStats", mock.Anything, []string{"_all"}).Return(indexStats(t), nil)

				sc := newElasticSearchScraper(receivertest.NewNopSettings(metadata.Type), createDefaultConfig().(*Config))
				err := sc.start(t.Context(), componenttest.NewNopHost())
				require.NoError(t, err)

				sc.client = &mockClient

				_, err = sc.scrape(t.Context())
				require.True(t, scrapererror.IsPartialScrapeError(err))
				require.ErrorContains(t, err, err404.Error())
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

				sc := newElasticSearchScraper(receivertest.NewNopSettings(metadata.Type), createDefaultConfig().(*Config))
				err := sc.start(t.Context(), componenttest.NewNopHost())
				require.NoError(t, err)

				sc.client = &mockClient

				m, err := sc.scrape(t.Context())
				require.ErrorContains(t, err, err404.Error())
				require.ErrorContains(t, err, err500.Error())

				require.Equal(t, 0, m.DataPointCount())
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
				mockClient.On("NodeStats", mock.Anything, []string{"_all"}).Return(nodeStatsLinux(t), nil)
				mockClient.On("ClusterHealth", mock.Anything).Return(ch, nil)
				mockClient.On("ClusterStats", mock.Anything, []string{"_all"}).Return(clusterStats(t), nil)
				mockClient.On("IndexStats", mock.Anything, []string{"_all"}).Return(indexStats(t), nil)

				sc := newElasticSearchScraper(receivertest.NewNopSettings(metadata.Type), createDefaultConfig().(*Config))
				err := sc.start(t.Context(), componenttest.NewNopHost())
				require.NoError(t, err)

				sc.client = &mockClient

				_, err = sc.scrape(t.Context())
				require.True(t, scrapererror.IsPartialScrapeError(err))
				require.ErrorContains(t, err, errUnknownClusterStatus.Error())
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.desc, testCase.run)
	}
}

func clusterHealth(t *testing.T) *model.ClusterHealth {
	clusterHealth := model.ClusterHealth{}
	require.NoError(t, json.Unmarshal(readSamplePayload(t, "health.json"), &clusterHealth))
	return &clusterHealth
}

func clusterStats(t *testing.T) *model.ClusterStats {
	clusterStats := model.ClusterStats{}
	require.NoError(t, json.Unmarshal(readSamplePayload(t, "cluster.json"), &clusterStats))
	return &clusterStats
}

func nodes(t *testing.T) *model.Nodes {
	nodes := model.Nodes{}
	require.NoError(t, json.Unmarshal(readSamplePayload(t, "nodes_linux.json"), &nodes))
	return &nodes
}

func nodeStatsLinux(t *testing.T) *model.NodeStats {
	nodeStats := model.NodeStats{}
	require.NoError(t, json.Unmarshal(readSamplePayload(t, "nodes_stats_linux.json"), &nodeStats))
	return &nodeStats
}

func nodeStatsOther(t *testing.T) *model.NodeStats {
	nodeStats := model.NodeStats{}
	require.NoError(t, json.Unmarshal(readSamplePayload(t, "nodes_stats_other.json"), &nodeStats))
	return &nodeStats
}

func indexStats(t *testing.T) *model.IndexStats {
	indexStats := model.IndexStats{}
	require.NoError(t, json.Unmarshal(readSamplePayload(t, "indices.json"), &indexStats))
	return &indexStats
}

func clusterMetadata(t *testing.T) *model.ClusterMetadataResponse {
	metadataResponse := model.ClusterMetadataResponse{}
	require.NoError(t, json.Unmarshal(readSamplePayload(t, "metadata.json"), &metadataResponse))
	return &metadataResponse
}
