// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package googlecloudpubsubreceiver

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	pb "cloud.google.com/go/pubsub/v2/apiv1/pubsubpb"
	"cloud.google.com/go/pubsub/v2/pstest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudpubsubreceiver/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudpubsubreceiver/internal/metadata"
)

func createTraceExport() []byte {
	out := ptrace.NewTraces()
	resources := out.ResourceSpans()
	resource := resources.AppendEmpty()
	libs := resource.ScopeSpans()
	spans := libs.AppendEmpty().Spans()
	span := spans.AppendEmpty()
	span.SetName("test")
	marshaler := ptrace.ProtoMarshaler{}
	data, _ := marshaler.MarshalTraces(out)
	return data
}

func createMetricExport() []byte {
	out := pmetric.NewMetrics()
	resources := out.ResourceMetrics()
	resource := resources.AppendEmpty()
	libs := resource.ScopeMetrics()
	metrics := libs.AppendEmpty().Metrics()
	metric := metrics.AppendEmpty()
	metric.SetName("test")
	marshaler := pmetric.ProtoMarshaler{}
	data, _ := marshaler.MarshalMetrics(out)
	return data
}

func createLogExport() []byte {
	out := plog.NewLogs()
	resources := out.ResourceLogs()
	resource := resources.AppendEmpty()
	libs := resource.ScopeLogs()
	logs := libs.AppendEmpty()
	logs.LogRecords().AppendEmpty()
	marshaler := plog.ProtoMarshaler{}
	data, _ := marshaler.MarshalLogs(out)
	return data
}

func createBaseReceiver() (*pstest.Server, *pubsubReceiver) {
	srv := pstest.NewServer()
	settings := receivertest.NewNopSettings(metadata.Type)
	return srv, &pubsubReceiver{
		settings:  settings,
		userAgent: "test-user-agent",

		config: &Config{
			Endpoint:  srv.Addr,
			Insecure:  true,
			ProjectID: "my-project",
			TimeoutSettings: exporterhelper.TimeoutConfig{
				Timeout: 12 * time.Second,
			},
			Subscription: "projects/my-project/subscriptions/otlp",
			FlowControlConfig: FlowControlConfig{
				StreamAckDeadline:       60 * time.Second,
				TriggerAckBatchDuration: 10 * time.Second,
			},
		},
	}
}

// createObservedReceiver creates a receiver whose logger captures log entries
// for assertion in tests.
func createObservedReceiver(t *testing.T, srv *pstest.Server) (*pubsubReceiver, *observer.ObservedLogs) {
	t.Helper()
	core, logs := observer.New(zapcore.DebugLevel)
	settings := receivertest.NewNopSettings(metadata.Type)
	settings.Logger = zap.New(core)
	telemetryBuilder, err := metadata.NewTelemetryBuilder(settings.TelemetrySettings)
	require.NoError(t, err)
	return &pubsubReceiver{
		settings:         settings,
		userAgent:        "test-user-agent",
		telemetryBuilder: telemetryBuilder,
		config: &Config{
			Endpoint:  srv.Addr,
			Insecure:  true,
			ProjectID: "my-project",
			TimeoutSettings: exporterhelper.TimeoutConfig{
				Timeout: 12 * time.Second,
			},
			Subscription: "projects/my-project/subscriptions/otlp",
		},
	}, logs
}

type fakeUnmarshalLog struct{}

func (fakeUnmarshalLog) Start(_ context.Context, _ component.Host) error {
	return nil
}

func (fakeUnmarshalLog) Shutdown(_ context.Context) error {
	return nil
}

func (fakeUnmarshalLog) UnmarshalLogs(_ []byte) (plog.Logs, error) {
	return plog.Logs{}, nil
}

type fakeHost struct{}

func (fakeHost) GetExtensions() map[component.ID]component.Component {
	ext := make(map[component.ID]component.Component)
	extensionID := component.ID{}
	_ = extensionID.UnmarshalText([]byte("text_encoding"))
	ext[extensionID] = fakeUnmarshalLog{}
	return ext
}

func TestStartReceiverNoSubscription(t *testing.T) {
	ctx := t.Context()
	srv, receiver := createBaseReceiver()
	defer func() {
		assert.NoError(t, srv.Close())
		assert.NoError(t, receiver.Shutdown(ctx))
	}()

	receiver.tracesConsumer = consumertest.NewNop()
	receiver.metricsConsumer = consumertest.NewNop()
	receiver.logsConsumer = consumertest.NewNop()
	// No error is thrown as the stream is handled async,
	// no locks should be kept though
	assert.NoError(t, receiver.Start(ctx, fakeHost{}))
}

func TestReceiver(t *testing.T) {
	ctx := t.Context()
	// Start a fake server running locally.
	srv := pstest.NewServer()
	defer srv.Close()
	_, err := srv.GServer.CreateTopic(ctx, &pb.Topic{
		Name: "projects/my-project/topics/otlp",
	})
	assert.NoError(t, err)
	_, err = srv.GServer.CreateSubscription(ctx, &pb.Subscription{
		Topic:              "projects/my-project/topics/otlp",
		Name:               "projects/my-project/subscriptions/otlp",
		AckDeadlineSeconds: 10,
	})
	assert.NoError(t, err)

	settings := receivertest.NewNopSettings(metadata.Type)
	traceSink := new(consumertest.TracesSink)
	metricSink := new(consumertest.MetricsSink)
	logSink := new(consumertest.LogsSink)

	obsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{
		ReceiverID:             component.NewID(metadata.Type),
		Transport:              reportTransport,
		LongLivedCtx:           false,
		ReceiverCreateSettings: settings,
	})
	require.NoError(t, err)

	receiver := &pubsubReceiver{
		settings:  settings,
		obsrecv:   obsrecv,
		userAgent: "test-user-agent",

		config: &Config{
			Endpoint:  srv.Addr,
			Insecure:  true,
			ProjectID: "my-project",
			TimeoutSettings: exporterhelper.TimeoutConfig{
				Timeout: 1 * time.Second,
			},
			Subscription: "projects/my-project/subscriptions/otlp",
			FlowControlConfig: FlowControlConfig{
				StreamAckDeadline:       60 * time.Second,
				TriggerAckBatchDuration: 10 * time.Second,
			},
		},
		tracesConsumer:  traceSink,
		metricsConsumer: metricSink,
		logsConsumer:    logSink,
	}
	assert.NoError(t, receiver.Start(ctx, fakeHost{}))

	receiver.tracesConsumer = traceSink
	receiver.metricsConsumer = metricSink
	receiver.logsConsumer = logSink
	// No error is thrown as the stream is handled async,
	// no locks should be kept though
	assert.NoError(t, receiver.Start(ctx, fakeHost{}))

	// Test an OTLP trace message
	traceSink.Reset()
	srv.Publish("projects/my-project/topics/otlp", createTraceExport(), map[string]string{
		"ce-type":      "org.opentelemetry.otlp.traces.v1",
		"content-type": "application/protobuf",
	})
	assert.Eventually(t, func() bool {
		return len(traceSink.AllTraces()) == 1
	}, 30*time.Second, 10*time.Millisecond)

	// Test an OTLP metric message
	metricSink.Reset()
	srv.Publish("projects/my-project/topics/otlp", createMetricExport(), map[string]string{
		"ce-type":      "org.opentelemetry.otlp.metrics.v1",
		"content-type": "application/protobuf",
	})
	assert.Eventually(t, func() bool {
		return len(metricSink.AllMetrics()) == 1
	}, 30*time.Second, 10*time.Millisecond)

	// Test an OTLP log message
	logSink.Reset()
	srv.Publish("projects/my-project/topics/otlp", createLogExport(), map[string]string{
		"ce-type":      "org.opentelemetry.otlp.logs.v1",
		"content-type": "application/protobuf",
	})
	assert.Eventually(t, func() bool {
		return len(logSink.AllLogs()) == 1
	}, 30*time.Second, 10*time.Millisecond)

	assert.NoError(t, receiver.Shutdown(ctx))
	assert.NoError(t, receiver.Shutdown(ctx))
}

func TestEncodingMultipleConsumersForAnEncoding(t *testing.T) {
	ctx := t.Context()
	srv, receiver := createBaseReceiver()
	defer func() {
		assert.NoError(t, srv.Close())
		assert.NoError(t, receiver.Shutdown(ctx))
	}()

	receiver.tracesConsumer = consumertest.NewNop()
	receiver.metricsConsumer = consumertest.NewNop()
	receiver.logsConsumer = consumertest.NewNop()
	receiver.config.Encoding = "foo"
	assert.ErrorContains(t, receiver.Start(ctx, fakeHost{}), "multiple consumers were attached")
}

func TestEncodingBuildInProtoTrace(t *testing.T) {
	ctx := t.Context()
	srv, receiver := createBaseReceiver()
	defer func() {
		assert.NoError(t, srv.Close())
		assert.NoError(t, receiver.Shutdown(ctx))
	}()

	receiver.tracesConsumer = consumertest.NewNop()
	receiver.config.Encoding = "otlp_proto_trace"

	assert.NoError(t, receiver.Start(ctx, fakeHost{}))
	assert.NotNil(t, receiver.tracesConsumer)
	assert.Nil(t, receiver.metricsConsumer)
	assert.Nil(t, receiver.logsConsumer)
}

func TestEncodingBuildInProtoMetric(t *testing.T) {
	ctx := t.Context()
	srv, receiver := createBaseReceiver()
	defer func() {
		assert.NoError(t, srv.Close())
		assert.NoError(t, receiver.Shutdown(ctx))
	}()

	receiver.metricsConsumer = consumertest.NewNop()
	receiver.config.Encoding = "otlp_proto_metric"

	assert.NoError(t, receiver.Start(ctx, fakeHost{}))
	assert.Nil(t, receiver.tracesConsumer)
	assert.NotNil(t, receiver.metricsConsumer)
	assert.Nil(t, receiver.logsConsumer)
}

func TestEncodingBuildInProtoLog(t *testing.T) {
	ctx := t.Context()
	srv, receiver := createBaseReceiver()
	defer func() {
		assert.NoError(t, srv.Close())
		assert.NoError(t, receiver.Shutdown(ctx))
	}()

	receiver.logsConsumer = consumertest.NewNop()
	receiver.config.Encoding = "otlp_proto_log"

	assert.NoError(t, receiver.Start(ctx, fakeHost{}))
	assert.Nil(t, receiver.tracesConsumer)
	assert.Nil(t, receiver.metricsConsumer)
	assert.NotNil(t, receiver.logsConsumer)
}

func TestEncodingConsumerMismatch(t *testing.T) {
	ctx := t.Context()
	srv, receiver := createBaseReceiver()
	defer func() {
		assert.NoError(t, srv.Close())
		assert.NoError(t, receiver.Shutdown(ctx))
	}()

	receiver.tracesConsumer = consumertest.NewNop()
	receiver.config.Encoding = "otlp_proto_log"

	assert.ErrorContains(t, receiver.Start(ctx, fakeHost{}), "build in encoding otlp_proto_log is not supported for traces")
}

func TestEncodingNotFound(t *testing.T) {
	ctx := t.Context()
	srv, receiver := createBaseReceiver()
	defer func() {
		assert.NoError(t, srv.Close())
		assert.NoError(t, receiver.Shutdown(ctx))
	}()

	receiver.tracesConsumer = consumertest.NewNop()
	receiver.config.Encoding = "foo"
	assert.ErrorContains(t, receiver.Start(ctx, fakeHost{}), "extension \"foo\" not found")
}

func TestEncodingRemovedRawText(t *testing.T) {
	ctx := t.Context()
	srv, receiver := createBaseReceiver()
	defer func() {
		assert.NoError(t, srv.Close())
		assert.NoError(t, receiver.Shutdown(ctx))
	}()

	receiver.logsConsumer = consumertest.NewNop()
	receiver.config.Encoding = "raw_text"
	assert.ErrorContains(t, receiver.Start(ctx, fakeHost{}), "build-in raw_text encoding is removed since v0.132.0")
}

func TestEncodingRemovedCloudLogging(t *testing.T) {
	ctx := t.Context()
	srv, receiver := createBaseReceiver()
	defer func() {
		assert.NoError(t, srv.Close())
		assert.NoError(t, receiver.Shutdown(ctx))
	}()

	receiver.logsConsumer = consumertest.NewNop()
	receiver.config.Encoding = "cloud_logging"
	assert.ErrorContains(t, receiver.Start(ctx, fakeHost{}), "build-in cloud_logging encoding is removed since v0.132.0")
}

func TestEncodingExtension(t *testing.T) {
	ctx := t.Context()
	srv, receiver := createBaseReceiver()
	defer func() {
		assert.NoError(t, srv.Close())
		assert.NoError(t, receiver.Shutdown(ctx))
	}()

	receiver.tracesConsumer = consumertest.NewNop()
	receiver.config.Encoding = "text_encoding"
	assert.ErrorContains(t, receiver.Start(ctx, fakeHost{}), "extension \"text_encoding\" is not a trace unmarshaler")
}

func TestEncodingExtensionMismatch(t *testing.T) {
	ctx := t.Context()
	srv, receiver := createBaseReceiver()
	defer func() {
		assert.NoError(t, srv.Close())
		assert.NoError(t, receiver.Shutdown(ctx))
	}()

	receiver.logsConsumer = consumertest.NewNop()
	receiver.config.Encoding = "text_encoding"
	assert.NoError(t, receiver.Start(ctx, fakeHost{}))
	assert.Nil(t, receiver.tracesConsumer)
	assert.Nil(t, receiver.metricsConsumer)
	assert.NotNil(t, receiver.logsConsumer)
}

func TestEncodingWithCompressionConfig(t *testing.T) {
	ctx := t.Context()
	srv, receiver := createBaseReceiver()
	defer func() {
		assert.NoError(t, srv.Close())
		assert.NoError(t, receiver.Shutdown(ctx))
	}()

	_, err := srv.GServer.CreateTopic(ctx, &pb.Topic{
		Name: "projects/my-project/topics/otlp",
	})
	assert.NoError(t, err)
	_, err = srv.GServer.CreateSubscription(ctx, &pb.Subscription{
		Topic:              "projects/my-project/topics/otlp",
		Name:               "projects/my-project/subscriptions/otlp",
		AckDeadlineSeconds: 10,
	})
	assert.NoError(t, err)

	obsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{
		ReceiverID:             component.NewID(metadata.Type),
		Transport:              reportTransport,
		LongLivedCtx:           false,
		ReceiverCreateSettings: receiver.settings,
	})
	require.NoError(t, err)

	traceSink := new(consumertest.TracesSink)
	receiver.obsrecv = obsrecv
	receiver.config.Encoding = "otlp_proto_trace"
	receiver.config.Compression = "gzip"
	receiver.tracesConsumer = traceSink
	assert.NoError(t, receiver.Start(ctx, fakeHost{}))

	// Publish a gzip-compressed trace message
	traceData := createTraceExport()
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	_, _ = w.Write(traceData)
	_ = w.Close()
	srv.Publish("projects/my-project/topics/otlp", buf.Bytes(), map[string]string{})

	assert.Eventually(t, func() bool {
		return len(traceSink.AllTraces()) == 1
	}, 30*time.Second, 10*time.Millisecond)
}

func invalidMessage(messageID string) *pb.ReceivedMessage {
	return &pb.ReceivedMessage{
		AckId: "ack-" + messageID,
		Message: &pb.PubsubMessage{
			MessageId:  messageID,
			Data:       []byte("this is not valid protobuf"),
			Attributes: map[string]string{"env": "test"},
		},
	}
}

// TestHandleEncodingErrorWarnLog verifies that a warning containing the pubsub
// message context (signal, message_id, total count, and error) is emitted when
// an unmarshaler returns an error, for each signal.
func TestHandleEncodingErrorWarnLog(t *testing.T) {
	tests := []struct {
		signal string
		handle func(recv *pubsubReceiver, ctx context.Context, msg *pb.ReceivedMessage) error
	}{
		{
			signal: "logs",
			handle: func(recv *pubsubReceiver, ctx context.Context, msg *pb.ReceivedMessage) error {
				recv.logsConsumer = consumertest.NewNop()
				recv.logsUnmarshaler = &plog.ProtoUnmarshaler{}
				return recv.handleLog(ctx, msg, uncompressed)
			},
		},
		{
			signal: "traces",
			handle: func(recv *pubsubReceiver, ctx context.Context, msg *pb.ReceivedMessage) error {
				recv.tracesConsumer = consumertest.NewNop()
				recv.tracesUnmarshaler = &ptrace.ProtoUnmarshaler{}
				return recv.handleTrace(ctx, msg, uncompressed)
			},
		},
		{
			signal: "metrics",
			handle: func(recv *pubsubReceiver, ctx context.Context, msg *pb.ReceivedMessage) error {
				recv.metricsConsumer = consumertest.NewNop()
				recv.metricsUnmarshaler = &pmetric.ProtoUnmarshaler{}
				return recv.handleMetric(ctx, msg, uncompressed)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.signal, func(t *testing.T) {
			srv := pstest.NewServer()
			defer srv.Close()

			recv, logs := createObservedReceiver(t, srv)
			msg := invalidMessage("msg-" + tt.signal + "-001")

			// The default policy acknowledges and drops, so no error is returned
			assert.NoError(t, tt.handle(recv, t.Context(), msg))

			warnLogs := logs.FilterLevelExact(zapcore.WarnLevel)
			require.Equal(t, 1, warnLogs.Len(), "expected exactly one warn log entry on encoding error")
			entry := warnLogs.All()[0]
			assert.Equal(t, "failed to decode pubsub message", entry.Message)
			assert.Equal(t, tt.signal, entry.ContextMap()["signal"])
			assert.Equal(t, "msg-"+tt.signal+"-001", entry.ContextMap()["message_id"])
			assert.Equal(t, int64(1), entry.ContextMap()["total_failed"])
		})
	}
}

// TestHandleEncodingErrorWarnRateLimited verifies that a burst of decode
// failures emits a single warning, and that the failures keep being counted.
func TestHandleEncodingErrorWarnRateLimited(t *testing.T) {
	ctx := t.Context()
	srv := pstest.NewServer()
	defer srv.Close()

	recv, logs := createObservedReceiver(t, srv)
	recv.logsConsumer = consumertest.NewNop()
	recv.logsUnmarshaler = &plog.ProtoUnmarshaler{}

	for i := range 10 {
		assert.NoError(t, recv.handleLog(ctx, invalidMessage(fmt.Sprintf("msg-%d", i)), uncompressed))
	}

	warnLogs := logs.FilterLevelExact(zapcore.WarnLevel)
	require.Equal(t, 1, warnLogs.Len(), "expected the decode warning to be rate limited to one entry")
	assert.Equal(t, "msg-0", warnLogs.All()[0].ContextMap()["message_id"])
	assert.Equal(t, int64(10), recv.decodeFailures.Load())
}

// TestHandleLogEncodingErrorIgnored verifies that when IgnoreEncodingError is
// true, the error is silently dropped AND the warning is still emitted.
func TestHandleLogEncodingErrorIgnored(t *testing.T) {
	ctx := t.Context()
	srv := pstest.NewServer()
	defer srv.Close()

	recv, logs := createObservedReceiver(t, srv)
	recv.logsConsumer = consumertest.NewNop()
	recv.logsUnmarshaler = &plog.ProtoUnmarshaler{}
	recv.config.IgnoreEncodingError = true

	// No error returned when IgnoreEncodingError is true
	assert.NoError(t, recv.handleLog(ctx, invalidMessage("msg-ignored-001"), uncompressed))

	// But the warning should still have been emitted
	warnLogs := logs.FilterLevelExact(zapcore.WarnLevel)
	require.Equal(t, 1, warnLogs.Len(), "expected warn log even when error is ignored")
	entry := warnLogs.All()[0]
	assert.Equal(t, "failed to decode pubsub message", entry.Message)
	assert.Equal(t, "msg-ignored-001", entry.ContextMap()["message_id"])
}

// TestDecodeErrorPolicies verifies the on_decode_error policies, including the
// backwards compatible mapping of ignore_encoding_error.
func TestDecodeErrorPolicies(t *testing.T) {
	tests := []struct {
		name                string
		onDecodeError       string
		ignoreEncodingError bool
		wantErr             error
	}{
		{name: "default ignores (ack and drop)", wantErr: nil},
		{name: "propagate", onDecodeError: "propagate", wantErr: assert.AnError},
		{name: "ignore", onDecodeError: "ignore", wantErr: nil},
		{name: "nack", onDecodeError: "nack", wantErr: internal.ErrNack},
		{name: "legacy ignore_encoding_error maps to ignore", ignoreEncodingError: true, wantErr: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := pstest.NewServer()
			defer srv.Close()

			recv, _ := createObservedReceiver(t, srv)
			recv.logsConsumer = consumertest.NewNop()
			recv.logsUnmarshaler = &plog.ProtoUnmarshaler{}
			recv.config.OnDecodeError = tt.onDecodeError
			recv.config.IgnoreEncodingError = tt.ignoreEncodingError

			err := recv.handleLog(t.Context(), invalidMessage("msg-policy-001"), uncompressed)
			switch {
			case tt.wantErr == nil:
				assert.NoError(t, err)
			case errors.Is(tt.wantErr, internal.ErrNack):
				assert.ErrorIs(t, err, internal.ErrNack)
			default:
				assert.Error(t, err)
				assert.NotErrorIs(t, err, internal.ErrNack)
			}
		})
	}
}

// TestPipelineErrorPolicies verifies the on_pipeline_error policies.
func TestPipelineErrorPolicies(t *testing.T) {
	tests := []struct {
		name            string
		onPipelineError string
		consumerErr     error
		wantNack        bool
		wantErr         bool
	}{
		{name: "transient error is neither acked nor nacked", consumerErr: assert.AnError, wantErr: true},
		{name: "transient error is neither acked nor nacked with nack policy", onPipelineError: "nack", consumerErr: assert.AnError, wantErr: true},
		{name: "default acks on permanent pipeline error", consumerErr: consumererror.NewPermanent(assert.AnError)},
		{name: "ack acks on permanent pipeline error", onPipelineError: "ack", consumerErr: consumererror.NewPermanent(assert.AnError)},
		{name: "nack nacks on permanent pipeline error", onPipelineError: "nack", consumerErr: consumererror.NewPermanent(assert.AnError), wantNack: true},
		{name: "nack never nacks on context cancellation", onPipelineError: "nack", consumerErr: context.Canceled, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := pstest.NewServer()
			defer srv.Close()

			recv, _ := createObservedReceiver(t, srv)
			recv.logsConsumer = consumertest.NewErr(tt.consumerErr)
			recv.logsUnmarshaler = &plog.ProtoUnmarshaler{}
			recv.config.OnPipelineError = tt.onPipelineError

			obsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{
				ReceiverID:             component.NewID(metadata.Type),
				Transport:              reportTransport,
				LongLivedCtx:           false,
				ReceiverCreateSettings: recv.settings,
			})
			require.NoError(t, err)
			recv.obsrecv = obsrecv

			msg := &pb.ReceivedMessage{
				AckId: "ack-pipeline",
				Message: &pb.PubsubMessage{
					MessageId: "msg-pipeline-001",
					Data:      createLogExport(),
				},
			}
			err = recv.handleLog(t.Context(), msg, uncompressed)
			switch {
			case tt.wantNack:
				assert.ErrorIs(t, err, internal.ErrNack)
			case tt.wantErr:
				assert.Error(t, err)
				assert.NotErrorIs(t, err, internal.ErrNack)
			default:
				assert.NoError(t, err)
			}
		})
	}
}

// TestUndeliverableMessagePolicies verifies that a message the multiplexing
// handler cannot deliver - an unknown encoding, or a signal without a consumer -
// follows the on_decode_error policy instead of silently redelivering forever
// (propagate) or vanishing unaccounted (the previous bare ack).
func TestUndeliverableMessagePolicies(t *testing.T) {
	tests := []struct {
		name          string
		attributes    map[string]string
		onDecodeError string
		wantErr       error
		wantSignal    string
	}{
		{
			name:       "unknown encoding is ignored by default",
			attributes: map[string]string{"ce-type": "com.example.unknown.v1"},
			wantErr:    nil,
			wantSignal: "unknown",
		},
		{
			name:          "unknown encoding nacks under nack policy",
			attributes:    map[string]string{"ce-type": "com.example.unknown.v1"},
			onDecodeError: "nack",
			wantErr:       internal.ErrNack,
			wantSignal:    "unknown",
		},
		{
			name: "signal without consumer is ignored by default",
			attributes: map[string]string{
				"ce-type":      "org.opentelemetry.otlp.traces.v1",
				"content-type": "application/protobuf",
			},
			wantErr:    nil,
			wantSignal: "traces",
		},
		{
			name: "signal without consumer nacks under nack policy",
			attributes: map[string]string{
				"ce-type":      "org.opentelemetry.otlp.traces.v1",
				"content-type": "application/protobuf",
			},
			onDecodeError: "nack",
			wantErr:       internal.ErrNack,
			wantSignal:    "traces",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := pstest.NewServer()
			defer srv.Close()

			recv, logs := createObservedReceiver(t, srv)
			// Only a logs consumer is attached, so a trace message has no consumer.
			recv.logsConsumer = consumertest.NewNop()
			recv.logsUnmarshaler = &plog.ProtoUnmarshaler{}
			recv.config.OnDecodeError = tt.onDecodeError

			obsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{
				ReceiverID:             component.NewID(metadata.Type),
				Transport:              reportTransport,
				LongLivedCtx:           false,
				ReceiverCreateSettings: recv.settings,
			})
			require.NoError(t, err)
			recv.obsrecv = obsrecv

			msg := &pb.ReceivedMessage{
				AckId:   "ack-undeliverable",
				Message: &pb.PubsubMessage{MessageId: "msg-undeliverable-001", Attributes: tt.attributes},
			}
			err = recv.handleMultiplexedMessage(t.Context(), msg)
			if tt.wantErr == nil {
				assert.NoError(t, err)
			} else {
				assert.ErrorIs(t, err, tt.wantErr)
			}

			warnLogs := logs.FilterLevelExact(zapcore.WarnLevel)
			require.Equal(t, 1, warnLogs.Len(), "expected one warn log for the undeliverable message")
			assert.Equal(t, tt.wantSignal, warnLogs.All()[0].ContextMap()["signal"])
		})
	}
}
