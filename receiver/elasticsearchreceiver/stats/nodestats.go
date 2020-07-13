package stats

// NodeStatsOutput contains all the node level stats of the
// following types - indices, process, jvm, thread pool,
// transport and http, and will be used to retrieve node
// level metrics
type NodeStatsOutput struct {
	Nodes       nodes                `json:"_nodes"`
	ClusterName *string              `json:"cluster_name"`
	NodeStats   map[string]NodeStats `json:"nodes"`
}

type nodes struct {
	Total      *int64 `json:"total"`
	Successful *int64 `json:"successful"`
	Failed     *int64 `json:"failed"`
}

// NodeStats holds node level ES stats
type NodeStats struct {
	Timestamp        *int64                     `json:"timestamp"`
	Name             *string                    `json:"name"`
	TransportAddress *string                    `json:"transport_address"`
	Host             *string                    `json:"host"`
	Roles            *[]string                  `json:"roles"`
	Attributes       attributes                 `json:"attributes"`
	Indices          IndexStatsGroups           `json:"indices"`
	Process          Process                    `json:"process"`
	JVM              JVM                        `json:"jvm"`
	ThreadPool       map[string]ThreadPoolStats `json:"thread_pool"`
	Transport        Transport                  `json:"transport"`
	HTTP             HTTP                       `json:"http"`
}

// ThreadPoolStats holds a list of stats per thread_pool
type ThreadPoolStats struct {
	Threads   *int64 `json:"threads"`
	Queue     *int64 `json:"queue"`
	Active    *int64 `json:"active"`
	Rejected  *int64 `json:"rejected"`
	Largest   *int64 `json:"largest"`
	Completed *int64 `json:"completed"`
}

// Process stats
type Process struct {
	Timestamp           *int64          `json:"timestamp"`
	OpenFileDescriptors *int64          `json:"open_file_descriptors"`
	MaxFileDescriptors  *int64          `json:"max_file_descriptors"`
	CPU                 processCPUStats `json:"cpu"`
	Mem                 processMemStats `json:"mem"`
}

type processCPUStats struct {
	Percent       *int64 `json:"percent"`
	TotalInMillis *int64 `json:"total_in_millis"`
}

type processMemStats struct {
	TotalVirtualInBytes *int64 `json:"total_virtual_in_bytes"`
}

// JVM stats
type JVM struct {
	Timestamp       *int64          `json:"timestamp"`
	UptimeInMillis  *int64          `json:"uptime_in_millis"`
	JvmMemStats     jvmMemStats     `json:"mem"`
	JvmThreadsStats jvmThreadsStats `json:"threads"`
	JvmGcStats      jvmGcStats      `json:"gc"`
	BufferPools     bufferPools     `json:"buffer_pools"`
	Classes         classes         `json:"classes"`
}

type jvmMemStats struct {
	HeapUsedInBytes         *int64 `json:"heap_used_in_bytes"`
	HeapUsedPercent         *int64 `json:"heap_used_percent"`
	HeapCommittedInBytes    *int64 `json:"heap_committed_in_bytes"`
	HeapMaxInBytes          *int64 `json:"heap_max_in_bytes"`
	NonHeapUsedInBytes      *int64 `json:"non_heap_used_in_bytes"`
	NonHeapCommittedInBytes *int64 `json:"non_heap_committed_in_bytes"`
	Pools                   pools  `json:"pools"`
}

type jvmThreadsStats struct {
	Count     *int64 `json:"count"`
	PeakCount *int64 `json:"peak_count"`
}

type jvmGcStats struct {
	Collectors collectors `json:"collectors"`
}

type collectors struct {
	Young young `json:"young"`
	Old   old   `json:"old"`
}

type young struct {
	CollectionCount        *int64 `json:"collection_count"`
	CollectionTimeInMillis *int64 `json:"collection_time_in_millis"`
}

type old struct {
	CollectionCount        *int64 `json:"collection_count"`
	CollectionTimeInMillis *int64 `json:"collection_time_in_millis"`
}

type pools struct {
	Young    poolsYoungStats    `json:"young"`
	Survivor poolsSurvivorStats `json:"survivor"`
	Old      poolsOldStats      `json:"old"`
}

type poolsYoungStats struct {
	UsedInBytes     *int64 `json:"used_in_bytes"`
	MaxInBytes      *int64 `json:"max_in_bytes"`
	PeakUsedInBytes *int64 `json:"peak_used_in_bytes"`
	PeakMaxInBytes  *int64 `json:"peak_max_in_bytes"`
}

type poolsSurvivorStats struct {
	UsedInBytes     *int64 `json:"used_in_bytes"`
	MaxInBytes      *int64 `json:"max_in_bytes"`
	PeakUsedInBytes *int64 `json:"peak_used_in_bytes"`
	PeakMaxInBytes  *int64 `json:"peak_max_in_bytes"`
}

type poolsOldStats struct {
	UsedInBytes     *int64 `json:"used_in_bytes"`
	MaxInBytes      *int64 `json:"max_in_bytes"`
	PeakUsedInBytes *int64 `json:"peak_used_in_bytes"`
	PeakMaxInBytes  *int64 `json:"peak_max_in_bytes"`
}

type bufferPools struct {
	Mapped mapped `json:"mapped"`
	Direct direct `json:"direct"`
}

type mapped struct {
	Count                *int64 `json:"count"`
	UsedInBytes          *int64 `json:"used_in_bytes"`
	TotalCapacityInBytes *int64 `json:"total_capacity_in_bytes"`
}

type direct struct {
	Count                *int64 `json:"count"`
	UsedInBytes          *int64 `json:"used_in_bytes"`
	TotalCapacityInBytes *int64 `json:"total_capacity_in_bytes"`
}

type classes struct {
	CurrentLoadedCount *int64 `json:"current_loaded_count"`
	TotalLoadedCount   *int64 `json:"total_loaded_count"`
	TotalUnloadedCount *int64 `json:"total_unloaded_count"`
}

// Transport stats
type Transport struct {
	ServerOpen    *int64 `json:"server_open"`
	RxCount       *int64 `json:"rx_count"`
	RxSizeInBytes *int64 `json:"rx_size_in_bytes"`
	TxCount       *int64 `json:"tx_count"`
	TxSizeInBytes *int64 `json:"tx_size_in_bytes"`
}

// HTTP stats
type HTTP struct {
	CurrentOpen *int64 `json:"current_open"`
	TotalOpened *int64 `json:"total_opened"`
}

type attributes struct {
	MlMachineMemory *string `json:"ml.machine_memory"`
	XpackInstalled  *string `json:"xpack.installed"`
	MlMaxOpenJobs   *string `json:"ml.max_open_jobs"`
	MlEnabled       *string `json:"ml.enabled"`
}
