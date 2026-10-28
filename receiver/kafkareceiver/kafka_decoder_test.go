// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkareceiver

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kgo"
	"go.opentelemetry.io/collector/client"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/consumer/xconsumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/pprofile"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pdata/testdata"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.opentelemetry.io/collector/receiver/xreceiver"
	"go.opentelemetry.io/otel/attribute"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkareceiver/internal/metadata"
)

func TestReceiver_MessageDecoderOnlySelection(t *testing.T) {
	for _, signal := range []string{"logs", "metrics", "traces", "profiles"} {
		t.Run(signal, func(t *testing.T) {
			r := newDecoderTestReceiver(t, signal, createDefaultConfig().(*Config), make(chan decodedMessage, 1)).(*franzConsumer)
			t.Cleanup(r.telemetryBuilder.Shutdown)
			_, err := r.newConsumeFn(extensionsHost{component.MustNewID("context"): &messageContextExtension{}}, nil, r.telemetryBuilder)
			require.NoError(t, err, "an extension implementing only the new message interface must be supported")
		})
	}
}

func TestMessageDecoderBuiltinNameSelection(t *testing.T) {
	for _, tc := range []struct {
		name      string
		extension component.Component
		wantErr   string
	}{
		{
			name:      "message interface overrides builtin",
			extension: logsOnlyMessageExtension{LogsMessageDecoderExtension: &messageContextExtension{}},
		},
		{
			name:      "wrong signal is rejected",
			extension: metricsOnlyMessageExtension{MetricsMessageDecoderExtension: &messageContextExtension{}},
			wantErr:   `extension "otlp_proto" is not a logs unmarshaler`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			decode, err := newLogsDecoder("otlp_proto", receivertest.NewNopSettings(metadata.Type), extensionsHost{
				component.MustNewID("otlp_proto"): tc.extension,
			})
			if tc.wantErr != "" {
				require.EqualError(t, err, tc.wantErr)
				assert.Nil(t, decode)
				return
			}
			require.NoError(t, err)
			// This payload is deliberately not OTLP protobuf: success must come
			// from the registered message extension rather than the builtin.
			data, err := decode(t.Context(), []byte("message payload"))
			require.NoError(t, err)
			value, ok := data.ResourceLogs().At(0).Resource().Attributes().Get("decoded.payload")
			require.True(t, ok)
			assert.Equal(t, "message payload", value.Str())
		})
	}
}

// These wrappers expose just one signal even though their test implementation
// supports all signals, so interface selection cannot accept the wrong signal.
type logsOnlyMessageExtension struct {
	encoding.LogsMessageDecoderExtension
}

type metricsOnlyMessageExtension struct {
	encoding.MetricsMessageDecoderExtension
}

type messageContextKey struct{}

type messageDecodeCall struct {
	ctx      context.Context
	payload  []byte
	metadata client.Metadata
}

func TestReceiver_MessageDecoderContext(t *testing.T) {
	for _, signal := range []string{"logs", "metrics", "traces", "profiles"} {
		for _, extract := range []bool{false, true} {
			t.Run(signal+"/header_extraction="+strconv.FormatBool(extract), func(t *testing.T) {
				_, cfg := mustNewFakeCluster(t)
				cfg.HeaderExtraction.ExtractHeaders = extract
				cfg.HeaderExtraction.Headers = []string{"tenant"}
				calls := make(chan messageDecodeCall, 2)
				ready := make(chan struct{})
				ext := &messageContextExtension{calls: calls, ready: ready}
				received := make(chan decodedMessage, 2)
				r := newDecoderTestReceiver(t, signal, cfg, received)
				require.NoError(t, r.Start(t.Context(), extensionsHost{component.MustNewID("context"): ext}))
				t.Cleanup(func() { assert.NoError(t, r.Shutdown(context.Background())) }) //nolint:usetesting

				// Hold both calls until their separate metadata configurations exist.
				errs := make(chan error, 2)
				for i := range 2 {
					go func() {
						id := strconv.Itoa(i)
						ctx := context.WithValue(t.Context(), messageContextKey{}, id)
						errs <- r.(*franzConsumer).consumeMessage(ctx, &kgo.Record{
							Topic: "messages", Partition: int32(i), Offset: int64(i + 10),
							Value: []byte(id + "-payload\x00\xff"),
							Headers: []kgo.RecordHeader{
								{Key: "tenant", Value: []byte("tenant-" + id)},
								{Key: "tag", Value: []byte("first-" + id)},
								{Key: "tag", Value: []byte("second-" + id)},
							},
						}, attribute.NewSet())
					}()
				}
				for range 2 {
					call := awaitDecoderValue(t, calls)
					id := call.ctx.Value(messageContextKey{}).(string)
					assert.Equal(t, []byte(id+"-payload\x00\xff"), call.payload)
					assert.Equal(t, []string{"tenant-" + id}, call.metadata.Get("tenant"))
					assert.Equal(t, []string{"first-" + id, "second-" + id}, call.metadata.Get("tag"))
				}
				close(ready)
				for range 2 {
					require.NoError(t, awaitDecoderValue(t, errs))
					message := awaitDecoderValue(t, received)
					id := message.ctx.Value(messageContextKey{}).(string)
					require.Len(t, message.resources, 1)
					assert.Positive(t, message.count)
					attrs := message.resources[0]
					assert.Equal(t, "tenant-"+id, attrs["decoded.tenant"])
					assert.Equal(t, "first-"+id+",second-"+id, attrs["decoded.tags"])
					assert.Equal(t, "messages", attrs["decoded.topic"])
					assert.Equal(t, id, attrs["decoded.partition"])
					assert.Equal(t, "1"+id, attrs["decoded.offset"])
					assert.Equal(t, id+"-payload\x00\xff", attrs["decoded.payload"])
					value, present := attrs["kafka.header.tenant"]
					assert.Equal(t, extract, present)
					if extract {
						assert.Equal(t, "tenant-"+id, value)
					}
				}
				assert.Empty(t, received, "each Kafka message must produce one consumer call")
			})
		}
	}
}

func TestReceiver_MessageDecoderContextCancellation(t *testing.T) {
	for _, signal := range []string{"logs", "metrics", "traces", "profiles"} {
		t.Run(signal, func(t *testing.T) {
			_, cfg := mustNewFakeCluster(t)
			calls := make(chan messageDecodeCall, 1)
			received := make(chan decodedMessage, 1)
			r := newDecoderTestReceiver(t, signal, cfg, received)
			require.NoError(t, r.Start(t.Context(), extensionsHost{component.MustNewID("context"): &messageContextExtension{calls: calls}}))
			t.Cleanup(func() { assert.NoError(t, r.Shutdown(context.Background())) }) //nolint:usetesting
			ctx, cancel := context.WithTimeout(context.WithValue(t.Context(), messageContextKey{}, "operation"), time.Minute)
			cancel()
			require.NoError(t, r.(*franzConsumer).consumeMessage(ctx, &kgo.Record{Topic: "messages", Value: []byte("payload")}, attribute.NewSet()))
			call := awaitDecoderValue(t, calls)
			assert.ErrorIs(t, call.ctx.Err(), context.Canceled)
			assert.Equal(t, ctx.Done(), call.ctx.Done())
			assert.Equal(t, "operation", call.ctx.Value(messageContextKey{}))
			wantDeadline, _ := ctx.Deadline()
			gotDeadline, ok := call.ctx.Deadline()
			assert.True(t, ok)
			assert.Equal(t, wantDeadline, gotDeadline)
			assert.ErrorIs(t, awaitDecoderValue(t, received).ctx.Err(), context.Canceled)
		})
	}
}

func TestReceiver_MessageDecoderSelectsFormatFromHeader(t *testing.T) {
	_, cfg := mustNewFakeCluster(t)
	cfg.Metrics.Encoding = "header"
	cfg.HeaderExtraction.ExtractHeaders = false
	next := new(consumertest.MetricsSink)
	r, err := NewFactory().CreateMetrics(t.Context(), receivertest.NewNopSettings(metadata.Type), cfg, next)
	require.NoError(t, err)
	ext := &headerMetricsMessageExtension{}
	require.NoError(t, r.Start(t.Context(), extensionsHost{component.MustNewID("header"): ext}))
	t.Cleanup(func() { assert.NoError(t, r.Shutdown(context.Background())) }) //nolint:usetesting

	// One shared extension selects the actual codec separately for each message.
	for i, tc := range []struct {
		format     string
		marshaler  pmetric.Marshaler
		metricName string
		value      int64
	}{
		{format: "json", marshaler: &pmetric.JSONMarshaler{}, metricName: "json.metric", value: 42},
		{format: "proto", marshaler: &pmetric.ProtoMarshaler{}, metricName: "protobuf.metric", value: 99},
	} {
		t.Run(tc.format, func(t *testing.T) {
			input := pmetric.NewMetrics()
			metric := input.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
			metric.SetName(tc.metricName)
			metric.SetEmptyGauge().DataPoints().AppendEmpty().SetIntValue(tc.value)
			payload, err := tc.marshaler.MarshalMetrics(input)
			require.NoError(t, err)
			require.NoError(t, r.(*franzConsumer).consumeMessage(t.Context(), &kgo.Record{
				Topic: "messages", Value: payload,
				Headers: []kgo.RecordHeader{{Key: "format", Value: []byte(tc.format)}},
			}, attribute.NewSet()))

			allMetrics := next.AllMetrics()
			require.Len(t, allMetrics, i+1)
			decoded := allMetrics[i]
			require.Equal(t, 1, decoded.ResourceMetrics().Len())
			resourceMetrics := decoded.ResourceMetrics().At(0)
			require.Equal(t, 1, resourceMetrics.ScopeMetrics().Len())
			metrics := resourceMetrics.ScopeMetrics().At(0).Metrics()
			require.Equal(t, 1, metrics.Len())
			actual := metrics.At(0)
			assert.Equal(t, tc.metricName, actual.Name())
			require.Equal(t, pmetric.MetricTypeGauge, actual.Type())
			require.Equal(t, 1, actual.Gauge().DataPoints().Len())
			point := actual.Gauge().DataPoints().At(0)
			assert.Equal(t, pmetric.NumberDataPointValueTypeInt, point.ValueType())
			assert.Equal(t, tc.value, point.IntValue())
		})
	}
}

func TestReceiver_MessageDecoderPreferredOverLegacy(t *testing.T) {
	for _, signal := range []string{"logs", "metrics", "traces", "profiles"} {
		t.Run(signal, func(t *testing.T) {
			_, cfg := mustNewFakeCluster(t)
			received := make(chan decodedMessage, 1)
			r := newDecoderTestReceiver(t, signal, cfg, received)
			ext := &messageAndLegacyExtension{}
			require.NoError(t, r.Start(t.Context(), extensionsHost{component.MustNewID("context"): ext}))
			t.Cleanup(func() { assert.NoError(t, r.Shutdown(context.Background())) }) //nolint:usetesting
			require.NoError(t, r.(*franzConsumer).consumeMessage(t.Context(), &kgo.Record{Topic: "messages", Value: []byte("message payload")}, attribute.NewSet()))
			result := awaitDecoderValue(t, received)
			assert.Equal(t, "message payload", result.resources[0]["decoded.payload"])
			assert.Zero(t, ext.legacyCalls)
		})
	}
}

func TestReceiver_MessageDecoderError(t *testing.T) {
	for name, decodeErr := range map[string]error{
		"invalid message": errors.New("invalid encoded message"),
		"EOF":             io.EOF,
		"wrapped EOF":     fmt.Errorf("incomplete encoded message: %w", io.EOF),
	} {
		for _, signal := range []string{"logs", "metrics", "traces", "profiles"} {
			t.Run(name+"/"+signal, func(t *testing.T) {
				_, cfg := mustNewFakeCluster(t)
				received := make(chan decodedMessage, 1)
				r := newDecoderTestReceiver(t, signal, cfg, received)
				require.NoError(t, r.Start(t.Context(), extensionsHost{component.MustNewID("context"): &messageContextExtension{err: decodeErr}}))
				t.Cleanup(func() { assert.NoError(t, r.Shutdown(context.Background())) }) //nolint:usetesting
				err := r.(*franzConsumer).consumeMessage(t.Context(), &kgo.Record{Topic: "messages", Value: []byte("payload")}, attribute.NewSet())
				require.ErrorIs(t, err, decodeErr)
				assert.True(t, consumererror.IsPermanent(err))
				assert.Empty(t, received, "a decoder result returned with an error must not be consumed")
			})
		}
	}
}

func TestReceiver_DualEncodingKeepsLegacyBehavior(t *testing.T) {
	_, cfg := mustNewFakeCluster(t)
	cfg.Logs.Encoding = "dual"
	ext := &legacyPreferredLogsExtension{}
	received := make(chan plog.Logs, 1)
	next, err := consumer.NewLogs(func(_ context.Context, data plog.Logs) error {
		received <- data
		return nil
	})
	require.NoError(t, err)
	r, err := NewFactory().CreateLogs(t.Context(), receivertest.NewNopSettings(metadata.Type), cfg, next)
	require.NoError(t, err)
	require.NoError(t, r.Start(t.Context(), extensionsHost{component.MustNewID("dual"): ext}))
	t.Cleanup(func() { assert.NoError(t, r.Shutdown(context.Background())) }) //nolint:usetesting
	require.NoError(t, r.(*franzConsumer).consumeMessage(t.Context(), &kgo.Record{Topic: "messages", Value: []byte("legacy payload")}, attribute.NewSet()))
	data := awaitDecoderValue(t, received)
	require.Equal(t, 1, data.LogRecordCount())
	assert.Equal(t, "legacy payload", data.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Body().Str())
	assert.Equal(t, 1, ext.legacyCalls)
	assert.Zero(t, ext.streamingCalls)
}

type legacyPreferredLogsExtension struct {
	component.Component
	legacyCalls    int
	streamingCalls int
}

func (e *legacyPreferredLogsExtension) UnmarshalLogs(payload []byte) (plog.Logs, error) {
	e.legacyCalls++
	data := plog.NewLogs()
	data.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetStr(string(payload))
	return data, nil
}

func (e *legacyPreferredLogsExtension) NewLogsDecoder(io.Reader, ...encoding.DecoderOption) (encoding.LogsDecoder, error) {
	e.streamingCalls++
	return nil, errors.New("streaming decoder does not support the legacy encoding")
}

func TestReceiver_LegacyUnmarshalerEOF(t *testing.T) {
	for _, signal := range []string{"logs", "metrics", "traces", "profiles"} {
		for name, unmarshalErr := range map[string]error{
			"EOF":         io.EOF,
			"wrapped EOF": fmt.Errorf("invalid payload: %w", io.EOF),
		} {
			t.Run(signal+"/"+name, func(t *testing.T) {
				_, cfg := mustNewFakeCluster(t)
				received := make(chan decodedMessage, 1)
				r := newDecoderTestReceiver(t, signal, cfg, received)
				ext := &legacyMessageExtension{err: unmarshalErr}
				require.NoError(t, r.Start(t.Context(), extensionsHost{component.MustNewID("context"): ext}))
				t.Cleanup(func() { assert.NoError(t, r.Shutdown(context.Background())) }) //nolint:usetesting
				err := r.(*franzConsumer).consumeMessage(t.Context(), &kgo.Record{
					Topic: "messages", Value: []byte("invalid"),
				}, attribute.NewSet())
				require.ErrorIs(t, err, unmarshalErr)
				assert.True(t, consumererror.IsPermanent(err))
				assert.Empty(t, received, "legacy unmarshal errors must remain decoding errors")
			})
		}
	}
}

func TestReceiver_LegacyUnmarshaler(t *testing.T) {
	for _, signal := range []string{"logs", "metrics", "traces", "profiles"} {
		t.Run(signal, func(t *testing.T) {
			_, cfg := mustNewFakeCluster(t)
			received := make(chan decodedMessage, 1)
			r := newDecoderTestReceiver(t, signal, cfg, received)
			require.NoError(t, r.Start(t.Context(), extensionsHost{component.MustNewID("context"): &legacyMessageExtension{}}))
			t.Cleanup(func() { assert.NoError(t, r.Shutdown(context.Background())) }) //nolint:usetesting
			require.NoError(t, r.(*franzConsumer).consumeMessage(t.Context(), &kgo.Record{
				Topic: "messages", Value: []byte("valid"),
			}, attribute.NewSet()))
			message := awaitDecoderValue(t, received)
			wantCount := 1
			if signal == "metrics" {
				wantCount = testdata.GenerateMetrics(1).DataPointCount()
			}
			assert.Equal(t, wantCount, message.count)
		})
	}
}

// legacyMessageExtension has no streaming factories. Its existing byte-only
// methods must retain their success and error behavior.
type legacyMessageExtension struct {
	component.Component
	err error
}

func (e *legacyMessageExtension) UnmarshalLogs([]byte) (plog.Logs, error) {
	if e.err != nil {
		return plog.Logs{}, e.err
	}
	return testdata.GenerateLogs(1), nil
}

func (e *legacyMessageExtension) UnmarshalMetrics([]byte) (pmetric.Metrics, error) {
	if e.err != nil {
		return pmetric.Metrics{}, e.err
	}
	return testdata.GenerateMetrics(1), nil
}

func (e *legacyMessageExtension) UnmarshalTraces([]byte) (ptrace.Traces, error) {
	if e.err != nil {
		return ptrace.Traces{}, e.err
	}
	return testdata.GenerateTraces(1), nil
}

func (e *legacyMessageExtension) UnmarshalProfiles([]byte) (pprofile.Profiles, error) {
	if e.err != nil {
		return pprofile.Profiles{}, e.err
	}
	return testdata.GenerateProfiles(1), nil
}

type decodedMessage struct {
	ctx       context.Context
	count     int
	resources []map[string]any
}

func awaitDecoderValue[T any](t *testing.T, ch <-chan T) T {
	t.Helper()
	select {
	case value := <-ch:
		return value
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for message decoding")
		var zero T
		return zero
	}
}

func newDecoderTestReceiver(t *testing.T, signal string, cfg *Config, received chan<- decodedMessage) component.Component {
	t.Helper()
	set := receivertest.NewNopSettings(metadata.Type)
	factory := NewFactory()
	var r component.Component
	var err error
	switch signal {
	case "logs":
		cfg.Logs.Encoding = "context"
		next, e := consumer.NewLogs(func(ctx context.Context, data plog.Logs) error {
			result := decodedMessage{ctx: ctx, count: data.LogRecordCount()}
			for _, resource := range data.ResourceLogs().All() {
				result.resources = append(result.resources, resource.Resource().Attributes().AsRaw())
			}
			received <- result
			return nil
		})
		require.NoError(t, e)
		r, err = factory.CreateLogs(t.Context(), set, cfg, next)
	case "metrics":
		cfg.Metrics.Encoding = "context"
		next, e := consumer.NewMetrics(func(ctx context.Context, data pmetric.Metrics) error {
			result := decodedMessage{ctx: ctx, count: data.DataPointCount()}
			for _, resource := range data.ResourceMetrics().All() {
				result.resources = append(result.resources, resource.Resource().Attributes().AsRaw())
			}
			received <- result
			return nil
		})
		require.NoError(t, e)
		r, err = factory.CreateMetrics(t.Context(), set, cfg, next)
	case "traces":
		cfg.Traces.Encoding = "context"
		next, e := consumer.NewTraces(func(ctx context.Context, data ptrace.Traces) error {
			result := decodedMessage{ctx: ctx, count: data.SpanCount()}
			for _, resource := range data.ResourceSpans().All() {
				result.resources = append(result.resources, resource.Resource().Attributes().AsRaw())
			}
			received <- result
			return nil
		})
		require.NoError(t, e)
		r, err = factory.CreateTraces(t.Context(), set, cfg, next)
	case "profiles":
		cfg.Profiles.Encoding = "context"
		next, e := xconsumer.NewProfiles(func(ctx context.Context, data pprofile.Profiles) error {
			result := decodedMessage{ctx: ctx, count: data.SampleCount()}
			for _, resource := range data.ResourceProfiles().All() {
				result.resources = append(result.resources, resource.Resource().Attributes().AsRaw())
			}
			received <- result
			return nil
		})
		require.NoError(t, e)
		r, err = factory.(xreceiver.Factory).CreateProfiles(t.Context(), set, cfg, next)
	}
	require.NoError(t, err)
	return r
}

// messageContextExtension implements only the additive message interfaces.
// Options remain local to each call, including when the shared extension is busy.
type messageContextExtension struct {
	component.Component
	calls chan<- messageDecodeCall
	ready <-chan struct{}
	err   error
}

func (e *messageContextExtension) decode(ctx context.Context, payload []byte, resource pcommon.Resource, options ...encoding.MessageDecodeOption) error {
	config, err := encoding.NewDecodeConfig(options...)
	if err != nil {
		return err
	}
	if e.calls != nil {
		e.calls <- messageDecodeCall{ctx: ctx, payload: payload, metadata: config.MessageMetadata}
	}
	if e.ready != nil {
		select {
		case <-e.ready:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	meta := config.MessageMetadata
	attrs := resource.Attributes()
	attrs.PutStr("decoded.tenant", strings.Join(meta.Get("tenant"), ","))
	attrs.PutStr("decoded.tags", strings.Join(meta.Get("tag"), ","))
	attrs.PutStr("decoded.topic", strings.Join(meta.Get("kafka.topic"), ","))
	attrs.PutStr("decoded.partition", strings.Join(meta.Get("kafka.partition"), ","))
	attrs.PutStr("decoded.offset", strings.Join(meta.Get("kafka.offset"), ","))
	attrs.PutStr("decoded.payload", string(payload))
	return e.err
}

func (e *messageContextExtension) DecodeLogs(ctx context.Context, payload []byte, options ...encoding.LogsDecodeOption) (plog.Logs, error) {
	data := testdata.GenerateLogs(1)
	return data, e.decode(ctx, payload, data.ResourceLogs().At(0).Resource(), options...)
}

func (e *messageContextExtension) DecodeMetrics(ctx context.Context, payload []byte, options ...encoding.MetricsDecodeOption) (pmetric.Metrics, error) {
	data := testdata.GenerateMetrics(1)
	return data, e.decode(ctx, payload, data.ResourceMetrics().At(0).Resource(), options...)
}

func (e *messageContextExtension) DecodeTraces(ctx context.Context, payload []byte, options ...encoding.TracesDecodeOption) (ptrace.Traces, error) {
	data := testdata.GenerateTraces(1)
	return data, e.decode(ctx, payload, data.ResourceSpans().At(0).Resource(), options...)
}

func (e *messageContextExtension) DecodeProfiles(ctx context.Context, payload []byte, options ...encoding.ProfilesDecodeOption) (pprofile.Profiles, error) {
	data := testdata.GenerateProfiles(1)
	return data, e.decode(ctx, payload, data.ResourceProfiles().At(0).Resource(), options...)
}

type headerMetricsMessageExtension struct {
	component.Component
}

func (*headerMetricsMessageExtension) DecodeMetrics(_ context.Context, payload []byte, options ...encoding.MetricsDecodeOption) (pmetric.Metrics, error) {
	config, err := encoding.NewDecodeConfig(options...)
	if err != nil {
		return pmetric.Metrics{}, err
	}
	formats := config.MessageMetadata.Get("format")
	if len(formats) != 1 {
		return pmetric.Metrics{}, errors.New("expected one format header")
	}
	switch formats[0] {
	case "json":
		return (&pmetric.JSONUnmarshaler{}).UnmarshalMetrics(payload)
	case "proto":
		return (&pmetric.ProtoUnmarshaler{}).UnmarshalMetrics(payload)
	default:
		return pmetric.Metrics{}, fmt.Errorf("unsupported metrics format %q", formats[0])
	}
}

type messageAndLegacyExtension struct {
	messageContextExtension
	legacyCalls int
}

func (e *messageAndLegacyExtension) UnmarshalLogs([]byte) (plog.Logs, error) {
	e.legacyCalls++
	return plog.Logs{}, errors.New("unexpected legacy decoding")
}

func (e *messageAndLegacyExtension) UnmarshalMetrics([]byte) (pmetric.Metrics, error) {
	e.legacyCalls++
	return pmetric.Metrics{}, errors.New("unexpected legacy decoding")
}

func (e *messageAndLegacyExtension) UnmarshalTraces([]byte) (ptrace.Traces, error) {
	e.legacyCalls++
	return ptrace.Traces{}, errors.New("unexpected legacy decoding")
}

func (e *messageAndLegacyExtension) UnmarshalProfiles([]byte) (pprofile.Profiles, error) {
	e.legacyCalls++
	return pprofile.Profiles{}, errors.New("unexpected legacy decoding")
}
