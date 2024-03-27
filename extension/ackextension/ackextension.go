// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ackextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/ackextension"

// AckExtension is an extension that can be used by other otel components to support acking of data and can be queried against
// to check the status of given ack ids.
type AckExtension interface {
	// ProcessEvent marks the beginning of processing an event. It generates an ack ID for the associated partition ID.
	// ACK IDs are only unique within a partition. Two partitions can have the same ACK IDs, but they are generated for different events.
	ProcessEvent(partitionID string) (ackID uint64)

	// Ack acknowledges an event has been processed.
	Ack(partitionID string, ackID uint64)

	// QueryAcks checks the statuses of given ackIDs for a partition.
	// ackIDs that are not generated from ProcessEvent or have been removed as a result of previous calls to QueryAcks will return false.
	QueryAcks(partitionID string, ackIDs []uint64) map[uint64]bool
}
