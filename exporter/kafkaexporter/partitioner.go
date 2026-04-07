// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkaexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter"

import (
	"fmt"

	"github.com/twmb/franz-go/pkg/kgo"
	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/kafka"
)

// RecordPartitionerExtension is implemented by extensions that supply a custom Kafka record
// partitioner for use with the kafka exporter.
type RecordPartitionerExtension interface {
	component.Component

	GetPartitioner() kgo.Partitioner
}

func buildPartitionerOpt(cfg RecordPartitionerConfig, host component.Host) (kgo.Opt, error) {
	if cfg.StickyKey != nil {
		switch cfg.StickyKey.Hasher {
		case HasherSaramaCompat:
			return kgo.RecordPartitioner(kgo.StickyKeyPartitioner(kafka.NewSaramaCompatHasher())), nil
		case HasherMurmur2:
			return kgo.RecordPartitioner(kgo.StickyKeyPartitioner(nil)), nil
		default:
			return nil, fmt.Errorf("unknown sticky key hasher type %q", cfg.StickyKey.Hasher)
		}
	}
	if cfg.RoundRobin != nil {
		return kgo.RecordPartitioner(kgo.RoundRobinPartitioner()), nil
	}
	if cfg.LeastBackup != nil {
		return kgo.RecordPartitioner(kgo.LeastBackupPartitioner()), nil
	}
	if cfg.Extension != nil {
		ext, ok := host.GetExtensions()[*cfg.Extension]
		if !ok {
			return nil, fmt.Errorf("partitioner extension %q not found", *cfg.Extension)
		}
		partExt, ok := ext.(RecordPartitionerExtension)
		if !ok {
			return nil, fmt.Errorf("extension %q does not implement RecordPartitionerExtension", *cfg.Extension)
		}
		return kgo.RecordPartitioner(partExt.GetPartitioner()), nil
	}
	// in practice, this shouldn't happen.
	// The config validation should catch the case where no partitioner is set.
	return nil, errRecordPartitionerMissing
}
