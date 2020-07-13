package stats

// ClusterStatsOutput holds cluster level stats from Elasticsearch
type ClusterStatsOutput struct {
	ClusterName                 *string  `json:"cluster_name"`
	Status                      *string  `json:"status"`
	TimedOut                    *bool    `json:"timed_out"`
	NumberOfNodes               *int64   `json:"number_of_nodes"`
	NumberOfDataNodes           *int64   `json:"number_of_data_nodes"`
	ActivePrimaryShards         *int64   `json:"active_primary_shards"`
	ActiveShards                *int64   `json:"active_shards"`
	RelocatingShards            *int64   `json:"relocating_shards"`
	InitializingShards          *int64   `json:"initializing_shards"`
	UnassignedShards            *int64   `json:"unassigned_shards"`
	DelayedUnassignedShards     *int64   `json:"delayed_unassigned_shards"`
	NumberOfPendingTasks        *int64   `json:"number_of_pending_tasks"`
	NumberOfInFlightFetch       *int64   `json:"number_of_in_flight_fetch"`
	TaskMaxWaitingInQueueMillis *int64   `json:"task_max_waiting_in_queue_millis"`
	ActiveShardsPercentAsNumber *float64 `json:"active_shards_percent_as_number"`
}
