// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkaexporter

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kfake"
	"github.com/twmb/franz-go/pkg/kgo"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/kafka"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/kafka/kafkatest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/kafka/configkafka"
)

type mockPartitionerExtension struct {
	partitioner kgo.Partitioner
}

func (*mockPartitionerExtension) Start(context.Context, component.Host) error { return nil }
func (*mockPartitionerExtension) Shutdown(context.Context) error              { return nil }
func (m *mockPartitionerExtension) GetPartitioner() kgo.Partitioner           { return m.partitioner }

type notAPartitionerExtension struct{}

func (*notAPartitionerExtension) Start(context.Context, component.Host) error { return nil }
func (*notAPartitionerExtension) Shutdown(context.Context) error              { return nil }

type mockHostWithExtensions struct {
	component.Host
	extensions map[component.ID]component.Component
}

func (h *mockHostWithExtensions) GetExtensions() map[component.ID]component.Component {
	return h.extensions
}

func TestRecordPartitionerConfig_Validate(t *testing.T) {
	extID := component.MustNewID("my_partitioner")
	unknownExtID := component.MustNewID("unknown")

	tests := []struct {
		name    string
		cfg     RecordPartitionerConfig
		wantErr string
	}{
		{
			name: "sarama_compat",
			cfg: RecordPartitionerConfig{StickyKey: &StickyKeyPartitionerConfig{
				Hasher: "sarama_compat",
			}},
		},
		{
			name: "round_robin",
			cfg:  RecordPartitionerConfig{RoundRobin: &struct{}{}},
		},
		{
			name: "least_backup",
			cfg:  RecordPartitionerConfig{LeastBackup: &struct{}{}},
		},
		{
			name: "extension with ID",
			cfg:  RecordPartitionerConfig{Extension: &extID},
		},
		{
			name: "unknown extension",
			cfg:  RecordPartitionerConfig{Extension: &unknownExtID},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.wantErr == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				require.ErrorContains(t, err, tt.wantErr)
			}
		})
	}
}

func TestBuildPartitionerOpt(t *testing.T) {
	extID := component.MustNewID("my_partitioner")
	customPartitioner := kgo.RoundRobinPartitioner()
	ext := &mockPartitionerExtension{partitioner: customPartitioner}

	hostWithExt := &mockHostWithExtensions{
		Host: componenttest.NewNopHost(),
		extensions: map[component.ID]component.Component{
			extID: ext,
		},
	}
	compID := component.MustNewID("missing")

	tests := []struct {
		name    string
		cfg     RecordPartitionerConfig
		host    component.Host
		wantErr string
	}{
		{
			name: "sarama_compat",
			cfg: RecordPartitionerConfig{StickyKey: &StickyKeyPartitionerConfig{
				Hasher: HasherSaramaCompat,
			}},
			host: componenttest.NewNopHost(),
		},
		{
			name: "round_robin",
			cfg:  RecordPartitionerConfig{RoundRobin: &struct{}{}},
			host: componenttest.NewNopHost(),
		},
		{
			name: "least_backup",
			cfg:  RecordPartitionerConfig{LeastBackup: &struct{}{}},
			host: componenttest.NewNopHost(),
		},
		{
			name: "extension",
			cfg:  RecordPartitionerConfig{Extension: &extID},
			host: hostWithExt,
		},
		{
			name:    "extension not found",
			cfg:     RecordPartitionerConfig{Extension: &compID},
			host:    componenttest.NewNopHost(),
			wantErr: `partitioner extension "missing" not found`,
		},
		{
			name: "extension does not implement RecordPartitionerExtension",
			cfg:  RecordPartitionerConfig{Extension: &extID},
			host: &mockHostWithExtensions{
				Host: componenttest.NewNopHost(),
				extensions: map[component.ID]component.Component{
					extID: &notAPartitionerExtension{},
				},
			},
			wantErr: `does not implement RecordPartitionerExtension`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opt, err := buildPartitionerOpt(tt.cfg, tt.host)
			if tt.wantErr == "" {
				require.NoError(t, err)
				require.NotNil(t, opt)
			} else {
				require.Error(t, err)
				require.ErrorContains(t, err, tt.wantErr)
			}
		})
	}
}

// newPartitioningProducer creates a kgo producer backed by a mock cluster with
// numPartitions partitions for topic.
func newPartitioningProducer(
	t *testing.T,
	cfg RecordPartitionerConfig,
	host component.Host,
	numPartitions int,
	topic string,
) (*kgo.Client, []string) {
	t.Helper()
	cluster, kcfg := kafkatest.NewCluster(t, kfake.SeedTopics(int32(numPartitions), topic))

	partOpt, err := buildPartitionerOpt(cfg, host)
	require.NoError(t, err)

	producerCfg := configkafka.NewDefaultProducerConfig()
	producerCfg.Linger = 0 // disable linger so each ProduceSync is its own batch
	client, err := kafka.NewFranzSyncProducer(
		t.Context(), host, kcfg, producerCfg,
		5*time.Second, zap.NewNop(),
		partOpt,
	)
	require.NoError(t, err)
	t.Cleanup(client.Close)
	return client, cluster.ListenAddrs()
}

func produceAndFetch(t *testing.T, client *kgo.Client, brokers []string, topic string, keys [][]byte) []*kgo.Record {
	t.Helper()
	for _, key := range keys {
		r := &kgo.Record{Topic: topic, Value: []byte("value")}
		if key != nil {
			r.Key = key
		}
		require.NoError(t, client.ProduceSync(t.Context(), r).FirstErr())
	}

	clientOpts := []kgo.Opt{
		kgo.SeedBrokers(brokers...),
		kgo.ConsumeTopics(topic),
		kgo.ConsumerGroup("group-id-" + topic),
	}
	consumer, err := kgo.NewClient(clientOpts...)
	require.NoError(t, err)
	defer consumer.Close()

	want := len(keys)
	var records []*kgo.Record
	for len(records) < want {
		fetches := consumer.PollRecords(t.Context(), want-len(records))
		require.NoError(t, fetches.Err())
		fetches.EachRecord(func(r *kgo.Record) {
			records = append(records, r)
		})
	}
	return records
}

// partitionSet collects the unique partition numbers from a slice of records.
func partitionSet(records []*kgo.Record) map[int32]struct{} {
	m := make(map[int32]struct{}, len(records))
	for _, r := range records {
		m[r.Partition] = struct{}{}
	}
	return m
}

func TestRecordPartitioner_RoundRobin(t *testing.T) {
	const numPartitions = 3
	const topic = "rr-topic"

	client, brokers := newPartitioningProducer(t,
		RecordPartitionerConfig{RoundRobin: &struct{}{}},
		componenttest.NewNopHost(), numPartitions, topic,
	)

	// Send one record per partition, keyless, each in its own ProduceSync.
	nils := make([][]byte, numPartitions)
	records := produceAndFetch(t, client, brokers, topic, nils)

	require.Len(t, records, numPartitions)
	partitions := partitionSet(records)
	require.Len(t, partitions, numPartitions,
		"round-robin should spread %d records across all %d partitions", numPartitions, numPartitions)
}

func TestRecordPartitioner_SaramaCompatible_SameKeyConsistency(t *testing.T) {
	const numPartitions = 4
	const topic = "sarama-key-topic"
	const numRecords = 6

	client, brokers := newPartitioningProducer(t,
		RecordPartitionerConfig{
			StickyKey: &StickyKeyPartitionerConfig{
				Hasher: HasherSaramaCompat,
			},
		},
		componenttest.NewNopHost(), numPartitions, topic,
	)

	key := []byte("stable-key")
	keys := make([][]byte, numRecords)
	for i := range keys {
		keys[i] = key
	}
	records := produceAndFetch(t, client, brokers, topic, keys)

	require.Len(t, records, numRecords)
	partitions := partitionSet(records)
	require.Len(t, partitions, 1,
		"all records with the same key should land on the same partition")
}

func TestRecordPartitioner_SaramaCompatible_Murmur2(t *testing.T) {
	const numPartitions = 4
	const topic = "sarama-key-topic"
	const numRecords = 6

	client, brokers := newPartitioningProducer(t,
		RecordPartitionerConfig{
			StickyKey: &StickyKeyPartitionerConfig{
				Hasher: HasherMurmur2,
			},
		},
		componenttest.NewNopHost(), numPartitions, topic,
	)

	key := []byte("stable-key")
	keys := make([][]byte, numRecords)
	for i := range keys {
		keys[i] = key
	}
	records := produceAndFetch(t, client, brokers, topic, keys)

	require.Len(t, records, numRecords)
	partitions := partitionSet(records)
	require.Len(t, partitions, 1,
		"all records with the same key should land on the same partition")
}

func TestRecordPartitioner_SaramaCompatible_DifferentKeys(t *testing.T) {
	const numPartitions = 8
	const topic = "sarama-diff-keys-topic"

	client, brokers := newPartitioningProducer(t,
		RecordPartitionerConfig{StickyKey: &StickyKeyPartitionerConfig{
			Hasher: HasherSaramaCompat,
		}},
		componenttest.NewNopHost(), numPartitions, topic,
	)

	// These two keys are known to hash to different FNV-1a buckets mod 8.
	keys := [][]byte{[]byte("key-alpha"), []byte("key-beta")}
	records := produceAndFetch(t, client, brokers, topic, keys)

	require.Len(t, records, 2)
	require.NotEqual(t, records[0].Partition, records[1].Partition,
		"distinct keys should land on distinct partitions")
}

func TestRecordPartitioner_LeastBackup(t *testing.T) {
	const numPartitions = 3
	const topic = "lb-topic"

	// LeastBackup picks the partition with the smallest producer buffer.
	// If multiple partitions are tied, franz-go randomly chooses one of them.
	//
	// When linger=0 and we send one record at a time (ProduceSync),
	// each send completes before the next partition is picked. That keeps
	// all buffers roughly equal, so each record is effectively sent to a
	// random partition.
	//
	// due to this randomness, a small number of messages might still all land on the same partition.
	// that's why we use large number of records.
	const numRecords = 50

	client, brokers := newPartitioningProducer(t,
		RecordPartitionerConfig{LeastBackup: &struct{}{}},
		componenttest.NewNopHost(), numPartitions, topic,
	)

	records := produceAndFetch(t, client, brokers, topic, make([][]byte, numRecords))
	require.Len(t, records, numRecords)

	partitions := partitionSet(records)
	require.Len(t, partitions, numPartitions)
}

func TestRecordPartitioner_Extension_CustomRouting(t *testing.T) {
	const numPartitions = 4
	const topic = "ext-partition-topic"
	const numRecords = 6

	// alwaysZeroPartitioner routes every record to partition 0.
	alwaysZero := kgo.BasicConsistentPartitioner(func(string) func(*kgo.Record, int) int {
		return func(_ *kgo.Record, _ int) int { return 0 }
	})

	extID := component.MustNewID("always_zero")
	host := &mockHostWithExtensions{
		Host: componenttest.NewNopHost(),
		extensions: map[component.ID]component.Component{
			extID: &mockPartitionerExtension{partitioner: alwaysZero},
		},
	}

	client, brokers := newPartitioningProducer(t,
		RecordPartitionerConfig{Extension: &extID},
		host, numPartitions, topic,
	)

	// Mix of keyed and keyless records; the custom partitioner should ignore keys.
	keys := [][]byte{nil, []byte("k1"), nil, []byte("k2"), nil, []byte("k3")}
	records := produceAndFetch(t, client, brokers, topic, keys)

	require.Len(t, records, numRecords)
	for _, r := range records {
		require.Equal(t, int32(0), r.Partition,
			"extension partitioner (always-zero) should route all records to partition 0")
	}
}
