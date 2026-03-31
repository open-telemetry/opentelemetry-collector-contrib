// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkaexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter"

import (
	"fmt"

	"github.com/twmb/franz-go/pkg/kgo"
	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/kafka"
)

const (
	// RecordPartitionerTypeSaramaCompatible is the default partitioner. It uses a sticky
	// key partitioner with Sarama-compatible FNV-1a hashing when a record key is set,
	// and a random sticky partition when no key is set.
	RecordPartitionerTypeSaramaCompatible = "sarama_compatible"

	// RecordPartitionerTypeRoundRobin distributes records evenly across all available
	// partitions in a round-robin fashion, regardless of the record key.
	RecordPartitionerTypeRoundRobin = "round_robin"

	// RecordPartitionerTypeLeastBackup routes each record to the partition with the fewest
	// buffered records, which can reduce produce latency under uneven load.
	RecordPartitionerTypeLeastBackup = "least_backup"

	// RecordPartitionerTypeCustom delegates partitioning to a user-provided extension
	// that implements RecordPartitionerExtension.
	RecordPartitionerTypeCustom = "custom"
)

// RecordPartitionerExtension is implemented by extensions that supply a custom Kafka record
// partitioner for use with the kafka exporter.
type RecordPartitionerExtension interface {
	component.Component

	GetPartitioner() kgo.Partitioner
}

func buildPartitionerOpt(cfg RecordPartitionerConfig, host component.Host) (kgo.Opt, error) {
	switch cfg.Type {
	case "", RecordPartitionerTypeSaramaCompatible:
		return kgo.RecordPartitioner(kafka.NewSaramaCompatPartitioner()), nil
	case RecordPartitionerTypeRoundRobin:
		return kgo.RecordPartitioner(kgo.RoundRobinPartitioner()), nil
	case RecordPartitionerTypeLeastBackup:
		return kgo.RecordPartitioner(kgo.LeastBackupPartitioner()), nil
	case RecordPartitionerTypeCustom:
		if cfg.Extension == nil {
			return nil, errRecordPartitionerExtRequired
		}
		ext, ok := host.GetExtensions()[*cfg.Extension]
		if !ok {
			return nil, fmt.Errorf("partitioner extension %q not found", cfg.Extension)
		}
		partExt, ok := ext.(RecordPartitionerExtension)
		if !ok {
			return nil, fmt.Errorf("extension %q does not implement RecordPartitionerExtension", cfg.Extension)
		}
		return kgo.RecordPartitioner(partExt.GetPartitioner()), nil
	default:
		return nil, fmt.Errorf("unknown partitioner type %q", cfg.Type)
	}
}
