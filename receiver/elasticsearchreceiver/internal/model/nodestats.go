// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package model // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/elasticsearchreceiver/internal/model"

// NodeStats represents a response from elasticsearch's /_nodes/stats endpoint.
// The struct is not exhaustive; It does not provide all values returned by elasticsearch,
// only the ones relevant to the metrics retrieved by the scraper.
type NodeStats struct {
	ClusterName string                        `json:"cluster_name"`
	Nodes       map[string]NodeStatsNodesInfo `json:"nodes"`
}

type NodeStatsNodesInfo struct {
	TimestampMsSinceEpoch int64                          `json:"timestamp"`
	Name                  string                         `json:"name"`
	Indices               NodeStatsNodesInfoIndices      `json:"indices"`
	ProcessStats          ProcessStats                   `json:"process"`
	JVMInfo               JVMInfo                        `json:"jvm"`
	ThreadPoolInfo        map[string]ThreadPoolStats     `json:"thread_pool"`
	TransportStats        TransportStats                 `json:"transport"`
	HTTPStats             HTTPStats                      `json:"http"`
	CircuitBreakerInfo    map[string]CircuitBreakerStats `json:"breakers"`
	FS                    FSStats                        `json:"fs"`
	OS                    OSStats                        `json:"os"`
	IndexingPressure      IndexingPressure               `json:"indexing_pressure"`
	Discovery             Discovery                      `json:"discovery"`
	Ingest                Ingest                         `json:"ingest"`
	Script                Script                         `json:"script"`
}

type Script struct {
	Compilations              int64 `json:"compilations"`
	CacheEvictions            int64 `json:"cache_evictions"`
	CompilationLimitTriggered int64 `json:"compilation_limit_triggered"`
}

type Ingest struct {
	Total     IngestTotalStats                    `json:"total"`
	Pipelines map[string]IngestPipelineTotalStats `json:"pipelines"`
}

type IngestTotalStats struct {
	Count        int64 `json:"count"`
	TimeInMillis int64 `json:"time_in_millis"`
	Current      int64 `json:"current"`
	Failed       int64 `json:"failed"`
}

type IngestPipelineTotalStats struct {
	IngestTotalStats
}

type Discovery struct {
	ClusterStateQueue       DiscoveryClusterStateQueue                       `json:"cluster_state_queue"`
	SerializedClusterStates map[string]DiscoverySerializedClusterStatesStats `json:"serialized_cluster_states"`
	PublishedClusterStates  DiscoveryPublishedClusterStates                  `json:"published_cluster_states"`
	ClusterStateUpdate      map[string]DiscoveryClusterStateUpdateStatsAll   `json:"cluster_state_update"`
}

type DiscoveryClusterStateQueue struct {
	Total     int64 `json:"total"`
	Pending   int64 `json:"pending"`
	Committed int64 `json:"committed"`
}

type DiscoverySerializedClusterStatesStats struct {
	Count                int64 `json:"count"`
	UncompressedSizeInBy int64 `json:"uncompressed_size_in_bytes"`
	CompressedSizeInBy   int64 `json:"compressed_size_in_bytes"`
}

type DiscoveryPublishedClusterStates struct {
	FullStates        int64 `json:"full_states"`
	IncompatibleDiffs int64 `json:"incompatible_diffs"`
	CompatibleDiffs   int64 `json:"compatible_diffs"`
}

type DiscoveryClusterStateUpdateStatsBase struct {
	Count                 int64 `json:"count"`
	ComputationTimeMillis int64 `json:"computation_time_millis"`
	PublicationTimeMillis int64 `json:"publication_time_millis"`
}

type DiscoveryClusterStateUpdateStatsAll struct {
	DiscoveryClusterStateUpdateStatsBase

	ContextConstructionTimeMillis int64 `json:"context_construction_time_millis"`
	CommitTimeMillis              int64 `json:"commit_time_millis"`
	CompletionTimeMillis          int64 `json:"completion_time_millis"`
	MasterApplyTimeMillis         int64 `json:"master_apply_time_millis"`
	NotificationTimeMillis        int64 `json:"notification_time_millis"`
}

type IndexingPressure struct {
	Memory IndexingPressureMemory `json:"memory"`
}

type IndexingPressureMemory struct {
	Current   IndexingPressureMemoryCurrentStats `json:"current"`
	Total     IndexingPressureMemoryTotalStats   `json:"total"`
	LimitInBy int64                              `json:"limit_in_bytes"`
}

type IndexingPressureMemoryCurrentStats struct {
	IndexingPressureMemoryStats
}

type IndexingPressureMemoryTotalStats struct {
	IndexingPressureMemoryStats
	PrimaryRejections int64 `json:"primary_rejections"`
	ReplicaRejections int64 `json:"replica_rejections"`
}

type IndexingPressureMemoryStats struct {
	CombinedCoordinatingAndPrimaryInBy int64 `json:"combined_coordinating_and_primary_in_bytes"`
	CoordinatingInBy                   int64 `json:"coordinating_in_bytes"`
	PrimaryInBy                        int64 `json:"primary_in_bytes"`
	ReplicaInBy                        int64 `json:"replica_in_bytes"`
	AllInBy                            int64 `json:"all_in_bytes"`
}

type OSStats struct {
	Timestamp int64          `json:"timestamp"`
	CPU       OSCPUStats     `json:"cpu"`
	Memory    OSCMemoryStats `json:"mem"`
}

type OSCPUStats struct {
	Usage   int64             `json:"percent"`
	LoadAvg OSCpuLoadAvgStats `json:"load_average"`
}

type OSCMemoryStats struct {
	TotalInBy int64 `json:"total_in_bytes"`
	FreeInBy  int64 `json:"free_in_bytes"`
	UsedInBy  int64 `json:"used_in_bytes"`
}

type OSCpuLoadAvgStats struct {
	OneMinute      float64 `json:"1m"`
	FiveMinutes    float64 `json:"5m"`
	FifteenMinutes float64 `json:"15m"`
}

type CircuitBreakerStats struct {
	LimitSizeInBytes     int64   `json:"limit_size_in_bytes"`
	LimitSize            string  `json:"limit_size"`
	EstimatedSizeInBytes int64   `json:"estimated_size_in_bytes"`
	EstimatedSize        string  `json:"estimated_size"`
	Overhead             float64 `json:"overhead"`
	Tripped              int64   `json:"tripped"`
}

type NodeStatsNodesInfoIndices struct {
	StoreInfo          StoreInfo           `json:"store"`
	DocumentStats      DocumentStats       `json:"docs"`
	IndexingOperations IndexingOperations  `json:"indexing"`
	GetOperation       GetOperation        `json:"get"`
	SearchOperations   SearchOperations    `json:"search"`
	MergeOperations    MergeOperations     `json:"merges"`
	RefreshOperations  BasicIndexOperation `json:"refresh"`
	FlushOperations    BasicIndexOperation `json:"flush"`
	WarmerOperations   BasicIndexOperation `json:"warmer"`
	QueryCache         BasicCacheInfo      `json:"query_cache"`
	FieldDataCache     BasicCacheInfo      `json:"fielddata"`
	TranslogStats      TranslogStats       `json:"translog"`
	RequestCacheStats  RequestCacheStats   `json:"request_cache"`
	SegmentsStats      SegmentsStats       `json:"segments"`
	SharedStats        SharedStats         `json:"shard_stats"`
	Mappings           MappingsStats       `json:"mappings"`
}

type SegmentsStats struct {
	Count                    int64 `json:"count"`
	DocumentValuesMemoryInBy int64 `json:"doc_values_memory_in_bytes"`
	IndexWriterMemoryInBy    int64 `json:"index_writer_memory_in_bytes"`
	MemoryInBy               int64 `json:"memory_in_bytes"`
	TermsMemoryInBy          int64 `json:"terms_memory_in_bytes"`
	FixedBitSetMemoryInBy    int64 `json:"fixed_bit_set_memory_in_bytes"`
}

type SharedStats struct {
	TotalCount int64 `json:"total_count"`
}

type MappingsStats struct {
	TotalCount                 int64 `json:"total_count"`
	TotalEstimatedOverheadInBy int64 `json:"total_estimated_overhead_in_bytes"`
}

type TranslogStats struct {
	Operations                int64 `json:"operations"`
	SizeInBy                  int64 `json:"size_in_bytes"`
	UncommittedOperationsInBy int64 `json:"uncommitted_size_in_bytes"`
}

type RequestCacheStats struct {
	MemorySizeInBy int64 `json:"memory_size_in_bytes"`
	Evictions      int64 `json:"evictions"`
	HitCount       int64 `json:"hit_count"`
	MissCount      int64 `json:"miss_count"`
}

type StoreInfo struct {
	SizeInBy        int64 `json:"size_in_bytes"`
	DataSetSizeInBy int64 `json:"total_data_set_size_in_bytes"`
	ReservedInBy    int64 `json:"reserved_in_bytes"`
}

type BasicIndexOperation struct {
	Total         int64 `json:"total"`
	TotalTimeInMs int64 `json:"total_time_in_millis"`
}

type MergeOperations struct {
	BasicIndexOperation
	TotalSizeInBytes int64 `json:"total_size_in_bytes"`
	TotalDocs        int64 `json:"total_docs"`
}

type IndexingOperations struct {
	IndexTotal     int64 `json:"index_total"`
	IndexTimeInMs  int64 `json:"index_time_in_millis"`
	DeleteTotal    int64 `json:"delete_total"`
	DeleteTimeInMs int64 `json:"delete_time_in_millis"`
}

type GetOperation struct {
	Total           int64 `json:"total"`
	TotalTimeInMs   int64 `json:"time_in_millis"`
	Exists          int64 `json:"exists_total"`
	ExistsTimeInMs  int64 `json:"exists_time_in_millis"`
	Missing         int64 `json:"missing_total"`
	MissingTimeInMs int64 `json:"missing_time_in_millis"`
}

type SearchOperations struct {
	QueryCurrent    int64 `json:"query_current"`
	QueryTotal      int64 `json:"query_total"`
	QueryTimeInMs   int64 `json:"query_time_in_millis"`
	FetchTotal      int64 `json:"fetch_total"`
	FetchTimeInMs   int64 `json:"fetch_time_in_millis"`
	ScrollTotal     int64 `json:"scroll_total"`
	ScrollTimeInMs  int64 `json:"scroll_time_in_millis"`
	SuggestTotal    int64 `json:"suggest_total"`
	SuggestTimeInMs int64 `json:"suggest_time_in_millis"`
}

type DocumentStats struct {
	ActiveCount  int64 `json:"count"`
	DeletedCount int64 `json:"deleted"`
}

type BasicCacheInfo struct {
	TotalCount     int64 `json:"total_count"`
	HitCount       int64 `json:"hit_count"`
	MissCount      int64 `json:"miss_count"`
	Evictions      int64 `json:"evictions"`
	CacheSize      int64 `json:"cache_size"`
	CacheCount     int64 `json:"cache_count"`
	MemorySizeInBy int64 `json:"memory_size_in_bytes"`
	MemorySize     int64 `json:"memory_size"`
}

type JVMInfo struct {
	UptimeInMs    int64         `json:"uptime_in_millis"`
	JVMMemoryInfo JVMMemoryInfo `json:"mem"`
	JVMThreadInfo JVMThreadInfo `json:"threads"`
	JVMGCInfo     JVMGCInfo     `json:"gc"`
	ClassInfo     JVMClassInfo  `json:"classes"`
}

type JVMMemoryInfo struct {
	HeapUsedInBy        int64          `json:"heap_used_in_bytes"`
	NonHeapUsedInBy     int64          `json:"non_heap_used_in_bytes"`
	MaxHeapInBy         int64          `json:"heap_max_in_bytes"`
	HeapCommittedInBy   int64          `json:"heap_committed_in_bytes"`
	HeapUsedPercent     int64          `json:"heap_used_percent"`
	NonHeapComittedInBy int64          `json:"non_heap_committed_in_bytes"`
	MemoryPools         JVMMemoryPools `json:"pools"`
}

type JVMMemoryPools struct {
	Young    JVMMemoryPoolInfo `json:"young"`
	Survivor JVMMemoryPoolInfo `json:"survivor"`
	Old      JVMMemoryPoolInfo `json:"old"`
}

type JVMMemoryPoolInfo struct {
	MemUsedBy int64 `json:"used_in_bytes"`
	MemMaxBy  int64 `json:"max_in_bytes"`
}

type JVMThreadInfo struct {
	PeakCount int64 `json:"peak_count"`
	Count     int64 `json:"count"`
}

type JVMGCInfo struct {
	Collectors JVMCollectors `json:"collectors"`
}

type JVMCollectors struct {
	Young BasicJVMCollectorInfo `json:"young"`
	Old   BasicJVMCollectorInfo `json:"old"`
}

type BasicJVMCollectorInfo struct {
	CollectionCount        int64 `json:"collection_count"`
	CollectionTimeInMillis int64 `json:"collection_time_in_millis"`
}

type JVMClassInfo struct {
	CurrentLoadedCount int64 `json:"current_loaded_count"`
}

type ThreadPoolStats struct {
	TotalThreads   int64 `json:"threads"`
	ActiveThreads  int64 `json:"active"`
	QueuedTasks    int64 `json:"queue"`
	CompletedTasks int64 `json:"completed"`
	RejectedTasks  int64 `json:"rejected"`
}

type ProcessStats struct {
	OpenFileDescriptorsCount int64              `json:"open_file_descriptors"`
	MaxFileDescriptorsCount  int64              `json:"max_file_descriptors_count"`
	CPU                      ProcessCPUStats    `json:"cpu"`
	Memory                   ProcessMemoryStats `json:"mem"`
}

type ProcessCPUStats struct {
	Percent   int64 `json:"percent"`
	TotalInMs int64 `json:"total_in_millis"`
}

type ProcessMemoryStats struct {
	TotalVirtual     int64 `json:"total_virtual"`
	TotalVirtualInBy int64 `json:"total_virtual_in_bytes"`
}

type TransportStats struct {
	OpenConnections int64 `json:"server_open"`
	ReceivedBytes   int64 `json:"rx_size_in_bytes"`
	SentBytes       int64 `json:"tx_size_in_bytes"`
}

type HTTPStats struct {
	OpenConnections int64 `json:"current_open"`
}

type FSStats struct {
	Total   FSTotalStats `json:"total"`
	IOStats *IOStats     `json:"io_stats,omitempty"`
}

type FSTotalStats struct {
	AvailableBytes int64 `json:"available_in_bytes"`
	TotalBytes     int64 `json:"total_in_bytes"`
	FreeBytes      int64 `json:"free_in_bytes"`
}

type IOStats struct {
	Total IOStatsTotal `json:"total"`
}

type IOStatsTotal struct {
	Operations      int64 `json:"operations"`
	ReadOperations  int64 `json:"read_operations"`
	WriteOperations int64 `json:"write_operations"`
	ReadBytes       int64 `json:"read_kilobytes"`
	WriteBytes      int64 `json:"write_kilobytes"`
}
