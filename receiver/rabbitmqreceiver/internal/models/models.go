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
	MessageStats map[string]interface{} `json:"message_stats"`
}
