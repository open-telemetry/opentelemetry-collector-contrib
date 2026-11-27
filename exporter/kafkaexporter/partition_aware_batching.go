// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkaexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter"

import (
	"context"
	"fmt"

	"github.com/twmb/franz-go/pkg/kgo"
	"go.opentelemetry.io/collector/client"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pdata/xpdata/xhash"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter/internal/kafkaclient"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/traceutil"
	kafkatopic "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/kafka/topic"
)

type partitionBucketKey struct {
	topic     string
	partition int
}

type partitionBucket[R any] struct {
	key  []byte
	refs []R
}

func getTopicPlan(
	plans map[string]*topicPartitionPlan,
	partitioner *partitionAwarePartitioner,
	topic string,
) (*topicPartitionPlan, bool) {
	if plan, ok := plans[topic]; ok {
		return plan, true
	}
	plan, ok := partitioner.newTopicPlan(topic)
	if !ok {
		return nil, false
	}
	plans[topic] = plan
	return plan, true
}

func appendPartitionRef[R any](
	buckets map[partitionBucketKey]partitionBucket[R],
	topic string,
	partition int,
	key []byte,
	ref R,
) {
	bucketKey := partitionBucketKey{topic: topic, partition: partition}
	bucket, ok := buckets[bucketKey]
	if !ok {
		bucket.key = key
	}
	bucket.refs = append(bucket.refs, ref)
	buckets[bucketKey] = bucket
}

func marshalPartitionBuckets[T, R any](
	buckets map[partitionBucketKey]partitionBucket[R],
	build func([]R) T,
	marshal func(T, func([]byte, []byte)) error,
	yield func(kgo.Record),
) error {
	for destination, bucket := range buckets {
		err := marshal(build(bucket.refs), func(_, value []byte) {
			yield(kgo.Record{
				Topic: destination.topic,
				Key:   bucket.key,
				Value: value,
				// Do not retain the conversion context while a request is queued.
				// ExportData rebases this marker onto the active push context.
				Context: kafkaclient.WithPlannedPartition(context.Background(), destination.partition),
			})
		})
		if err != nil {
			return fmt.Errorf("error exporting to topic %q: %w", destination.topic, err)
		}
	}
	return nil
}

type traceRef struct {
	resource int
	scope    int
	span     int
}

func (e *kafkaTracesMessenger) partitionAwareRecords(
	ctx context.Context,
	td ptrace.Traces,
	partitioner *partitionAwarePartitioner,
	yield func(kgo.Record),
) (bool, error) {
	if !e.config.PartitionTracesByID || !supportsPartitionAwareBatching(e.marshaler) {
		return false, nil
	}

	buckets := make(map[partitionBucketKey]partitionBucket[traceRef])
	plans := make(map[string]*topicPartitionPlan)
	for resourceIndex, resourceSpans := range td.ResourceSpans().All() {
		topic := getTopicForResource(ctx, e.config.Traces, e.config.TopicFromAttribute, resourceSpans.Resource())
		plan, ok := getTopicPlan(plans, partitioner, topic)
		if !ok {
			return false, nil
		}
		for scopeIndex, scopeSpans := range resourceSpans.ScopeSpans().All() {
			for spanIndex, span := range scopeSpans.Spans().All() {
				key := []byte(traceutil.TraceIDToHexOrEmptyString(span.TraceID()))
				partition := plan.plan(key)
				appendPartitionRef(buckets, topic, partition, key, traceRef{
					resource: resourceIndex,
					scope:    scopeIndex,
					span:     spanIndex,
				})
			}
		}
	}

	err := marshalPartitionBuckets(
		buckets,
		func(refs []traceRef) ptrace.Traces { return buildTraces(td, refs) },
		e.marshalData,
		yield,
	)
	return true, err
}

func buildTraces(source ptrace.Traces, refs []traceRef) ptrace.Traces {
	result := ptrace.NewTraces()
	lastResource, lastScope := -1, -1
	var targetResource ptrace.ResourceSpans
	var targetScope ptrace.ScopeSpans
	for _, ref := range refs {
		sourceResource := source.ResourceSpans().At(ref.resource)
		if ref.resource != lastResource {
			targetResource = result.ResourceSpans().AppendEmpty()
			sourceResource.Resource().CopyTo(targetResource.Resource())
			targetResource.SetSchemaUrl(sourceResource.SchemaUrl())
			lastResource, lastScope = ref.resource, -1
		}
		sourceScope := sourceResource.ScopeSpans().At(ref.scope)
		if ref.scope != lastScope {
			targetScope = targetResource.ScopeSpans().AppendEmpty()
			sourceScope.Scope().CopyTo(targetScope.Scope())
			targetScope.SetSchemaUrl(sourceScope.SchemaUrl())
			lastScope = ref.scope
		}
		sourceScope.Spans().At(ref.span).CopyTo(targetScope.Spans().AppendEmpty())
	}
	return result
}

type logRef struct {
	resource int
	scope    int
	record   int
}

type unplannedLogRef struct {
	topic string
	ref   logRef
}

func (e *kafkaLogsMessenger) partitionAwareRecords(
	ctx context.Context,
	ld plog.Logs,
	partitioner *partitionAwarePartitioner,
	yield func(kgo.Record),
) (bool, error) {
	if (!e.config.PartitionLogsByResourceAttributes && !e.config.PartitionLogsByTraceID) ||
		!supportsPartitionAwareBatching(e.marshaler) {
		return false, nil
	}

	buckets := make(map[partitionBucketKey]partitionBucket[logRef])
	plans := make(map[string]*topicPartitionPlan)
	var unplanned []unplannedLogRef
	for resourceIndex, resourceLogs := range ld.ResourceLogs().All() {
		topic := getTopicForResource(ctx, e.config.Logs, e.config.TopicFromAttribute, resourceLogs.Resource())
		plan, ok := getTopicPlan(plans, partitioner, topic)
		if !ok {
			return false, nil
		}
		var resourceKey []byte
		if e.config.PartitionLogsByResourceAttributes {
			hash := xhash.MapHash(resourceLogs.Resource().Attributes())
			resourceKey = hash[:]
		}
		for scopeIndex, scopeLogs := range resourceLogs.ScopeLogs().All() {
			for recordIndex, record := range scopeLogs.LogRecords().All() {
				key := resourceKey
				if e.config.PartitionLogsByTraceID {
					if record.TraceID().IsEmpty() {
						unplanned = append(unplanned, unplannedLogRef{
							topic: topic,
							ref: logRef{
								resource: resourceIndex,
								scope:    scopeIndex,
								record:   recordIndex,
							},
						})
						continue
					}
					key = []byte(traceutil.TraceIDToHexOrEmptyString(record.TraceID()))
				}
				partition := plan.plan(key)
				appendPartitionRef(buckets, topic, partition, key, logRef{
					resource: resourceIndex,
					scope:    scopeIndex,
					record:   recordIndex,
				})
			}
		}
	}

	err := marshalPartitionBuckets(
		buckets,
		func(refs []logRef) plog.Logs { return buildLogs(ld, refs) },
		e.marshalData,
		yield,
	)
	if err != nil {
		return true, err
	}
	for _, unplannedRef := range unplanned {
		data := buildLogs(ld, []logRef{unplannedRef.ref})
		err := e.marshalData(data, func(key, value []byte) {
			yield(kgo.Record{
				Topic: unplannedRef.topic,
				Key:   key,
				Value: value,
			})
		})
		if err != nil {
			return true, fmt.Errorf("error exporting to topic %q: %w", unplannedRef.topic, err)
		}
	}
	return true, nil
}

func buildLogs(source plog.Logs, refs []logRef) plog.Logs {
	result := plog.NewLogs()
	lastResource, lastScope := -1, -1
	var targetResource plog.ResourceLogs
	var targetScope plog.ScopeLogs
	for _, ref := range refs {
		sourceResource := source.ResourceLogs().At(ref.resource)
		if ref.resource != lastResource {
			targetResource = result.ResourceLogs().AppendEmpty()
			sourceResource.Resource().CopyTo(targetResource.Resource())
			targetResource.SetSchemaUrl(sourceResource.SchemaUrl())
			lastResource, lastScope = ref.resource, -1
		}
		sourceScope := sourceResource.ScopeLogs().At(ref.scope)
		if ref.scope != lastScope {
			targetScope = targetResource.ScopeLogs().AppendEmpty()
			sourceScope.Scope().CopyTo(targetScope.Scope())
			targetScope.SetSchemaUrl(sourceScope.SchemaUrl())
			lastScope = ref.scope
		}
		sourceScope.LogRecords().At(ref.record).CopyTo(targetScope.LogRecords().AppendEmpty())
	}
	return result
}

func (e *kafkaMetricsMessenger) partitionAwareRecords(
	ctx context.Context,
	md pmetric.Metrics,
	partitioner *partitionAwarePartitioner,
	yield func(kgo.Record),
) (bool, error) {
	if !e.config.PartitionMetricsByResourceAttributes || !supportsPartitionAwareBatching(e.marshaler) {
		return false, nil
	}

	buckets := make(map[partitionBucketKey]partitionBucket[int])
	plans := make(map[string]*topicPartitionPlan)
	for resourceIndex, resourceMetrics := range md.ResourceMetrics().All() {
		topic := getTopicForResource(ctx, e.config.Metrics, e.config.TopicFromAttribute, resourceMetrics.Resource())
		plan, ok := getTopicPlan(plans, partitioner, topic)
		if !ok {
			return false, nil
		}
		hash := xhash.MapHash(resourceMetrics.Resource().Attributes())
		key := hash[:]
		partition := plan.plan(key)
		appendPartitionRef(buckets, topic, partition, key, resourceIndex)
	}

	err := marshalPartitionBuckets(
		buckets,
		func(refs []int) pmetric.Metrics { return buildMetrics(md, refs) },
		e.marshalData,
		yield,
	)
	return true, err
}

func buildMetrics(source pmetric.Metrics, refs []int) pmetric.Metrics {
	result := pmetric.NewMetrics()
	for _, resourceIndex := range refs {
		source.ResourceMetrics().At(resourceIndex).CopyTo(result.ResourceMetrics().AppendEmpty())
	}
	return result
}

func supportsPartitionAwareBatching(value any) bool {
	_, ok := value.(interface{ SupportsPartitionAwareBatching() })
	return ok
}

func getTopicForResource(
	ctx context.Context,
	signalCfg SignalConfig,
	topicFromAttribute string,
	resource pcommon.Resource,
) string {
	if key := signalCfg.TopicFromMetadataKey; key != "" {
		if values := client.FromContext(ctx).Metadata.Get(key); len(values) > 0 {
			return values[0]
		}
	}
	if topicFromAttribute != "" {
		if value, ok := resource.Attributes().Get(topicFromAttribute); ok && value.Str() != "" {
			return value.Str()
		}
	}
	if value, ok := kafkatopic.FromContext(ctx); ok {
		return value
	}
	return signalCfg.Topic
}
