// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkaexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter"

import (
	"fmt"

	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/twmb/franz-go/pkg/kgo"
	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter/internal/kafkaclient"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/kafka"
)

// RecordPartitionerExtension is implemented by extensions that supply a custom Kafka record
// partitioner for use with the kafka exporter.
type RecordPartitionerExtension interface {
	component.Component

	GetPartitioner() kgo.Partitioner
}

func buildPartitioner(cfg RecordPartitionerConfig, host component.Host) (kgo.Partitioner, error) {
	if cfg.StickyKey != nil {
		switch cfg.StickyKey.Hasher {
		case HasherSaramaCompat:
			return kgo.StickyKeyPartitioner(kafka.NewSaramaCompatHasher()), nil
		case HasherMurmur2:
			return kgo.StickyKeyPartitioner(nil), nil
		default:
			return nil, fmt.Errorf("unknown sticky key hasher type %q", cfg.StickyKey.Hasher)
		}
	}
	if cfg.RoundRobin != nil {
		return kgo.RoundRobinPartitioner(), nil
	}
	if cfg.LeastBackup != nil {
		return kgo.LeastBackupPartitioner(), nil
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
		return partExt.GetPartitioner(), nil
	}
	// in practice, this shouldn't happen.
	// The config validation should catch the case where no partitioner is set.
	return nil, errRecordPartitionerMissing
}

// partitionAwarePartitioner observes partition counts while preserving the
// configured partitioner's behavior for ordinary records. Records marked with
// a planned partition bypass rehashing so every key merged into that record
// remains on the partition for which the batch was built.
type partitionAwarePartitioner struct {
	partitioner     kgo.Partitioner
	partitionCounts *lru.Cache[string, int]
}

func newPartitionAwarePartitioner(partitioner kgo.Partitioner, topicCacheSize int) *partitionAwarePartitioner {
	partitionCounts, err := lru.New[string, int](topicCacheSize)
	if err != nil {
		// ponytail: Config.Validate guarantees a positive capacity.
		panic(err)
	}
	return &partitionAwarePartitioner{
		partitioner:     partitioner,
		partitionCounts: partitionCounts,
	}
}

func (p *partitionAwarePartitioner) ForTopic(topic string) kgo.TopicPartitioner {
	return &partitionAwareTopicPartitioner{
		topic:       topic,
		partitioner: p.partitioner.ForTopic(topic),
		counts:      p.partitionCounts,
	}
}

func (p *partitionAwarePartitioner) newTopicPlan(topic string) (*topicPartitionPlan, bool) {
	value, ok := p.partitionCounts.Get(topic)
	if !ok {
		return nil, false
	}
	return &topicPartitionPlan{
		partitioner:    p.partitioner.ForTopic(topic),
		partitionCount: value,
	}, true
}

type topicPartitionPlan struct {
	partitioner    kgo.TopicPartitioner
	partitionCount int
}

func (p *topicPartitionPlan) plan(key []byte) int {
	return p.partitioner.Partition(&kgo.Record{Key: key}, p.partitionCount)
}

type partitionAwareTopicPartitioner struct {
	topic       string
	partitioner kgo.TopicPartitioner
	counts      *lru.Cache[string, int]
}

func (p *partitionAwareTopicPartitioner) RequiresConsistency(record *kgo.Record) bool {
	return p.partitioner.RequiresConsistency(record)
}

func (p *partitionAwareTopicPartitioner) Partition(record *kgo.Record, partitionCount int) int {
	if partition, ok := kafkaclient.PlannedPartition(record.Context); ok {
		p.counts.Add(p.topic, partitionCount)
		if partition >= 0 && partition < partitionCount {
			return partition
		}
		return p.partitioner.Partition(record, partitionCount)
	}
	if p.partitioner.RequiresConsistency(record) {
		// For consistent keyed records franz-go supplies all partitions in
		// partition-number order. Unkeyed records may receive only currently
		// writable partitions, which is not a usable topic partition count.
		p.counts.Add(p.topic, partitionCount)
	}
	return p.partitioner.Partition(record, partitionCount)
}

func (p *partitionAwareTopicPartitioner) OnNewBatch() {
	if partitioner, ok := p.partitioner.(kgo.TopicPartitionerOnNewBatch); ok {
		partitioner.OnNewBatch()
	}
}
