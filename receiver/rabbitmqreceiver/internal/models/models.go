// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package models // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/rabbitmqreceiver/internal/models"

// Queue represents a queue in the API response
type Queue struct {
	// Identifiers
	Name  string `json:"name"`
	Node  string `json:"node"`
	VHost string `json:"vhost"`

	// Metrics
	Consumers              int64 `json:"consumers"`
	UnacknowledgedMessages int64 `json:"messages_unacknowledged"`
	ReadyMessages          int64 `json:"messages_ready"`

	// Embedded Metrics
	MessageStats map[string]any `json:"message_stats"`
}

// Node represents a RabbitMQ node in the API response
type Node struct {
	// Identifiers
	Name string `json:"name"`

	// Metrics
	DiskFree        int64   `json:"disk_free"`
	DiskFreeLimit   int64   `json:"disk_free_limit"`
	DiskFreeAlarm   bool    `json:"disk_free_alarm"`
	FDUsed          int64   `json:"fd_used"`
	FDTotal         int64   `json:"fd_total"`
	SocketsUsed     int64   `json:"sockets_used"`
	SocketsTotal    int64   `json:"sockets_total"`
	ProcUsed        int64   `json:"proc_used"`
	ProcTotal       int64   `json:"proc_total"`
	MemUsed         int64   `json:"mem_used"`
	MemUsedRate     float64 `json:"mem_used_details.rate"`
	MemLimit        int64   `json:"mem_limit"`
	MemAlarm        bool    `json:"mem_alarm"`
	FDUsedRate      float64 `json:"fd_used_details.rate"`
	SocketsUsedRate float64 `json:"sockets_used_details.rate"`
	ProcUsedRate    float64 `json:"proc_used_details.rate"`
	DiskFreeRate    float64 `json:"disk_free_details.rate"`

	// Additional system metrics
	Uptime              int64   `json:"uptime"`
	RunQueue            int64   `json:"run_queue"`
	Processors          int64   `json:"processors"`
	ContextSwitches     int64   `json:"context_switches"`
	ContextSwitchesRate float64 `json:"context_switches_details.rate"`

	// GC stats
	GCNum                int64   `json:"gc_num"`
	GCNumRate            float64 `json:"gc_num_details.rate"`
	GCBytesReclaimed     int64   `json:"gc_bytes_reclaimed"`
	GCBytesReclaimedRate float64 `json:"gc_bytes_reclaimed_details.rate"`

	// I/O stats
	IOReadCount       int64   `json:"io_read_count"`
	IOReadCountRate   float64 `json:"io_read_count_details.rate"`
	IOReadBytes       int64   `json:"io_read_bytes"`
	IOReadBytesRate   float64 `json:"io_read_bytes_details.rate"`
	IOReadAvgTime     float64 `json:"io_read_avg_time"`
	IOReadAvgTimeRate float64 `json:"io_read_avg_time_details.rate"`

	IOWriteCount       int64   `json:"io_write_count"`
	IOWriteCountRate   float64 `json:"io_write_count_details.rate"`
	IOWriteBytes       int64   `json:"io_write_bytes"`
	IOWriteBytesRate   float64 `json:"io_write_bytes_details.rate"`
	IOWriteAvgTime     float64 `json:"io_write_avg_time"`
	IOWriteAvgTimeRate float64 `json:"io_write_avg_time_details.rate"`

	IOSyncCount       int64   `json:"io_sync_count"`
	IOSyncCountRate   float64 `json:"io_sync_count_details.rate"`
	IOSyncAvgTime     float64 `json:"io_sync_avg_time"`
	IOSyncAvgTimeRate float64 `json:"io_sync_avg_time_details.rate"`

	IOSeekCount       int64   `json:"io_seek_count"`
	IOSeekCountRate   float64 `json:"io_seek_count_details.rate"`
	IOSeekAvgTime     float64 `json:"io_seek_avg_time"`
	IOSeekAvgTimeRate float64 `json:"io_seek_avg_time_details.rate"`

	IOReopenCount     int64   `json:"io_reopen_count"`
	IOReopenCountRate float64 `json:"io_reopen_count_details.rate"`

	// Mnesia transactions
	MnesiaRAMTxCount  int64   `json:"mnesia_ram_tx_count"`
	MnesiaRAMTxRate   float64 `json:"mnesia_ram_tx_count_details.rate"`
	MnesiaDiskTxCount int64   `json:"mnesia_disk_tx_count"`
	MnesiaDiskTxRate  float64 `json:"mnesia_disk_tx_count_details.rate"`

	// Message store I/O
	MsgStoreReadCount    int64   `json:"msg_store_read_count"`
	MsgStoreReadRate     float64 `json:"msg_store_read_count_details.rate"`
	MsgStoreWriteCount   int64   `json:"msg_store_write_count"`
	MsgStoreWriteRate    float64 `json:"msg_store_write_count_details.rate"`
	QueueIndexWriteCount int64   `json:"queue_index_write_count"`
	QueueIndexWriteRate  float64 `json:"queue_index_write_count_details.rate"`
	QueueIndexReadCount  int64   `json:"queue_index_read_count"`
	QueueIndexReadRate   float64 `json:"queue_index_read_count_details.rate"`

	// Connection/channel/queue stats
	ConnectionCreated     int64   `json:"connection_created"`
	ConnectionCreatedRate float64 `json:"connection_created_details.rate"`
	ConnectionClosed      int64   `json:"connection_closed"`
	ConnectionClosedRate  float64 `json:"connection_closed_details.rate"`

	ChannelCreated     int64   `json:"channel_created"`
	ChannelCreatedRate float64 `json:"channel_created_details.rate"`
	ChannelClosed      int64   `json:"channel_closed"`
	ChannelClosedRate  float64 `json:"channel_closed_details.rate"`

	QueueDeclared     int64   `json:"queue_declared"`
	QueueDeclaredRate float64 `json:"queue_declared_details.rate"`
	QueueCreated      int64   `json:"queue_created"`
	QueueCreatedRate  float64 `json:"queue_created_details.rate"`
	QueueDeleted      int64   `json:"queue_deleted"`
	QueueDeletedRate  float64 `json:"queue_deleted_details.rate"`
}
