package stats

import (
	"context"
	"errors"
	"fmt"
	"sync"

	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/util"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/utils"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumerdata"
	"go.opentelemetry.io/collector/consumer/pdatautil"
	"go.uber.org/zap"
)

type ESReceiver struct {
	logger *zap.Logger
	config *Config
	next   consumer.MetricsConsumer
	ctx    context.Context
	cancel context.CancelFunc
}

func NewESReceiver(
	logger *zap.Logger,
	config *Config,
	consumer consumer.MetricsConsumer,
) *ESReceiver {
	return &ESReceiver{
		logger: logger,
		config: config,
		next:   consumer,
	}
}

// Set up and kick off the interval runner.
func (e *ESReceiver) Start(ctx context.Context, host component.Host) error {
	httpClient, err := e.config.HTTPClientSettings.ToClient()
	if err != nil {
		return err
	}

	esClient := NewESClient(e.config.HTTPClientSettings.Endpoint, httpClient)

	e.ctx, e.cancel = context.WithCancel(ctx)

	var isInitialized bool

	// To handle metadata about metrics that is shared
	shared := sharedInfo{
		lock:   sync.RWMutex{},
		logger: e.logger,
	}

	util.RunOnInterval(e.ctx, func() {
		// Fetch metadata from Elasticsearch nodes
		err := shared.fetchNodeAndClusterMetadata(esClient, e.config.Cluster)

		// For any reason the monitor is not able to fetch cluster and node information upfront we capture that
		// to ensure that the stats are not sent in with faulty dimensions. The monitor will try to fetch this
		// information again every MetadataRefreshInterval seconds
		if err != nil {
			e.logger.Error("Failed to get Cluster and node metadata upfront", zap.Error(err))
			return
		}

		// The first time fetchNodeAndClusterMetadata succeeded the monitor can schedule fetchers for all the other stats
		if !isInitialized {
			// Collect Node level stats
			util.RunOnInterval(e.ctx, func() {
				_, defaultNodeMetricDimensions, _ := shared.getAllSharedInfo()

				e.fetchNodeStats(esClient, e.config, defaultNodeMetricDimensions)
			}, e.config.CollectionInterval)

			// Collect cluster level stats
			util.RunOnInterval(e.ctx, func() {
				defaultDimensions, _, nodeIsCurrentMaster := shared.getAllSharedInfo()

				if !*e.config.ClusterHealthStatsMasterOnly || nodeIsCurrentMaster {
					e.fetchClusterStats(esClient, e.config, defaultDimensions)
				}
			}, e.config.CollectionInterval)

			// Collect Index stats
			if *e.config.EnableIndexStats {
				util.RunOnInterval(e.ctx, func() {
					defaultDimensions, _, nodeIsCurrentMaster := shared.getAllSharedInfo()

					// if "IndexStatsMasterOnly" is true collect stats only from master,
					// otherwise collect index stats from all nodes
					if !*e.config.IndexStatsMasterOnly || nodeIsCurrentMaster {
						e.fetchIndexStats(esClient, e.config, defaultDimensions)
					}
				}, e.config.IndexStatsInterval)
			}
			isInitialized = true
		}
	}, e.config.MetadataRefreshInterval)

	return nil
}

func (e *ESReceiver) Shutdown(ctx context.Context) error {
	if e.cancel != nil {
		e.cancel()
	}
	return nil
}

func (e *ESReceiver) fetchNodeStats(esClient ESStatsHTTPClient, conf *Config, defaultNodeDimensions map[string]string) {
	nodeStatsOutput, err := esClient.GetNodeAndThreadPoolStats()
	if err != nil {
		e.logger.Error("Failed to GET node stats", zap.Error(err))
		return
	}

	e.sendMetrics(GetNodeStatsMetrics(nodeStatsOutput, defaultNodeDimensions, utils.StringSliceToMap(conf.ThreadPools), utils.StringSliceToMap(conf.EnableEnhancedNodeStatsForIndexGroups), map[string]bool{
		HTTPStatsGroup:       conf.EnableEnhancedHTTPStats,
		JVMStatsGroup:        conf.EnableEnhancedJVMStats,
		ProcessStatsGroup:    conf.EnableEnhancedProcessStats,
		ThreadpoolStatsGroup: conf.EnableEnhancedThreadPoolStats,
		TransportStatsGroup:  conf.EnableEnhancedTransportStats,
	}))
}

func (e *ESReceiver) fetchClusterStats(esClient ESStatsHTTPClient, conf *Config, defaultDimensions map[string]string) {
	clusterStatsOutput, err := esClient.GetClusterStats()

	if err != nil {
		e.logger.Error("Failed to GET cluster stats", zap.Error(err))
		return
	}

	e.sendMetrics(GetClusterStatsDatapoints(clusterStatsOutput, defaultDimensions, conf.EnableEnhancedClusterHealthStats))
}

func (e *ESReceiver) fetchIndexStats(esClient ESStatsHTTPClient, conf *Config, defaultDimensions map[string]string) {
	indexStatsOutput, err := esClient.GetIndexStats()

	if err != nil {
		e.logger.Error("Failed to GET index stats", zap.Error(err))
		return
	}

	if conf.IndexSummaryOnly {
		e.sendMetrics(GetIndexStatsSummaryDatapoints(indexStatsOutput.AllIndexStats, defaultDimensions, utils.StringSliceToMap(conf.EnableEnhancedIndexStatsForIndexGroups), conf.EnableIndexStatsPrimaries))
		return
	}

	e.sendMetrics(GetIndexStatsDatapoints(indexStatsOutput.Indices, utils.StringSliceToMap(conf.Indexes), defaultDimensions, utils.StringSliceToMap(conf.EnableEnhancedIndexStatsForIndexGroups), conf.EnableIndexStatsPrimaries))
}

// Prepares dimensions that are common to all datapoints from the monitor
func prepareDefaultDimensions(userProvidedClusterName string, queriedClusterName *string) (map[string]string, error) {
	dims := map[string]string{}
	clusterName := userProvidedClusterName

	if clusterName == "" {
		if queriedClusterName == nil {
			return nil, errors.New("failed to GET cluster name from Elasticsearch API")
		}
		clusterName = *queriedClusterName
	}

	dims["cluster"] = clusterName

	return dims, nil
}

func isCurrentMaster(nodeID string, masterInfoOutput *MasterInfoOutput) (bool, error) {
	masterNode := masterInfoOutput.MasterNode

	if masterNode == nil {
		return false, errors.New("unable to identify Elasticsearch cluster master node, assuming current node is not the current master")
	}

	return nodeID == *masterNode, nil
}

func prepareNodeMetricsDimensions(nodeInfo map[string]NodeInfo) (map[string]string, error) {
	var nodeID string

	if len(nodeInfo) != 1 {
		return nil, fmt.Errorf("expected info about exactly one node, received a map with %d entries", len(nodeInfo))
	}

	// nodes will have exactly one entry, for the current node since the monitor hits "_nodes/_local" endpoint
	for node := range nodeInfo {
		nodeID = node
	}

	if nodeID == "" {
		return nil, errors.New("failed to obtain Elasticsearch node id")
	}

	dims := map[string]string{}
	dims["node_id"] = nodeID

	nodeName := nodeInfo[nodeID].Name

	if nodeName != nil {
		dims["node_name"] = *nodeName
	}

	return dims, nil
}

func (e *ESReceiver) sendMetrics(ms []*metricspb.Metric) {
	// This is the filtering in place trick from https://github.com/golang/go/wiki/SliceTricks#filter-in-place
	n := 0
	for i := range ms {
		if ms[i] == nil {
			continue
		}
		ms[n] = ms[i]
		n++
	}

	e.next.ConsumeMetrics(e.ctx, pdatautil.MetricsFromMetricsData([]consumerdata.MetricsData{{Metrics: ms}}))
}
