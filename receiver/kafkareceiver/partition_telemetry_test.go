// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkareceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkareceiver"

import (
	"context"
	"errors"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kgo"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.opentelemetry.io/otel/attribute"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/metric/metricdata/metricdatatest"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkareceiver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkareceiver/internal/metadatatest"
)

func TestPartitionDataWait(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		set, tel, _ := mustNewSettings(t)
		cfg := createDefaultConfig().(*Config)
		cfg.PartitionProcessing.Independent = true
		cfg.MessageMarking = MessageMarking{After: true}
		cfg.ErrorBackOff.Enabled = false
		c, err := newFranzKafkaConsumer(cfg, set, nil, nil, nil)
		require.NoError(t, err)
		defer c.telemetryBuilder.Shutdown()
		ctx, cancel := context.WithCancelCause(t.Context())
		attrs := attribute.NewSet(attribute.String("topic", "test"), attribute.Int("partition", 0))
		p := &pc{ctx: ctx, cancel: cancel, attrs: attrs, telemetry: newPartitionTelemetry(attrs)}
		stableAttrs := attribute.NewSet(attribute.String("topic", "test"), attribute.Int("partition", 1))
		stable := &pc{ctx: t.Context(), attrs: stableAttrs, telemetry: newPartitionTelemetry(stableAttrs)}
		c.assignments = map[topicPartition]*pc{{topic: "test", partition: 0}: p, {topic: "test", partition: 1}: stable}
		c.partitionTelemetry = map[topicPartition]*partitionTelemetry{{topic: "test", partition: 0}: p.telemetry, {topic: "test", partition: 1}: stable.telemetry}
		c.storeAssignmentSnapshot()
		m := newPartitionMailbox(ctx, 2, 0)
		m.telemetry = p.telemetry
		c.consumeMessage = func(context.Context, *kgo.Record, attribute.Set, *partitionTelemetry) error {
			return errors.New("rewind required")
		}
		for _, tc := range []struct {
			name   string
			act    func()
			active int64
			wait   float64
		}{
			{name: "ongoing wait", act: m.waitForData, wait: 1},
			{name: "stale notification", act: func() { m.notify <- struct{}{}; <-m.notify; m.waitForData() }, wait: 2},
			{name: "queued batch", act: func() { require.True(t, m.enqueue(offsetBatch(2), func(partitionPauseReason) {})) }, wait: 2},
			{name: "current batch", act: func() { _, ok := m.dequeue(func() {}); require.True(t, ok) }, wait: 2},
			{name: "cross fetch active calls", act: func() { p.telemetry.startCall(); p.telemetry.startCall(); m.waitForData() }, active: 2, wait: 2},
			{name: "one call remains", act: func() { p.telemetry.finishCall(nil) }, active: 1, wait: 2},
			{name: "last call returns without notification", act: func() { p.telemetry.finishCall(nil) }, wait: 3},
			{name: "failed call starts recovery", act: func() { require.Error(t, c.handleMessage(p, &kgo.Record{})) }, wait: 3},
			{name: "waiting cannot override recovery", act: m.waitForData, wait: 3},
			{name: "recovery completes", act: func() { m.resumeAfterOffsetChange(func() {}); m.waitForData() }, wait: 4},
			{name: "pending rewind", act: func() { m.requestRewind(&kgo.Record{Offset: 1}, true, func() {}); m.waitForData() }, wait: 4},
			{name: "rewind completes", act: func() { require.NotNil(t, m.takeOffsetChange()); m.resumeAfterOffsetChange(func() {}); m.waitForData() }, wait: 5},
			{name: "cancellation", act: func() { p.cancelContext(errors.New("revoked")); m.discard(); m.waitForData() }, wait: 5},
		} {
			t.Log(tc.name)
			tc.act()
			time.Sleep(time.Second)
			metadatatest.AssertEqualKafkaReceiverPartitionDataWait(t, tel, []metricdata.DataPoint[float64]{
				{Attributes: attrs, Value: tc.wait}, {Attributes: stableAttrs},
			}, metricdatatest.IgnoreTimestamp())
			active := []metricdata.DataPoint[int64]{{Attributes: stableAttrs}}
			if ctx.Err() == nil {
				active = append(active, metricdata.DataPoint[int64]{Attributes: attrs, Value: tc.active})
			}
			metadatatest.AssertEqualKafkaReceiverPartitionInFlightRecords(t, tel, active, metricdatatest.IgnoreTimestamp())
		}
	})
}

func TestPartitionTelemetryLifecycle(t *testing.T) {
	client, err := kgo.NewClient()
	require.NoError(t, err)
	defer client.Close()
	synctest.Test(t, func(t *testing.T) {
		set, tel, _ := mustNewSettings(t)
		cfg := createDefaultConfig().(*Config)
		cfg.PartitionProcessing.Independent = true
		cfg.MessageMarking = MessageMarking{After: true}
		c, err := newFranzKafkaConsumer(cfg, set, nil, nil, nil)
		require.NoError(t, err)
		defer c.telemetryBuilder.Shutdown()
		c.client = client
		c.consumeMessage = func(context.Context, *kgo.Record, attribute.Set, *partitionTelemetry) error {
			return consumererror.NewPermanent(errors.New("terminal failure"))
		}
		start := time.Now()
		c.assigned(t.Context(), client, map[string][]int32{"test": {0, 1}})
		defer c.lost(t.Context(), client, map[string][]int32{"test": {0, 1}}, true)
		old := c.assignments[topicPartition{topic: "test", partition: 1}]
		stableAttrs := c.assignments[topicPartition{topic: "test", partition: 0}].attrs
		for _, tc := range []struct {
			name     string
			act      func()
			wait     float64
			assigned bool
		}{
			{name: "assigned", act: func() {}, wait: 1, assigned: true},
			{name: "lost", act: func() { c.lost(t.Context(), client, map[string][]int32{"test": {1}}, true) }, wait: 1},
			{name: "reassigned", act: func() { c.assigned(t.Context(), client, map[string][]int32{"test": {1}}) }, wait: 2, assigned: true},
			{name: "late old worker", act: func() { old.telemetry.startCall(); old.telemetry.finishCall(nil) }, wait: 3, assigned: true},
			{name: "terminal pause", act: func() {
				current := c.assignments[topicPartition{topic: "test", partition: 1}]
				current.mailbox.enqueue(offsetBatch(1), func(partitionPauseReason) {})
			}, wait: 3},
		} {
			t.Log(tc.name)
			tc.act()
			synctest.Wait()
			time.Sleep(time.Second)
			metadatatest.AssertEqualKafkaReceiverPartitionDataWait(t, tel, []metricdata.DataPoint[float64]{
				{Attributes: stableAttrs, Value: time.Since(start).Seconds()}, {Attributes: old.attrs, Value: tc.wait},
			}, metricdatatest.IgnoreTimestamp())
			active := []metricdata.DataPoint[int64]{{Attributes: stableAttrs}}
			if tc.assigned {
				active = append(active, metricdata.DataPoint[int64]{Attributes: old.attrs})
			}
			metadatatest.AssertEqualKafkaReceiverPartitionInFlightRecords(t, tel, active, metricdatatest.IgnoreTimestamp())
		}
	})
}

func TestPartitionProcessingStageTelemetry(t *testing.T) {
	for _, tc := range []struct {
		name         string
		decodeError  bool
		consumeError bool
		retry        bool
	}{
		{name: "success"},
		{name: "decode failure", decodeError: true},
		{name: "consume failure", consumeError: true},
		{name: "consume retry", retry: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				bounds := []float64{0.015, 0.03}
				tel := componenttest.NewTelemetry(componenttest.WithMetricOptions(sdkmetric.WithView(sdkmetric.NewView(
					sdkmetric.Instrument{Kind: sdkmetric.InstrumentKindHistogram},
					sdkmetric.Stream{Aggregation: sdkmetric.AggregationExplicitBucketHistogram{Boundaries: bounds}},
				))))
				defer func() { require.NoError(t, tel.Shutdown(t.Context())) }()
				set := receivertest.NewNopSettings(metadata.Type)
				set.TelemetrySettings = tel.NewTelemetrySettings()
				cfg := createDefaultConfig().(*Config)
				cfg.PartitionProcessing.Independent = true
				cfg.MessageMarking = MessageMarking{After: true}
				cfg.ErrorBackOff = configretry.BackOffConfig{
					Enabled: true, InitialInterval: time.Second,
					RandomizationFactor: 0, Multiplier: 1, MaxInterval: time.Second,
				}
				c, err := newFranzKafkaConsumer(cfg, set, nil, nil, nil)
				require.NoError(t, err)
				defer c.telemetryBuilder.Shutdown()
				obs, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{ReceiverID: set.ID, Transport: transport, ReceiverCreateSettings: set})
				require.NoError(t, err)
				calls := 0
				handler := &tracesHandler{
					obsrecv: obs,
					decode: func(context.Context, []byte) (ptrace.Traces, error) {
						time.Sleep(10 * time.Millisecond)
						if tc.decodeError {
							return ptrace.Traces{}, errors.New("decode failed")
						}
						return ptrace.NewTraces(), nil
					},
					consumer: newTracesConsumer(func(context.Context, ptrace.Traces) error {
						time.Sleep(20 * time.Millisecond)
						calls++
						if tc.consumeError {
							return consumererror.NewPermanent(errors.New("consume failed"))
						}
						if tc.retry && calls == 1 {
							return errors.New("retry")
						}
						return nil
					}),
				}
				attrs := attribute.NewSet(attribute.String("topic", "test"), attribute.Int("partition", 0))
				p := &pc{ctx: t.Context(), attrs: attrs, telemetry: newPartitionTelemetry(attrs), logger: zap.NewNop()}
				p.telemetry.setWaiting(true)
				c.assignments[topicPartition{topic: "test", partition: 0}] = p
				c.partitionTelemetry[topicPartition{topic: "test", partition: 0}] = p.telemetry
				c.storeAssignmentSnapshot()
				c.consumeMessage = func(ctx context.Context, record *kgo.Record, _ attribute.Set, stages *partitionTelemetry) error {
					return processMessage(ctx, record, cfg, set.Logger, c.telemetryBuilder, handler, attrs, stages, nil, nil)
				}
				done := make(chan struct{})
				go func() { _ = c.handleMessage(p, &kgo.Record{}); close(done) }()
				time.Sleep(500 * time.Millisecond)
				if tc.retry {
					metadatatest.AssertEqualKafkaReceiverPartitionInFlightRecords(t, tel, []metricdata.DataPoint[int64]{{Attributes: attrs, Value: 1}}, metricdatatest.IgnoreTimestamp())
					metadatatest.AssertEqualKafkaReceiverPartitionDataWait(t, tel, []metricdata.DataPoint[float64]{{Attributes: attrs}}, metricdatatest.IgnoreTimestamp())
				}
				<-done
				count := uint64(1)
				if tc.retry {
					count = 2
				}
				stages := []string{"decode", "consume"}
				if tc.decodeError {
					stages = stages[:1]
				}
				var points []metricdata.HistogramDataPoint[float64]
				for _, stage := range stages {
					seconds := 0.01
					buckets := []uint64{count, 0, 0}
					if stage == "consume" {
						seconds = 0.02
						buckets = []uint64{0, count, 0}
					}
					points = append(points, metricdata.HistogramDataPoint[float64]{
						Attributes: attribute.NewSet(attribute.String("topic", "test"), attribute.Int("partition", 0), attribute.String("stage", stage)),
						Count:      count, Sum: seconds * float64(count), Bounds: bounds, BucketCounts: buckets,
						Min: metricdata.NewExtrema(seconds), Max: metricdata.NewExtrema(seconds),
					})
				}
				metadatatest.AssertEqualKafkaReceiverRecordProcessingDuration(t, tel, points, metricdatatest.IgnoreTimestamp(), metricdatatest.IgnoreExemplars())
			})
		})
	}
}
