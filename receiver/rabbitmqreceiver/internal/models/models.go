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
	DiskFree        int64 `json:"disk_free"`
	DiskFreeLimit   int64 `json:"disk_free_limit"`
	DiskFreeAlarm   bool  `json:"disk_free_alarm"`
	FDUsed          int64 `json:"fd_used"`
	FDTotal         int64 `json:"fd_total"`
	SocketsUsed     int64 `json:"sockets_used"`
	SocketsTotal    int64 `json:"sockets_total"`
	ProcUsed        int64 `json:"proc_used"`
	ProcTotal       int64 `json:"proc_total"`
	MemUsed         int64 `json:"mem_used"`
	MemUsedRate     int64 `json:"mem_used_details.rate"`
	MemLimit        int64 `json:"mem_limit"`
	MemAlarm        bool  `json:"mem_alarm"`
	FDUsedRate      int64 `json:"fd_used_details.rate"`
	SocketsUsedRate int64 `json:"sockets_used_details.rate"`
	ProcUsedRate    int64 `json:"proc_used_details.rate"`
	DiskFreeRate    int64 `json:"disk_free_details.rate"`
}
