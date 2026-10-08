// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package partitioningprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/partitioningprocessor"

import (
	"context"
	"errors"
	"maps"
	"sync"

	"go.opentelemetry.io/collector/client"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

// partitioningProcessor is the consumer wrapped by processorhelper; it
// splits each batch and fans the partitions out to the next consumer.
type partitioningProcessor struct {
	nextLogs          consumer.Logs
	logsPartitioner   logsPartitioner
	nextTraces        consumer.Traces
	tracesPartitioner tracesPartitioner

	// keyNames holds the lower-cased partition key names in sorted order,
	// matching the order of values produced by each partitioner.
	keyNames []string
}

// Capabilities reports MutatesData: true because, when a batch is split,
// items are moved (not copied) out of the input into the partitions.
func (*partitioningProcessor) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: true}
}

func (p *partitioningProcessor) ConsumeLogs(ctx context.Context, ld plog.Logs) error {
	parts, err := p.logsPartitioner.partitionLogs(ctx, ld)
	if err != nil {
		// Key evaluation depends only on the data, so a retry would fail the same way.
		return consumererror.NewPermanent(err)
	}
	return consumePartitions(ctx, p.keyNames, parts, p.nextLogs.ConsumeLogs)
}

func (p *partitioningProcessor) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	parts, err := p.tracesPartitioner.partitionTraces(ctx, td)
	if err != nil {
		// Key evaluation depends only on the data, so a retry would fail the same way.
		return consumererror.NewPermanent(err)
	}
	return consumePartitions(ctx, p.keyNames, parts, p.nextTraces.ConsumeTraces)
}

// consumePartitions forwards all partitions concurrently.
//
// Every partition is delivered even if a sibling fails; all errors are
// joined. The parent context is passed through unchanged (no derived
// cancellation), so downstream consumers that retain the context after
// returning do not observe a spurious cancellation.
func consumePartitions[T any](ctx context.Context, keyNames []string, parts []partitioned[T], consume func(context.Context, T) error) error {
	if len(parts) == 0 {
		return nil
	}
	info := client.FromContext(ctx)
	base := baseMetadata(info.Metadata, keyNames)
	if len(parts) == 1 {
		return consume(withPartitionMetadata(ctx, info, base, keyNames, parts[0].values), parts[0].data)
	}

	var wg sync.WaitGroup
	errs := make([]error, len(parts))
	for i, part := range parts {
		wg.Go(func() {
			errs[i] = consume(withPartitionMetadata(ctx, info, base, keyNames, part.values), part.data)
		})
	}
	wg.Wait()
	return errors.Join(errs...)
}

// baseMetadata copies the inbound metadata, minus the partition keys, once
// per request so that each partition only has to add its own values.
// keyNames must already be lower-cased.
func baseMetadata(md client.Metadata, keyNames []string) map[string][]string {
	base := make(map[string][]string)
	for k := range md.Keys() {
		base[k] = md.Get(k)
	}
	for _, name := range keyNames {
		delete(base, name)
	}
	return base
}

// withPartitionMetadata returns a new context carrying info with the given
// partition key/value pairs merged into base. Partition keys take precedence
// over any pre-existing metadata values for the same keys, since downstream
// consumers need a deterministic value. Keys whose evaluated value is nil
// (isNil: true) are omitted. keyNames must already be lower-cased.
func withPartitionMetadata(ctx context.Context, info client.Info, base map[string][]string, keyNames []string, values []keyValue) context.Context {
	merged := make(map[string][]string, len(base)+len(keyNames))
	maps.Copy(merged, base)
	for i, name := range keyNames {
		if !values[i].isNil {
			merged[name] = []string{values[i].value}
		}
	}
	info.Metadata = client.NewMetadata(merged)
	return client.NewContext(ctx, info)
}
