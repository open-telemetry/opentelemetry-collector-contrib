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
	"github.com/twmb/franz-go/pkg/kfake"
	"github.com/twmb/franz-go/pkg/kgo"
	"go.opentelemetry.io/collector/client"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/consumer/xconsumer"
	"go.opentelemetry.io/collector/featuregate"
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

func TestReceiver_StreamingDecoder(t *testing.T) {
	gate := metadata.ReceiverKafkaPreferStreamingEncodingFeatureGate
	previous := gate.IsEnabled()
	require.NoError(t, featuregate.GlobalRegistry().Set(gate.ID(), true))
	t.Cleanup(func() { assert.NoError(t, featuregate.GlobalRegistry().Set(gate.ID(), previous)) })

	kafkaClient, cfg := mustNewFakeCluster(t, kfake.SeedTopics(1, defaultLogsTopic))
	cfg.Logs.Encoding = "streaming"
	require.NoError(t, kafkaClient.ProduceSync(t.Context(), &kgo.Record{
		Topic: defaultLogsTopic,
		Value: []byte("first\nsecond"),
	}).FirstErr())

	used := make(chan string, 1)
	ext := &streamingLogsExtension{used: used}
	received := make(chan plog.Logs, 1)
	next, err := consumer.NewLogs(func(_ context.Context, logs plog.Logs) error {
		received <- logs
		return nil
	})
	require.NoError(t, err)
	r, err := NewFactory().CreateLogs(t.Context(), receivertest.NewNopSettings(metadata.Type), cfg, next)
	require.NoError(t, err)
	require.NoError(t, r.Start(t.Context(), extensionsHost{component.MustNewID("streaming"): ext}))
	t.Cleanup(func() { assert.NoError(t, r.Shutdown(context.Background())) }) //nolint:usetesting

	select {
	case path := <-used:
		require.Equal(t, "streaming", path, "native streaming decoding must take precedence over the legacy unmarshaler")
	case <-time.After(10 * time.Second):
		t.Fatal("message was not decoded")
	}
	select {
	case logs := <-received:
		require.Equal(t, 2, logs.LogRecordCount())
		require.Equal(t, 2, logs.ResourceLogs().Len())
		assert.Equal(t, "first", logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Body().Str())
		assert.Equal(t, "second", logs.ResourceLogs().At(1).ScopeLogs().At(0).LogRecords().At(0).Body().Str())
	case <-time.After(10 * time.Second):
		t.Fatal("decoded message was not consumed")
	}
}

// streamingLogsExtension implements both contracts to verify the receiver chooses
// the streaming contract when preference for streaming is explicitly enabled.
type streamingLogsExtension struct {
	component.Component
	used chan<- string
}

func (e *streamingLogsExtension) UnmarshalLogs([]byte) (plog.Logs, error) {
	e.used <- "legacy"
	return plog.Logs{}, errors.New("unexpected legacy decoding")
}

func (e *streamingLogsExtension) NewLogsDecoder(reader io.Reader, _ ...encoding.DecoderOption) (encoding.LogsDecoder, error) {
	e.used <- "streaming"
	payload, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	return &streamingLogsDecoder{batches: strings.Split(string(payload), "\n")}, nil
}

type streamingLogsDecoder struct {
	batches []string
	next    int
}

func (d *streamingLogsDecoder) DecodeLogs() (plog.Logs, error) {
	if d.next == len(d.batches) {
		return plog.Logs{}, io.EOF
	}
	logs := plog.NewLogs()
	logs.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetStr(d.batches[d.next])
	d.next++
	return logs, nil
}

func (d *streamingLogsDecoder) Offset() int64 { return int64(d.next) }

func TestReceiver_DualEncodingKeepsLegacyBehavior(t *testing.T) {
	require.False(t, metadata.ReceiverKafkaPreferStreamingEncodingFeatureGate.IsEnabled(),
		"streaming preference must remain disabled by default")
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
	require.NoError(t, r.(*franzConsumer).consumeMessage(t.Context(), &kgo.Record{
		Topic: "messages", Value: []byte("legacy payload"),
	}, attribute.NewSet()))
	data := awaitDecoderValue(t, received)
	require.Equal(t, 1, data.LogRecordCount())
	assert.Equal(t, "legacy payload", data.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Body().Str())
	assert.Equal(t, 1, ext.legacyCalls)
	assert.Zero(t, ext.streamingCalls, "a dual extension's streaming path may not support its legacy encodings")
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

func TestReceiver_StreamingDecoderContext(t *testing.T) {
	for _, signal := range []string{"logs", "metrics", "traces", "profiles"} {
		for _, extract := range []bool{false, true} {
			t.Run(signal+"/header_extraction="+strconv.FormatBool(extract), func(t *testing.T) {
				_, cfg := mustNewFakeCluster(t)
				cfg.HeaderExtraction.ExtractHeaders = extract
				cfg.HeaderExtraction.Headers = []string{"tenant"}
				created := make(chan struct{}, 2)
				ready := make(chan struct{})
				ext := &messageContextExtension{created: created, ready: ready}
				received := make(chan decodedMessage, 4)
				r := newDecoderTestReceiver(t, signal, cfg, received)
				require.NoError(t, r.Start(t.Context(), extensionsHost{component.MustNewID("context"): ext}))
				t.Cleanup(func() { assert.NoError(t, r.Shutdown(context.Background())) }) //nolint:usetesting

				// Both decoders must exist before either starts reading headers. This
				// makes accidental shared message state observable without sleeps.
				errs := make(chan error, 2)
				for i := range 2 {
					go func() {
						id := strconv.Itoa(i)
						errs <- r.(*franzConsumer).consumeMessage(t.Context(), &kgo.Record{
							Topic: "messages", Partition: int32(i), Offset: int64(i + 10),
							Value: []byte(id + "-first\n" + id + "-second"),
							Headers: []kgo.RecordHeader{
								{Key: "tenant", Value: []byte("tenant-" + id)},
								{Key: "tag", Value: []byte("first-" + id)},
								{Key: "tag", Value: []byte("second-" + id)},
							},
						}, attribute.NewSet())
					}()
				}
				for range 2 {
					select {
					case <-created:
					case <-time.After(10 * time.Second):
						close(ready)
						t.Fatal("a decoder was not created for each message")
					}
				}
				close(ready)
				for range 2 {
					require.NoError(t, awaitDecoderValue(t, errs))
					message := awaitDecoderValue(t, received)
					id := client.FromContext(message.ctx).Metadata.Get("tenant")[0][len("tenant-"):]
					require.Equal(t, 2, message.count)
					require.Len(t, message.resources, 2)
					for j, attrs := range message.resources {
						assert.Equal(t, "tenant-"+id, attrs["decoded.tenant"])
						assert.Equal(t, "first-"+id+",second-"+id, attrs["decoded.tags"])
						assert.Equal(t, "messages", attrs["decoded.topic"])
						assert.Equal(t, id, attrs["decoded.partition"])
						assert.Equal(t, "1"+id, attrs["decoded.offset"])
						assert.Equal(t, []string{id + "-first", id + "-second"}[j], attrs["decoded.payload"])
						value, present := attrs["kafka.header.tenant"]
						assert.Equal(t, extract, present)
						if extract {
							assert.Equal(t, "tenant-"+id, value)
						}
					}
					if signal == "profiles" {
						assert.Equal(t, []string{id + "-first", id + "-second"}, message.profileTypes,
							"each batch's dictionary references must retain their original meaning")
					}
				}
				assert.Empty(t, received, "each Kafka message must produce a single consumer call")
			})
		}
	}
}

func TestReceiver_StreamingDecoderError(t *testing.T) {
	for name, decodeErr := range map[string]error{
		"invalid message": errors.New("invalid encoded message"),
		"wrapped EOF":     fmt.Errorf("incomplete encoded message: %w", io.EOF),
	} {
		for _, signal := range []string{"logs", "metrics", "traces", "profiles"} {
			for _, construction := range []bool{false, true} {
				t.Run(name+"/"+signal+"/construction="+strconv.FormatBool(construction), func(t *testing.T) {
					_, cfg := mustNewFakeCluster(t)
					ext := &messageContextExtension{}
					if construction {
						ext.newErr = decodeErr
					} else {
						ext.decodeErr = decodeErr
					}
					received := make(chan decodedMessage, 2)
					r := newDecoderTestReceiver(t, signal, cfg, received)
					require.NoError(t, r.Start(t.Context(), extensionsHost{component.MustNewID("context"): ext}))
					t.Cleanup(func() { assert.NoError(t, r.Shutdown(context.Background())) }) //nolint:usetesting
					err := r.(*franzConsumer).consumeMessage(t.Context(), &kgo.Record{
						Topic: "messages", Value: []byte("valid-first-batch"),
					}, attribute.NewSet())
					require.ErrorIs(t, err, decodeErr)
					assert.True(t, consumererror.IsPermanent(err))
					assert.Empty(t, received, "a message must not be partially consumed if decoding fails")
				})
			}
		}
	}
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
				assert.Empty(t, received, "legacy unmarshal errors must not be interpreted as successful stream completion")
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
	ctx          context.Context
	count        int
	resources    []map[string]any
	profileTypes []string
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
				for _, scope := range resource.ScopeProfiles().All() {
					for _, profile := range scope.Profiles().All() {
						result.profileTypes = append(result.profileTypes, data.Dictionary().StringTable().At(int(profile.SampleType().TypeStrindex())))
					}
				}
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

// messageContextExtension supports only the native streaming contracts. Each
// factory call captures a different message context and payload in a new decoder.
type messageContextExtension struct {
	component.Component
	created   chan<- struct{}
	ready     <-chan struct{}
	newErr    error
	decodeErr error
}

func (e *messageContextExtension) newDecoder(reader io.Reader, options ...encoding.DecoderOption) (*messageContextDecoder, error) {
	if e.newErr != nil {
		return nil, e.newErr
	}
	payload, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	d := &messageContextDecoder{
		ctx: encoding.NewDecoderOptions(options...).Context, batches: strings.Split(string(payload), "\n"),
		ready: e.ready, decodeErr: e.decodeErr,
	}
	if e.created != nil {
		e.created <- struct{}{}
	}
	return d, nil
}

func (e *messageContextExtension) NewLogsDecoder(reader io.Reader, options ...encoding.DecoderOption) (encoding.LogsDecoder, error) {
	return e.newDecoder(reader, options...)
}

func (e *messageContextExtension) NewMetricsDecoder(reader io.Reader, options ...encoding.DecoderOption) (encoding.MetricsDecoder, error) {
	return e.newDecoder(reader, options...)
}

func (e *messageContextExtension) NewTracesDecoder(reader io.Reader, options ...encoding.DecoderOption) (encoding.TracesDecoder, error) {
	return e.newDecoder(reader, options...)
}

func (e *messageContextExtension) NewProfilesDecoder(reader io.Reader, options ...encoding.DecoderOption) (encoding.ProfilesDecoder, error) {
	return e.newDecoder(reader, options...)
}

type messageContextDecoder struct {
	ctx       context.Context
	batches   []string
	next      int
	ready     <-chan struct{}
	decodeErr error
}

func (d *messageContextDecoder) nextResource(resource pcommon.Resource) error {
	if d.ready != nil {
		select {
		case <-d.ready:
		case <-d.ctx.Done():
			return d.ctx.Err()
		}
	}
	if d.next == len(d.batches) {
		if d.decodeErr != nil {
			return d.decodeErr
		}
		return io.EOF
	}
	meta := client.FromContext(d.ctx).Metadata
	attrs := resource.Attributes()
	attrs.PutStr("decoded.tenant", strings.Join(meta.Get("tenant"), ","))
	attrs.PutStr("decoded.tags", strings.Join(meta.Get("tag"), ","))
	attrs.PutStr("decoded.topic", strings.Join(meta.Get("kafka.topic"), ","))
	attrs.PutStr("decoded.partition", strings.Join(meta.Get("kafka.partition"), ","))
	attrs.PutStr("decoded.offset", strings.Join(meta.Get("kafka.offset"), ","))
	attrs.PutStr("decoded.payload", d.batches[d.next])
	d.next++
	return nil
}

func (d *messageContextDecoder) DecodeLogs() (plog.Logs, error) {
	data := plog.NewLogs()
	resource := data.ResourceLogs().AppendEmpty()
	if err := d.nextResource(resource.Resource()); err != nil {
		return plog.Logs{}, err
	}
	resource.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
	return data, nil
}

func (d *messageContextDecoder) DecodeMetrics() (pmetric.Metrics, error) {
	data := pmetric.NewMetrics()
	resource := data.ResourceMetrics().AppendEmpty()
	if err := d.nextResource(resource.Resource()); err != nil {
		return pmetric.Metrics{}, err
	}
	resource.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty().SetEmptyGauge().DataPoints().AppendEmpty()
	return data, nil
}

func (d *messageContextDecoder) DecodeTraces() (ptrace.Traces, error) {
	data := ptrace.NewTraces()
	resource := data.ResourceSpans().AppendEmpty()
	if err := d.nextResource(resource.Resource()); err != nil {
		return ptrace.Traces{}, err
	}
	resource.ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	return data, nil
}

func (d *messageContextDecoder) DecodeProfiles() (pprofile.Profiles, error) {
	data := testdata.GenerateProfiles(1)
	resource := data.ResourceProfiles().At(0)
	if err := d.nextResource(resource.Resource()); err != nil {
		return pprofile.Profiles{}, err
	}
	// Both batches use the same index for different strings, so the receiver
	// must merge and remap dictionary references when accumulating the batches.
	idx := data.Dictionary().StringTable().Len()
	data.Dictionary().StringTable().Append(d.batches[d.next-1])
	resource.ScopeProfiles().At(0).Profiles().At(0).SampleType().SetTypeStrindex(int32(idx))
	return data, nil
}

func (d *messageContextDecoder) Offset() int64 { return int64(d.next) }
