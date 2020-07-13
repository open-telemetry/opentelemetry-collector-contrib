package stats

import (
	"time"

	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configmodels"
)

type Config struct {
	configmodels.ReceiverSettings `mapstructure:",squash"`

	confighttp.HTTPClientSettings `mapstructure:",squash"`

	// The duration between ElasticSearch metric fetches.
	CollectionInterval time.Duration `mapstructure:"collection_interval"`

	// Cluster name to which the node belongs. This is an optional config that
	// will override the cluster name fetched from a node and will be used to
	// populate the plugin_instance dimension
	Cluster string `mapstructure:"cluster"`
	// Enable Index stats. If set to true, by default the a subset of index
	// stats will be collected (see docs for list of default index metrics collected).
	EnableIndexStats *bool `mapstructure:"enable_index_stats" default:"true"`
	// Indexes to collect stats from (by default stats from all indexes are collected)
	Indexes []string `mapstructure:"indexes"`
	// Interval to report IndexStats on
	IndexStatsInterval time.Duration `mapstructure:"index_stats_interval" default:"60"`
	// Collect only aggregated index stats across all indexes
	IndexSummaryOnly bool `mapstructure:"index_summary_only"`
	// Collect index stats only from Master node
	IndexStatsMasterOnly *bool `mapstructure:"index_stats_master_only" default:"true"`
	// EnableClusterHealth enables reporting on the cluster health
	EnableClusterHealth *bool `mapstructure:"enable_cluster_health" default:"true"`
	// Whether or not non master nodes should report cluster health
	ClusterHealthStatsMasterOnly *bool `mapstructure:"cluster_health_stats_master_only" default:"true"`
	// Enable enhanced HTTP stats
	EnableEnhancedHTTPStats bool `mapstructure:"enable_enhanced_http_stats"`
	// Enable enhanced JVM stats
	EnableEnhancedJVMStats bool `mapstructure:"enable_enhanced_jvm_stats"`
	// Enable enhanced Process stats
	EnableEnhancedProcessStats bool `mapstructure:"enable_enhanced_process_stats"`
	// Enable enhanced ThreadPool stats
	EnableEnhancedThreadPoolStats bool `mapstructure:"enable_enhanced_thread_pool_stats"`
	// Enable enhanced Transport stats
	EnableEnhancedTransportStats bool `mapstructure:"enable_enhanced_transport_stats"`
	// Enable enhanced node level index stats groups. A list of index stats
	// groups for which to collect enhanced stats
	EnableEnhancedNodeStatsForIndexGroups []string `mapstructure:"enable_enhanced_node_indices_stats"`
	// ThreadPools to report threadpool node stats on
	ThreadPools []string `mapstructure:"thread_pools" default:"[\"search\", \"index\"]"`
	// Enable Cluster level stats. These stats report only from master Elasticserach nodes
	EnableEnhancedClusterHealthStats bool `mapstructure:"enable_enhanced_cluster_health_stats"`
	// Enable enhanced index level index stats groups. A list of index stats groups
	// for which to collect enhanced stats
	EnableEnhancedIndexStatsForIndexGroups []string `mapstructure:"enable_enhanced_index_stats_for_index_groups"`
	// To enable index stats from only primary shards. By default the index stats collected
	// are aggregated across all shards
	EnableIndexStatsPrimaries bool `mapstructure:"enable_index_stats_primaries"`
	// How often to refresh metadata about the node and cluster
	MetadataRefreshInterval time.Duration `mapstructure:"metadata_refresh_interval" default:"30"`
}

var trueVal = true

var DefaultConfig = Config{
	CollectionInterval:           10 * time.Second,
	MetadataRefreshInterval:      10 * time.Second,
	IndexStatsInterval:           10 * time.Second,
	EnableIndexStats:             &trueVal,
	IndexStatsMasterOnly:         &trueVal,
	EnableClusterHealth:          &trueVal,
	ClusterHealthStatsMasterOnly: &trueVal,
}
