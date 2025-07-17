// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package googlecloudpubsubreceiver

import (
	"context"
	"testing"
	"time"

	pb "cloud.google.com/go/pubsub/apiv1/pubsubpb"
	"cloud.google.com/go/pubsub/pstest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudpubsubreceiver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudpubsubreceiver/testdata"
)

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
		},
	}
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
	ctx := context.Background()
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
	ctx := context.Background()
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

	time.Sleep(1 * time.Second)

	// Test an OTLP trace message
	traceSink.Reset()
	srv.Publish("projects/my-project/topics/otlp", testdata.CreateTraceExport(), map[string]string{
		"ce-type":      "org.opentelemetry.otlp.traces.v1",
		"content-type": "application/protobuf",
	})
	assert.Eventually(t, func() bool {
		return len(traceSink.AllTraces()) == 1
	}, 100*time.Second, 10*time.Millisecond)

	// Test an OTLP metric message
	metricSink.Reset()
	srv.Publish("projects/my-project/topics/otlp", testdata.CreateMetricExport(), map[string]string{
		"ce-type":      "org.opentelemetry.otlp.metrics.v1",
		"content-type": "application/protobuf",
	})
	assert.Eventually(t, func() bool {
		return len(metricSink.AllMetrics()) == 1
	}, time.Second, 10*time.Millisecond)

	// Test an OTLP log message
	logSink.Reset()
	srv.Publish("projects/my-project/topics/otlp", testdata.CreateLogExport(), map[string]string{
		"ce-type":      "org.opentelemetry.otlp.logs.v1",
		"content-type": "application/protobuf",
	})
	assert.Eventually(t, func() bool {
		return len(logSink.AllLogs()) == 1
	}, time.Second, 10*time.Millisecond)

	// Test a plain log message
	logSink.Reset()
	srv.Publish("projects/my-project/topics/otlp", testdata.CreateTextExport(), map[string]string{
		"content-type": "text/plain",
	})
	assert.Eventually(t, func() bool {
		return len(logSink.AllLogs()) == 1
	}, time.Second, 10*time.Millisecond)

	assert.NoError(t, receiver.Shutdown(ctx))
	assert.NoError(t, receiver.Shutdown(ctx))
}

func TestEncodingMultipleConsumersForAnEncoding(t *testing.T) {
	ctx := context.Background()
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
	ctx := context.Background()
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
	ctx := context.Background()
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
	ctx := context.Background()
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
	ctx := context.Background()
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
	ctx := context.Background()
	srv, receiver := createBaseReceiver()
	defer func() {
		assert.NoError(t, srv.Close())
		assert.NoError(t, receiver.Shutdown(ctx))
	}()

	receiver.tracesConsumer = consumertest.NewNop()
	receiver.config.Encoding = "foo"
	assert.ErrorContains(t, receiver.Start(ctx, fakeHost{}), "extension \"foo\" not found")
}

func TestEncodingExtension(t *testing.T) {
	ctx := context.Background()
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
	ctx := context.Background()
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
