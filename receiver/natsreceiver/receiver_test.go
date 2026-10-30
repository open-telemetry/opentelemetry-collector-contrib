// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package natsreceiver

import (
	"context"
	"errors"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/nats-io/nats-server/v2/server"
	natstest "github.com/nats-io/nats-server/v2/test"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pdata/testdata"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/natsclient"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/natsreceiver/internal/metadata"
)

// runServer starts an embedded nats-server on a random port for a test.
func runServer(t *testing.T) string {
	t.Helper()
	opts := natstest.DefaultTestOptions
	opts.Port = -1 // random free port
	srv := natstest.RunServer(&opts)
	t.Cleanup(srv.Shutdown)
	return srv.ClientURL()
}

// connect opens a plain client connection used to publish test messages.
func connect(t *testing.T, url string) *nats.Conn {
	t.Helper()
	nc, err := nats.Connect(url)
	require.NoError(t, err)
	t.Cleanup(nc.Close)
	return nc
}

func newTestConfig(url string) *Config {
	cfg := createDefaultConfig().(*Config)
	cfg.ClientConfig.Endpoint = url
	return cfg
}

func newTestReceiver(cfg *Config) *natsReceiver {
	return newReceiver(receivertest.NewNopSettings(metadata.Type), cfg)
}

func publish(t *testing.T, nc *nats.Conn, subject string, data []byte) {
	t.Helper()
	require.NoError(t, nc.Publish(subject, data))
	require.NoError(t, nc.Flush())
}

func marshalLogs(t *testing.T, ld plog.Logs) []byte {
	t.Helper()
	data, err := (&plog.ProtoMarshaler{}).MarshalLogs(ld)
	require.NoError(t, err)
	return data
}

func TestReceiver_CoreNATS(t *testing.T) {
	t.Parallel()

	url := runServer(t)
	nc := connect(t, url)

	cfg := newTestConfig(url)
	cfg.Logs.Subject = "otel.logs.>"
	cfg.Metrics.Encoding = encodingOTLPJSON
	r := newTestReceiver(cfg)
	logs, metrics, traces := new(consumertest.LogsSink), new(consumertest.MetricsSink), new(consumertest.TracesSink)
	r.nextLogs, r.nextMetrics, r.nextTraces = logs, metrics, traces

	require.NoError(t, r.Start(t.Context(), componenttest.NewNopHost()))
	defer func() { require.NoError(t, r.Shutdown(t.Context())) }()

	ld := testdata.GenerateLogs(2)
	md := testdata.GenerateMetrics(2)
	td := testdata.GenerateTraces(2)
	mdJSON, err := (&pmetric.JSONMarshaler{}).MarshalMetrics(md)
	require.NoError(t, err)
	tdProto, err := (&ptrace.ProtoMarshaler{}).MarshalTraces(td)
	require.NoError(t, err)
	publish(t, nc, "otel.logs.app", marshalLogs(t, ld))
	publish(t, nc, "otel_metrics", mdJSON)
	publish(t, nc, "otel_traces", tdProto)

	require.Eventually(t, func() bool {
		return logs.LogRecordCount() == 2 && metrics.DataPointCount() == md.DataPointCount() && traces.SpanCount() == 2
	}, 5*time.Second, 10*time.Millisecond)
	assert.Equal(t, ld, logs.AllLogs()[0])
	assert.Equal(t, md, metrics.AllMetrics()[0])
	assert.Equal(t, td, traces.AllTraces()[0])
}

// TestReceiver_OnlyUsedSignals verifies that only signals wired into a pipeline
// are subscribed, so an unused signal's settings cannot fail Start.
func TestReceiver_OnlyUsedSignals(t *testing.T) {
	t.Parallel()

	url := runServer(t)
	cfg := newTestConfig(url)
	cfg.Traces.EncodingExtension = "missing"
	r := newTestReceiver(cfg)
	r.nextLogs = consumertest.NewNop()

	require.NoError(t, r.Start(t.Context(), componenttest.NewNopHost()))
	defer func() { require.NoError(t, r.Shutdown(t.Context())) }()
	assert.Equal(t, 1, r.conn.NumSubscriptions())
	assert.Equal(t, "receiver/"+r.settings.ID.String(), r.conn.Opts.Name)
}

func TestReceiver_QueueGroup(t *testing.T) {
	t.Parallel()

	url := runServer(t)
	nc := connect(t, url)

	// Two members of one queue group split the messages instead of each
	// receiving all of them.
	var sinks [2]consumertest.LogsSink
	for i := range sinks {
		cfg := newTestConfig(url)
		cfg.Logs.QueueGroup = "collectors"
		r := newTestReceiver(cfg)
		r.nextLogs = &sinks[i]
		require.NoError(t, r.Start(t.Context(), componenttest.NewNopHost()))
		defer func() { require.NoError(t, r.Shutdown(t.Context())) }()
	}

	payload := marshalLogs(t, testdata.GenerateLogs(1))
	const n = 20
	for range n {
		publish(t, nc, "otel_logs", payload)
	}

	require.Eventually(t, func() bool {
		return sinks[0].LogRecordCount()+sinks[1].LogRecordCount() == n
	}, 5*time.Second, 10*time.Millisecond)
	// Nothing beyond the n published messages arrives.
	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, n, sinks[0].LogRecordCount()+sinks[1].LogRecordCount())
}

// TestReceiver_DropsFailedMessages verifies, for every signal, that a payload
// that fails to unmarshal and one the pipeline rejects are each logged with
// their subject and dropped.
func TestReceiver_DropsFailedMessages(t *testing.T) {
	t.Parallel()

	url := runServer(t)
	nc := connect(t, url)

	md, err := (&pmetric.ProtoMarshaler{}).MarshalMetrics(testdata.GenerateMetrics(1))
	require.NoError(t, err)
	td, err := (&ptrace.ProtoMarshaler{}).MarshalTraces(testdata.GenerateTraces(1))
	require.NoError(t, err)

	pipelineErr := consumertest.NewErr(errors.New("pipeline unavailable"))
	tests := []struct {
		signal  string
		subject string
		payload []byte
		wire    func(*natsReceiver)
	}{
		{"logs", "otel_logs", marshalLogs(t, testdata.GenerateLogs(1)), func(r *natsReceiver) { r.nextLogs = pipelineErr }},
		{"metrics", "otel_metrics", md, func(r *natsReceiver) { r.nextMetrics = pipelineErr }},
		{"traces", "otel_traces", td, func(r *natsReceiver) { r.nextTraces = pipelineErr }},
	}
	for _, tt := range tests {
		t.Run(tt.signal, func(t *testing.T) {
			t.Parallel()

			core, logs := observer.New(zapcore.ErrorLevel)
			set := receivertest.NewNopSettings(metadata.Type)
			set.Logger = zap.New(core)
			r := newReceiver(set, newTestConfig(url))
			tt.wire(r)
			// A Start deadline exercises the flush that honors it.
			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()
			require.NoError(t, r.Start(ctx, componenttest.NewNopHost()))
			defer func() { require.NoError(t, r.Shutdown(t.Context())) }()

			publish(t, nc, tt.subject, []byte("not otlp"))
			publish(t, nc, tt.subject, tt.payload)

			for _, msg := range []string{"failed to unmarshal message; dropping it", "pipeline rejected message; dropping it"} {
				require.Eventually(t, func() bool { return logs.FilterMessage(msg).Len() == 1 }, 5*time.Second, 10*time.Millisecond, msg)
				assert.Equal(t, tt.subject, logs.FilterMessage(msg).All()[0].ContextMap()["subject"])
			}
		})
	}
}

// TestReceiver_DropsUndecodable verifies that a payload that fails to unmarshal
// is dropped without stopping the subscription.
func TestReceiver_DropsUndecodable(t *testing.T) {
	t.Parallel()

	url := runServer(t)
	nc := connect(t, url)

	r := newTestReceiver(newTestConfig(url))
	sink := new(consumertest.LogsSink)
	r.nextLogs = sink
	require.NoError(t, r.Start(t.Context(), componenttest.NewNopHost()))
	defer func() { require.NoError(t, r.Shutdown(t.Context())) }()

	publish(t, nc, "otel_logs", []byte("not otlp"))
	publish(t, nc, "otel_logs", marshalLogs(t, testdata.GenerateLogs(1)))

	require.Eventually(t, func() bool { return sink.LogRecordCount() == 1 }, 5*time.Second, 10*time.Millisecond)
	assert.Len(t, sink.AllLogs(), 1)
}

func TestReceiver_EncodingExtension(t *testing.T) {
	t.Parallel()

	url := runServer(t)
	nc := connect(t, url)

	cfg := newTestConfig(url)
	cfg.Logs.EncodingExtension = "fake_encoding"
	r := newTestReceiver(cfg)
	sink := new(consumertest.LogsSink)
	r.nextLogs = sink
	host := &extensionHost{exts: map[component.ID]component.Component{
		component.MustNewID("fake_encoding"): &logsEncoding{},
	}}
	require.NoError(t, r.Start(t.Context(), host))
	defer func() { require.NoError(t, r.Shutdown(t.Context())) }()

	publish(t, nc, "otel_logs", []byte("hello"))

	require.Eventually(t, func() bool { return sink.LogRecordCount() == 1 }, 5*time.Second, 10*time.Millisecond)
	body := sink.AllLogs()[0].ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Body()
	assert.Equal(t, "hello", body.Str())
}

func TestReceiver_StartErrors(t *testing.T) {
	t.Parallel()

	url := runServer(t)

	tests := []struct {
		name    string
		mutate  func(*Config)
		wantErr string
	}{
		{
			name:    "jetstream",
			mutate:  func(c *Config) { c.JetStream = &JetStreamConfig{} },
			wantErr: "jetstream consumption is not yet implemented",
		},
		{
			name:    "missing encoding extension",
			mutate:  func(c *Config) { c.Logs.EncodingExtension = "missing" },
			wantErr: "logs: encoding extension not found: missing",
		},
		{
			name:    "extension cannot unmarshal the signal",
			mutate:  func(c *Config) { c.Metrics.EncodingExtension = "fake_encoding" },
			wantErr: "metrics: encoding extension fake_encoding does not support unmarshaling metrics",
		},
		{
			// Passes the client's subject check but the server rejects it and
			// closes the connection.
			name:    "subject rejected by server",
			mutate:  func(c *Config) { c.Logs.Subject = "otel.>.logs" },
			wantErr: "Invalid Subject",
		},
		{
			name:    "subject rejected by client",
			mutate:  func(c *Config) { c.Logs.Subject = "otel logs" },
			wantErr: "invalid subject",
		},
		{
			name:    "unreachable server",
			mutate:  func(c *Config) { c.ClientConfig.Endpoint = "nats://127.0.0.1:1" },
			wantErr: "no servers available",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := newTestConfig(url)
			tt.mutate(cfg)
			r := newTestReceiver(cfg)
			r.nextLogs, r.nextMetrics = consumertest.NewNop(), consumertest.NewNop()
			host := &extensionHost{exts: map[component.ID]component.Component{
				component.MustNewID("fake_encoding"): &logsEncoding{},
			}}

			err := r.Start(t.Context(), host)
			require.ErrorContains(t, err, tt.wantErr)
			assert.Nil(t, r.conn, "a failed Start must not leave a connection open")
			require.NoError(t, r.Shutdown(t.Context()))
		})
	}
}

// TestReceiver_Reconnects verifies that the receiver reports the lost
// connection as a recoverable error and resumes receiving, reporting OK, once
// the NATS server comes back on the same address.
func TestReceiver_Reconnects(t *testing.T) {
	t.Parallel()

	opts := natstest.DefaultTestOptions
	opts.Port = -1 // random free port
	srv := natstest.RunServer(&opts)
	url := srv.ClientURL()
	port := srv.Addr().(*net.TCPAddr).Port

	r := newTestReceiver(newTestConfig(url))
	sink := new(consumertest.LogsSink)
	r.nextLogs = sink
	host := &statusHost{}
	require.NoError(t, r.Start(t.Context(), host))
	defer func() { require.NoError(t, r.Shutdown(t.Context())) }()

	srv.Shutdown()
	require.Eventually(t, func() bool {
		return host.lastStatus() == componentstatus.StatusRecoverableError
	}, 5*time.Second, 10*time.Millisecond)

	opts.Port = port
	srv = natstest.RunServer(&opts)
	t.Cleanup(srv.Shutdown)
	require.Eventually(t, func() bool {
		return host.lastStatus() == componentstatus.StatusOK
	}, 10*time.Second, 10*time.Millisecond)

	nc := connect(t, url)
	publish(t, nc, "otel_logs", marshalLogs(t, testdata.GenerateLogs(1)))
	require.Eventually(t, func() bool { return sink.LogRecordCount() == 1 }, 5*time.Second, 10*time.Millisecond)
}

// TestReceiver_ShutdownDrains verifies that messages already received are
// handed to the pipeline before Shutdown returns.
func TestReceiver_ShutdownDrains(t *testing.T) {
	t.Parallel()

	url := runServer(t)
	nc := connect(t, url)

	r := newTestReceiver(newTestConfig(url))
	sink := new(consumertest.LogsSink)
	r.nextLogs = &slowLogs{Logs: sink, delay: 20 * time.Millisecond}
	require.NoError(t, r.Start(t.Context(), componenttest.NewNopHost()))

	payload := marshalLogs(t, testdata.GenerateLogs(1))
	const n = 5
	for range n {
		publish(t, nc, "otel_logs", payload)
	}

	require.NoError(t, r.Shutdown(t.Context()))
	assert.Equal(t, n, sink.LogRecordCount())
	assert.True(t, r.conn.IsClosed())
}

// TestReceiver_ShutdownTimeout verifies that Shutdown gives up at the ctx
// deadline and cancels the context of the stuck consume.
func TestReceiver_ShutdownTimeout(t *testing.T) {
	t.Parallel()

	url := runServer(t)
	nc := connect(t, url)

	r := newTestReceiver(newTestConfig(url))
	blocked := make(chan struct{})
	r.nextLogs = &blockingLogs{started: blocked}
	require.NoError(t, r.Start(t.Context(), componenttest.NewNopHost()))

	publish(t, nc, "otel_logs", marshalLogs(t, testdata.GenerateLogs(1)))
	<-blocked

	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()
	require.ErrorIs(t, r.Shutdown(ctx), context.DeadlineExceeded)
	assert.True(t, r.conn.IsClosed())
}

// TestReceiver_ShutdownWithoutDeadline verifies that Shutdown is bounded even
// when its ctx has no deadline, and that the consume it aborts is not reported
// as a pipeline error.
func TestReceiver_ShutdownWithoutDeadline(t *testing.T) {
	t.Parallel()

	url := runServer(t)
	nc := connect(t, url)

	core, logs := observer.New(zapcore.DebugLevel)
	set := receivertest.NewNopSettings(metadata.Type)
	set.Logger = zap.New(core)
	r := newReceiver(set, newTestConfig(url))
	r.drainTimeout = 50 * time.Millisecond
	blocked := make(chan struct{})
	r.nextLogs = &blockingLogs{started: blocked}
	require.NoError(t, r.Start(t.Context(), componenttest.NewNopHost()))

	publish(t, nc, "otel_logs", marshalLogs(t, testdata.GenerateLogs(1)))
	<-blocked

	// t.Context() has no deadline, so Shutdown falls back to drainTimeout.
	require.ErrorIs(t, r.Shutdown(t.Context()), context.DeadlineExceeded)
	assert.True(t, r.conn.IsClosed())
	require.Eventually(t, func() bool {
		return logs.FilterMessage("shutdown aborted message; dropping it").Len() == 1
	}, 5*time.Second, 10*time.Millisecond)
	assert.Zero(t, logs.FilterLevelExact(zapcore.ErrorLevel).Len())
}

// TestReceiver_PermissionDenied verifies that a subscription the server denies
// fails Start: unlike an invalid subject, it leaves the connection open.
func TestReceiver_PermissionDenied(t *testing.T) {
	t.Parallel()

	opts := natstest.DefaultTestOptions
	opts.Port = -1 // random free port
	opts.Users = []*server.User{{
		Username: "otel",
		Password: "s3cret",
		Permissions: &server.Permissions{
			Subscribe: &server.SubjectPermission{Deny: []string{"otel_logs"}},
		},
	}}
	srv := natstest.RunServer(&opts)
	t.Cleanup(srv.Shutdown)

	cfg := newTestConfig(srv.ClientURL())
	cfg.ClientConfig.Auth.User = &natsclient.UserConfig{Username: "otel", Password: "s3cret"}
	r := newTestReceiver(cfg)
	r.nextLogs = consumertest.NewNop()

	err := r.Start(t.Context(), componenttest.NewNopHost())
	require.ErrorIs(t, err, nats.ErrPermissionViolation)
	assert.ErrorContains(t, err, "otel_logs")
	assert.Nil(t, r.conn)
}

// TestReceiver_CountsBeforeHandoff verifies that obsreport counts items before
// the pipeline takes ownership of the data, as the batch processor does by
// moving it out.
func TestReceiver_CountsBeforeHandoff(t *testing.T) {
	t.Parallel()

	url := runServer(t)
	nc := connect(t, url)

	tel := componenttest.NewTelemetry()
	defer func() { require.NoError(t, tel.Shutdown(t.Context())) }()
	set := receivertest.NewNopSettings(metadata.Type)
	set.TelemetrySettings = tel.NewTelemetrySettings()
	r := newReceiver(set, newTestConfig(url))
	sink := new(consumertest.LogsSink)
	r.nextLogs = &movingLogs{next: sink}
	require.NoError(t, r.Start(t.Context(), componenttest.NewNopHost()))
	defer func() { require.NoError(t, r.Shutdown(t.Context())) }()

	publish(t, nc, "otel_logs", marshalLogs(t, testdata.GenerateLogs(3)))

	require.Eventually(t, func() bool { return sink.LogRecordCount() == 3 }, 5*time.Second, 10*time.Millisecond)
	require.Eventually(t, func() bool {
		m, err := tel.GetMetric("otelcol_receiver_accepted_log_records")
		if err != nil {
			return false
		}
		sum, ok := m.Data.(metricdata.Sum[int64])
		return ok && len(sum.DataPoints) == 1 && sum.DataPoints[0].Value == 3
	}, 5*time.Second, 10*time.Millisecond)
}

func TestReceiver_ShutdownWithoutStart(t *testing.T) {
	r := newTestReceiver(createDefaultConfig().(*Config))
	require.NoError(t, r.Shutdown(t.Context()))
}

// extensionHost is a component.Host exposing a fixed set of extensions.
type extensionHost struct {
	exts map[component.ID]component.Component
}

func (h *extensionHost) GetExtensions() map[component.ID]component.Component {
	return h.exts
}

// statusHost is a component.Host that records the last reported status.
type statusHost struct {
	extensionHost
	mu   sync.Mutex
	last componentstatus.Status
}

func (h *statusHost) Report(e *componentstatus.Event) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.last = e.Status()
}

func (h *statusHost) lastStatus() componentstatus.Status {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.last
}

// logsEncoding is a logs-only encoding extension that turns each payload into
// one log record with the payload as its body.
type logsEncoding struct {
	component.StartFunc
	component.ShutdownFunc
}

func (*logsEncoding) UnmarshalLogs(buf []byte) (plog.Logs, error) {
	ld := plog.NewLogs()
	ld.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetStr(string(buf))
	return ld, nil
}

// slowLogs delays each consume to keep messages pending in the client.
type slowLogs struct {
	consumer.Logs
	delay time.Duration
}

func (s *slowLogs) ConsumeLogs(ctx context.Context, ld plog.Logs) error {
	time.Sleep(s.delay)
	return s.Logs.ConsumeLogs(ctx, ld)
}

// movingLogs moves the data out before forwarding it, leaving the caller's
// copy empty as the batch processor does.
type movingLogs struct {
	next consumer.Logs
}

func (*movingLogs) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: true}
}

func (m *movingLogs) ConsumeLogs(ctx context.Context, ld plog.Logs) error {
	moved := plog.NewLogs()
	ld.ResourceLogs().MoveAndAppendTo(moved.ResourceLogs())
	return m.next.ConsumeLogs(ctx, moved)
}

// blockingLogs blocks each consume until its context is canceled.
type blockingLogs struct {
	consumertest.LogsSink
	started chan struct{}
}

func (b *blockingLogs) ConsumeLogs(ctx context.Context, _ plog.Logs) error {
	close(b.started)
	<-ctx.Done()
	return ctx.Err()
}
