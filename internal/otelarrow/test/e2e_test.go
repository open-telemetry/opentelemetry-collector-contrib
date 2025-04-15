// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package test

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/open-telemetry/otel-arrow/pkg/datagen"
	otel_assert "github.com/open-telemetry/otel-arrow/pkg/otel/assert"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pdata/ptrace/ptraceotlp"
	"go.opentelemetry.io/collector/pdata/testdata"
	"go.opentelemetry.io/collector/receiver"
	otelcodes "go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/otelarrowexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/otelarrow/testutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/otelarrowreceiver"
)

type testParams struct {
	threadCount      int
	requestWhileTrue func(*testConsumer) bool

	// missingDeadline is configured so the zero value implies a deadline,
	// which is the default.
	missingDeadline bool

	// skipVerify allows skipping verification for very large tests.  The
	// reason for this is that we have an O(N^2) verification step.
	skipVerify bool
}

type testConsumer struct {
	t *testing.T

	sink      consumertest.TracesSink
	sentSpans atomic.Int64

	recvCfg *otelarrowreceiver.Config
	expCfg  *otelarrowexporter.Config

	recvLogs *observer.ObservedLogs
	expLogs  *observer.ObservedLogs

	recvSpans *tracetest.InMemoryExporter
	expSpans  *tracetest.InMemoryExporter

	expectDeadline bool
}

var _ consumer.Traces = &testConsumer{}

type (
	ExpConfig       = otelarrowexporter.Config
	RecvConfig      = otelarrowreceiver.Config
	CfgFunc         func(*ExpConfig, *RecvConfig)
	GenFunc         func(int) ptrace.Traces
	MkGen           func() GenFunc
	EndFunc         func(t *testing.T, tp testParams, testCon *testConsumer, expect [][]ptrace.Traces) (rops, eops map[string]int)
	ConsumerErrFunc func(t *testing.T, err error)
)

func (*testConsumer) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{}
}

func (tc *testConsumer) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	time.Sleep(time.Duration(float64(time.Millisecond) * (1 + rand.Float64())))

	dead, hasDeadline := ctx.Deadline()
	timeout := time.Until(dead)

	require.Equal(tc.t, tc.expectDeadline, hasDeadline, "deadline set or not set: %v", timeout)

	return tc.sink.ConsumeTraces(ctx, td)
}

func testLoggerSettings(_ *testing.T) (component.TelemetrySettings, *observer.ObservedLogs, *tracetest.InMemoryExporter) {
	tset := componenttest.NewNopTelemetrySettings()

	core, obslogs := observer.New(zapcore.DebugLevel)

	exp := tracetest.NewInMemoryExporter()

	// Note: To debug any of the logs-based assertions in this test, uncomment
	// the following line:
	//
	// tset.Logger = zap.New(zapcore.NewTee(core, zaptest.NewLogger(t).Core()))
	tset.Logger = zap.New(core)
	tset.TracerProvider = trace.NewTracerProvider(trace.WithSyncer(exp))

	return tset, obslogs, exp
}

func basicTestConfig(t *testing.T, tp testParams, cfgF CfgFunc) (*testConsumer, exporter.Traces, receiver.Traces) {
	ctx := context.Background()

	efact := otelarrowexporter.NewFactory()
	rfact := otelarrowreceiver.NewFactory()

	ecfg := efact.CreateDefaultConfig()
	rcfg := rfact.CreateDefaultConfig()

	receiverCfg := rcfg.(*RecvConfig)
	exporterCfg := ecfg.(*ExpConfig)

	addr := testutil.GetAvailableLocalAddress(t)

	receiverCfg.GRPC.NetAddr.Endpoint = addr

	exporterCfg.Endpoint = addr
	exporterCfg.WaitForReady = true
	exporterCfg.TLSSetting.Insecure = true
	exporterCfg.TimeoutSettings.Timeout = time.Minute
	exporterCfg.QueueSettings.Enabled = false
	exporterCfg.RetryConfig.Enabled = true
	exporterCfg.Arrow.NumStreams = 1
	exporterCfg.Arrow.MaxStreamLifetime = 5 * time.Second
	exporterCfg.Arrow.DisableDowngrade = true

	if cfgF != nil {
		cfgF(exporterCfg, receiverCfg)
	}

	expTset, expLogs, expSpans := testLoggerSettings(t)
	recvTset, recvLogs, recvSpans := testLoggerSettings(t)

	testCon := &testConsumer{
		t: t,

		recvCfg: receiverCfg,
		expCfg:  exporterCfg,

		recvLogs: recvLogs,
		expLogs:  expLogs,

		recvSpans: recvSpans,
		expSpans:  expSpans,

		expectDeadline: !tp.missingDeadline,
	}

	receiver, err := rfact.CreateTraces(ctx, receiver.Settings{
		ID:                component.NewID(rfact.Type()),
		TelemetrySettings: recvTset,
	}, receiverCfg, testCon)
	require.NoError(t, err)

	exporter, err := efact.CreateTraces(ctx, exporter.Settings{
		ID:                component.NewID(efact.Type()),
		TelemetrySettings: expTset,
	}, exporterCfg)
	require.NoError(t, err)

	return testCon, exporter, receiver
}

func testIntegrationTraces(ctx context.Context, t *testing.T, tp testParams, cfgf CfgFunc, mkgen MkGen, errf ConsumerErrFunc, endf EndFunc) {
	host := componenttest.NewNopHost()

	testCon, exporter, receiver := basicTestConfig(t, tp, cfgf)

	var startWG sync.WaitGroup
	var exporterShutdownWG sync.WaitGroup
	var startExporterShutdownWG sync.WaitGroup
	var receiverShutdownWG sync.WaitGroup // wait for receiver shutdown

	receiverShutdownWG.Add(1)
	exporterShutdownWG.Add(1)
	startExporterShutdownWG.Add(1)
	startWG.Add(1)

	// Run the receiver, shutdown after exporter does.
	go func() {
		defer receiverShutdownWG.Done()
		assert.NoError(t, receiver.Start(ctx, host))
		exporterShutdownWG.Wait()
		assert.NoError(t, receiver.Shutdown(ctx))
	}()

	// Run the exporter and wait for clients to finish
	go func() {
		defer exporterShutdownWG.Done()
		assert.NoError(t, exporter.Start(ctx, host))
		startWG.Done()
		startExporterShutdownWG.Wait()
		assert.NoError(t, exporter.Shutdown(ctx))
	}()

	// wait for the exporter to start
	startWG.Wait()
	var clientDoneWG sync.WaitGroup // wait for client to finish

	expect := make([][]ptrace.Traces, tp.threadCount)

	for num := 0; num < tp.threadCount; num++ {
		clientDoneWG.Add(1)
		go func(num int) {
			defer clientDoneWG.Done()
			generator := mkgen()
			for i := 0; tp.requestWhileTrue(testCon); i++ {
				td := generator(i)

				errf(t, exporter.ConsumeTraces(ctx, td))
				testCon.sentSpans.Add(int64(td.SpanCount()))
				expect[num] = append(expect[num], td)
			}
		}(num)
	}

	// wait til senders finish
	clientDoneWG.Wait()

	// shut down exporter; it triggers receiver to shut down
	startExporterShutdownWG.Done()

	// wait for receiver to shut down
	receiverShutdownWG.Wait()

	endf(t, tp, testCon, expect)
}

func makeTestTraces(i int) ptrace.Traces {
	td := ptrace.NewTraces()
	td.ResourceSpans().AppendEmpty().Resource().Attributes().PutStr("resource-attr", fmt.Sprint("resource-attr-val-", i))

	ss := td.ResourceSpans().At(0).ScopeSpans().AppendEmpty().Spans()
	span := ss.AppendEmpty()

	span.SetName("operationA")
	span.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	span.SetEndTimestamp(pcommon.NewTimestampFromTime(time.Now()))

	span.SetTraceID(testutil.UInt64ToTraceID(rand.Uint64(), rand.Uint64()))
	span.SetSpanID(testutil.UInt64ToSpanID(rand.Uint64()))
	evs := span.Events()
	ev0 := evs.AppendEmpty()
	ev0.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	ev0.SetName("event-with-attr")
	ev0.Attributes().PutStr("span-event-attr", "span-event-attr-val")
	ev0.SetDroppedAttributesCount(2)
	ev1 := evs.AppendEmpty()
	ev1.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	ev1.SetName("event")
	ev1.SetDroppedAttributesCount(2)
	span.SetDroppedEventsCount(1)
	status := span.Status()
	status.SetCode(ptrace.StatusCodeError)
	status.SetMessage("status-cancelled")

	return td
}

func bulkyGenFunc() MkGen {
	return func() GenFunc {
		entropy := datagen.NewTestEntropy(int64(rand.Uint64()))

		tracesGen := datagen.NewTracesGenerator(
			entropy,
			entropy.NewStandardResourceAttributes(),
			entropy.NewStandardInstrumentationScopes(),
		)
		return func(x int) ptrace.Traces {
			if x == 0 {
				return testdata.GenerateTraces(1)
			}
			return tracesGen.Generate(1000, time.Minute)
		}
	}
}

func standardEnding(t *testing.T, params testParams, testCon *testConsumer, expect [][]ptrace.Traces) (rops, eops map[string]int) {
	// Check for matching request count and data
	require.Equal(t, int(testCon.sentSpans.Load()), testCon.sink.SpanCount())

	if !params.skipVerify {
		var expectJSON []json.Marshaler
		for _, tdn := range expect {
			for _, td := range tdn {
				expectJSON = append(expectJSON, ptraceotlp.NewExportRequestFromTraces(td))
			}
		}
		var receivedJSON []json.Marshaler

		for _, td := range testCon.sink.AllTraces() {
			receivedJSON = append(receivedJSON, ptraceotlp.NewExportRequestFromTraces(td))
		}
		asserter := otel_assert.NewStdUnitTest(t)

		otel_assert.Equiv(asserter, expectJSON, receivedJSON)
	}

	rops = map[string]int{}
	eops = map[string]int{}

	for _, span := range testCon.expSpans.GetSpans() {
		eops[fmt.Sprintf("%v/%v", span.Name, span.Status.Code)]++

		// This span has a recognized span error which we can't easily fix. See
		// https://github.com/open-telemetry/opentelemetry-go-contrib/issues/2644
		if span.Name == "opentelemetry.proto.experimental.arrow.v1.ArrowTracesService/ArrowTraces" {
			continue
		}

		require.NotEqual(t, otelcodes.Error, span.Status.Code,
			"Exporter span has error: %v: %v", span.Name, span.Status.Description)
	}
	for _, span := range testCon.recvSpans.GetSpans() {
		rops[fmt.Sprintf("%v/%v", span.Name, span.Status.Code)]++
		// This span occasionally has a "transport is closing error"
		if span.Name == "opentelemetry.proto.experimental.arrow.v1.ArrowTracesService/ArrowTraces" {
			continue
		}

		require.NotEqual(t, otelcodes.Error, span.Status.Code,
			"Receiver span has error: %v: %v", span.Name, span.Status.Description)
	}
	return rops, eops
}

// logSigs computes a signature of a structured log message emitted by
// the component via the Zap observer.  The encoding is the message,
// "|||", followed by the field names in order, separated by "///".
//
// Specifically, we expect "arrow stream error" messages.  When this is
// the case, the corresponding message is returned as a slice.
func logSigs(obs *observer.ObservedLogs) (map[string]int, []string) {
	counts := map[string]int{}
	var msgs []string
	for _, rl := range obs.All() {
		var attrs []string
		for _, f := range rl.Context {
			attrs = append(attrs, f.Key)

			// One way we can see memory limit errors is through the
			// OTel-Arrow common "arrow stream error" message, which both
			// sides will log.
			if rl.Message == "arrow stream error" && f.Key == "message" {
				msgs = append(msgs, f.String)
			}
		}
		var sig strings.Builder
		sig.WriteString(rl.Message)
		sig.WriteString("|||")
		sig.WriteString(strings.Join(attrs, "///"))
		counts[sig.String()]++
	}
	return counts, msgs
}

var limitRegexp = regexp.MustCompile(`memory limit exceeded`)

func countMemoryLimitErrors(msgs []string) (cnt int) {
	for _, msg := range msgs {
		// The memory errors are expected from the receiver,
		// so whether these print on the exporter or receiver,
		// the message will contain "otel-arrow decode" from
		// the receiver.
		if limitRegexp.MatchString(msg) && strings.Contains(msg, "otel-arrow decode") {
			cnt++
		}
	}
	return
}

func failureMemoryLimitEnding(t *testing.T, _ testParams, testCon *testConsumer, _ [][]ptrace.Traces) (rops, eops map[string]int) {
	eSigs, eMsgs := logSigs(testCon.expLogs)
	rSigs, rMsgs := logSigs(testCon.recvLogs)

	// Test for arrow stream errors on both sides.
	require.Positive(t, eSigs["arrow stream error|||code///message///where"], "should have exporter arrow stream errors: %v", eMsgs)
	require.Positive(t, rSigs["arrow stream error|||code///message///where"], "should have receiver arrow stream errors: %v", rSigs)

	// Ensure both side's error logs include admission control errors.
	require.Positive(t, countMemoryLimitErrors(rMsgs), "should have memory limit errors: %v", rMsgs)
	require.Positive(t, countMemoryLimitErrors(eMsgs), "should have memory limit errors: %v", eMsgs)

	return nil, nil
}

var admissionRegexp = regexp.MustCompile(`too much pending data`)

func countAdmissionLimitErrors(msgs []string) (cnt int) {
	for _, msg := range msgs {
		if admissionRegexp.MatchString(msg) {
			cnt++
		}
	}
	return
}

func failureAdmissionLimitEnding(t *testing.T, _ testParams, testCon *testConsumer, _ [][]ptrace.Traces) (rops, eops map[string]int) {
	eSigs, eMsgs := logSigs(testCon.expLogs)
	rSigs, rMsgs := logSigs(testCon.recvLogs)

	// Test for arrow stream errors on both sides.
	require.Positive(t, eSigs["arrow stream error|||code///message///where"], "should have exporter arrow stream errors: %v", eMsgs)
	require.Positive(t, rSigs["arrow stream error|||code///message///where"], "should have receiver arrow stream errors: %v", rSigs)

	// Ensure both side's error logs include admission limit errors.
	require.Positive(t, countAdmissionLimitErrors(rMsgs), "should have admission limit errors: %v", rMsgs)
	require.Positive(t, countAdmissionLimitErrors(eMsgs), "should have admission limit errors: %v", eMsgs)

	return nil, nil
}

func consumerSuccess(t *testing.T, err error) {
	require.NoError(t, err)
}

func consumerFailure(t *testing.T, err error) {
	if err == nil {
		return
	}

	// there should be no permanent errors anywhere in this test.
	require.False(t, consumererror.IsPermanent(err),
		"should not be permanent: %v", err)

	stat, ok := status.FromError(err)
	require.True(t, ok, "should be a status error: %v", err)

	switch stat.Code() {
	case codes.ResourceExhausted, codes.Canceled:
		// Cool
	default:
		// Not cool
		t.Fatalf("unexpected status code %v", stat)
	}
}

func TestIntegrationTracesSimple(t *testing.T) {
	for _, n := range []int{1, 2, 4, 8} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			// until 10 threads can write 1000 spans
			params := testParams{
				threadCount: 10,
				requestWhileTrue: func(test *testConsumer) bool {
					return test.sink.SpanCount() < 1000
				},
			}

			testIntegrationTraces(ctx, t, params, func(ecfg *ExpConfig, _ *RecvConfig) {
				ecfg.Arrow.NumStreams = n
			}, func() GenFunc { return makeTestTraces }, consumerSuccess, standardEnding)
		})
	}
}

func TestIntegrationDeadlinePropagation(t *testing.T) {
	for _, hasDeadline := range []bool{false, true} {
		t.Run(fmt.Sprint("deadline=", hasDeadline), func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			// Until at least one span is written.
			params := testParams{
				threadCount: 1,
				requestWhileTrue: func(test *testConsumer) bool {
					return test.sink.SpanCount() < 1
				},
				missingDeadline: !hasDeadline,
			}

			testIntegrationTraces(ctx, t, params, func(ecfg *ExpConfig, _ *RecvConfig) {
				if !hasDeadline {
					// 0 disables the exporthelper-set timeout.
					ecfg.TimeoutSettings.Timeout = 0
				} else {
					ecfg.TimeoutSettings.Timeout = 37 * time.Minute
				}
			}, func() GenFunc { return makeTestTraces }, consumerSuccess, standardEnding)
		})
	}
}

func TestIntegrationMemoryLimited(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// until exporter and receiver finish at least one ArrowTraces span.
	params := testParams{
		threadCount: 10,
		requestWhileTrue: func(test *testConsumer) bool {
			cf := func(spans tracetest.SpanStubs) (cnt int) {
				for _, span := range spans {
					if span.Name == "opentelemetry.proto.experimental.arrow.v1.ArrowTracesService/ArrowTraces" {
						cnt++
					}
				}
				return
			}
			rcnt := cf(test.recvSpans.GetSpans())
			ecnt := cf(test.expSpans.GetSpans())
			return ecnt == 0 || rcnt == 0
		},
	}

	testIntegrationTraces(ctx, t, params, func(ecfg *ExpConfig, rcfg *RecvConfig) {
		rcfg.Arrow.MemoryLimitMiB = 1
		ecfg.Arrow.NumStreams = 10
		// Shorten timeouts for this test, because we intend
		// for it to fail and don't want to wait for retries.
		ecfg.TimeoutSettings.Timeout = 5 * time.Second
		ecfg.RetryConfig.InitialInterval = 1 * time.Second
		ecfg.RetryConfig.MaxInterval = 2 * time.Second
		ecfg.RetryConfig.MaxElapsedTime = 30 * time.Second
		ecfg.Arrow.MaxStreamLifetime = 5 * time.Second
	}, bulkyGenFunc(), consumerFailure, failureMemoryLimitEnding)
}

func multiStreamEnding(t *testing.T, p testParams, testCon *testConsumer, td [][]ptrace.Traces) (_, _ map[string]int) {
	recvOps, expOps := standardEnding(t, p, testCon, td)

	const streamName = "opentelemetry.proto.experimental.arrow.v1.ArrowTracesService/ArrowTraces"

	total := int(testCon.sentSpans.Load())

	// Exporter spans:
	//
	// This span is the Arrow gRPC client stream.  Should have no
	// stream errors, > 1 streams.
	expStreamsUnset := expOps[streamName+"/Unset"]
	expStreamsError := expOps[streamName+"/Error"]
	require.Less(t, 1, expStreamsUnset+expStreamsError)
	require.Equal(t, 1, expStreamsError)

	// Number of export requests: exact match.  This is the
	// exporterhelper's base span.
	require.Equal(t, total, expOps["exporter/otelarrow/traces/Unset"])

	// Number of export requests: exact match.  This span covers
	// handling one request in the Arrow exporter.
	require.Equal(t, total, expOps["otel_arrow_stream_send/Unset"])

	// Receiver spans
	//
	// This span is the Arrow gRPC server stream, instrumented by
	// otelgrpc.  Because of
	// https://github.com/open-telemetry/opentelemetry-go-contrib/issues/2644
	// we expect either an error or unset.  There should be > 1
	// streams.
	recvStreamsUnset := recvOps[streamName+"/Unset"]
	recvStreamsError := recvOps[streamName+"/Error"]
	require.Equal(t, 0, recvStreamsError)
	require.Less(t, 1, recvStreamsUnset+recvStreamsError)

	// For each stream, there is one Recv() span at the end that ends
	// in cancelation (or EOF).  So we expect total to be less than
	// this span count.
	require.Equal(t, total+recvStreamsUnset+recvStreamsError, recvOps["otel_arrow_stream_inflight/Unset"])

	// This is in request context, the Arrow stream handling one request.
	require.Equal(t, total, recvOps["otel_arrow_stream_recv/Unset"])

	// This is in request context, the receiverhelper's per-request span.
	require.Equal(t, total, recvOps["receiver/otelarrow/TraceDataReceived/Unset"])

	// Exporter and Receiver stream span counts match:
	require.Equal(t, expStreamsUnset+expStreamsError, recvStreamsUnset+recvStreamsError)

	return recvOps, expOps
}

func TestIntegrationSelfTracing(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// until 2 Arrow stream spans are received from self instrumentation
	params := testParams{
		threadCount: 10,
		requestWhileTrue: func(test *testConsumer) bool {
			cnt := 0
			for _, span := range test.expSpans.GetSpans() {
				if span.Name == "opentelemetry.proto.experimental.arrow.v1.ArrowTracesService/ArrowTraces" {
					cnt++
				}
			}
			return cnt < 2
		},
	}

	testIntegrationTraces(ctx, t, params, func(_ *ExpConfig, rcfg *RecvConfig) {
		rcfg.GRPC.Keepalive = &configgrpc.KeepaliveServerConfig{
			ServerParameters: &configgrpc.KeepaliveServerParameters{
				MaxConnectionAge:      time.Second,
				MaxConnectionAgeGrace: 5 * time.Second,
			},
		}
	}, func() GenFunc { return makeTestTraces }, consumerSuccess, multiStreamEnding)
}

func nearLimitGenFunc() MkGen {
	var sizer ptrace.ProtoMarshaler
	const nearLimit = 900 << 10 // close to 1 MiB
	const hardLimit = 1 << 20   // 1 MiB

	return func() GenFunc {
		entropy := datagen.NewTestEntropy(int64(rand.Uint64()))

		tracesGen := datagen.NewTracesGenerator(
			entropy,
			entropy.NewStandardResourceAttributes(),
			entropy.NewStandardInstrumentationScopes(),
		)

		return func(int) ptrace.Traces {
			size := 100
			for {
				td := tracesGen.Generate(size, time.Minute)
				uncomp := sizer.TracesSize(td)
				if uncomp > nearLimit && uncomp < hardLimit {
					return td
				}
				switch {
				case uncomp > hardLimit:
					size -= 10
				case uncomp < nearLimit/2:
					size *= 2
				default:
					size += 10
				}
			}
		}
	}
}

func TestIntegrationAdmissionLimited(t *testing.T) {
	for _, test := range []struct {
		bounded   bool
		allowWait bool
	}{
		{
			bounded:   true,  // bounded queue
			allowWait: false, // no waiters allowed
		},
		{
			bounded:   true, // bounded queue
			allowWait: true, // with waiters allowed
		},
		{
			bounded:   false, // bounded queue
			allowWait: true,  // config not used
		},
	} {
		t.Run(fmt.Sprint("bounded=", test.bounded, ",allow_wait=", test.allowWait), func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			// until exporter and receiver finish at least one ArrowTraces span.
			params := testParams{
				threadCount: 10,
				requestWhileTrue: func(test *testConsumer) bool {
					return test.sink.SpanCount() < 10000
				},
				missingDeadline: true,

				// this test produces a lot of data,
				// which is too expensive to verify
				// with an O(N^2) algorithm.
				skipVerify: true,
			}

			var ending func(*testing.T, testParams, *testConsumer, [][]ptrace.Traces) (_, _ map[string]int)
			var waitingLimit uint64
			var admitLimit uint64
			if test.bounded {
				if test.allowWait {
					admitLimit = 1
					waitingLimit = uint64(params.threadCount)
					ending = standardEnding
				} else {
					admitLimit = 1
					waitingLimit = 0
					ending = failureAdmissionLimitEnding
				}
			} else {
				admitLimit = 0
				waitingLimit = 0
				ending = standardEnding
			}

			testIntegrationTraces(ctx, t, params, func(ecfg *ExpConfig, rcfg *RecvConfig) {
				rcfg.Admission.RequestLimitMiB = admitLimit
				rcfg.Admission.WaitingLimitMiB = waitingLimit

				ecfg.Arrow.NumStreams = 10

				// Disable export timeout and retry,
				// because we expect progress to be
				// slow for this test.
				ecfg.TimeoutSettings.Timeout = 0
				ecfg.RetryConfig.Enabled = false
				ecfg.Arrow.MaxStreamLifetime = 5 * time.Second
			}, nearLimitGenFunc(), consumerFailure, ending)
		})
	}
}
