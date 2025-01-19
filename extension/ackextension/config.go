// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ackextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/ackextension"
import (
	"go.opentelemetry.io/collector/component"
)

// Config defines configuration for ack extension
type Config struct {
	// StorageID defines the storage type of the extension. In-memory type is set by default (if not provided). Future consideration is disk type.
	StorageID *component.ID `mapstructure:"storage"`
	// MaxNumPartition Specifies the maximum number of partitions that clients can acquire for this extension instance.
	// Implementation defines how limit exceeding should be handled.
	MaxNumPartition uint64 `mapstructure:"max_number_of_partition"`
	// MaxNumPendingAcksPerPartition Specifies the maximum number of ackIDs and their corresponding status information that are waiting to be queried in each partition.
	MaxNumPendingAcksPerPartition uint64 `mapstructure:"max_number_of_pending_acks_per_partition"`
}
