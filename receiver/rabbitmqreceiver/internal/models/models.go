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
	DiskFree int64 `json:"disk_free"`
	FDUsed   int64 `json:"fd_used"`
	MemLimit int64 `json:"mem_limit"`
	MemUsed  int64 `json:"mem_used"`
}
