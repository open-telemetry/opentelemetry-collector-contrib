// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otelarrowreceiver

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"sync"
	"testing"
	"time"

	arrowpb "github.com/open-telemetry/otel-arrow/api/experimental/arrow/v1"
	arrowRecord "github.com/open-telemetry/otel-arrow/pkg/otel/arrow_record"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/client"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configauth"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pdata/ptrace/ptraceotlp"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/metric/metricdata/metricdatatest"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
	"golang.org/x/net/http2/hpack"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/otelarrow/testdata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/otelarrow/testutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/otelarrowreceiver/internal/arrow/mock"
	componentmetadata "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/otelarrowreceiver/internal/metadata"
)

const otlpReceiverName = "receiver_test"

var testReceiverID = component.NewIDWithName(componentmetadata.Type, otlpReceiverName)

func TestGRPCNewPortAlreadyUsed(t *testing.T) {
	addr := testutil.GetAvailableLocalAddress(t)
	ln, err := net.Listen("tcp", addr)
	require.NoError(t, err, "failed to listen on %q: %v", addr, err)
	t.Cleanup(func() {
		assert.NoError(t, ln.Close())
	})
	tt := componenttest.NewNopTelemetrySettings()
	r := newGRPCReceiver(t, addr, tt, consumertest.NewNop(), consumertest.NewNop())
	require.NotNil(t, r)

	require.Error(t, r.Start(context.Background(), componenttest.NewNopHost()))
}

// TestOTelArrowReceiverGRPCTracesIngestTest checks that the gRPC trace receiver
// is returning the proper response (return and metrics) when the next consumer
// in the pipeline reports error. The test changes the responses returned by the
// next trace consumer, checks if data was passed down the pipeline and if
// proper metrics were recorded. It also uses all endpoints supported by the
// trace receiver.
func TestOTelArrowReceiverGRPCTracesIngestTest(t *testing.T) {
	type ingestionStateTest struct {
		okToIngest   bool
		expectedCode codes.Code
	}

	expectedReceivedBatches := 2
	ingestionStates := []ingestionStateTest{
		{
			okToIngest:   true,
			expectedCode: codes.OK,
		},
		{
			okToIngest:   false,
			expectedCode: codes.Unknown,
		},
		{
			okToIngest:   true,
			expectedCode: codes.OK,
		},
	}

	addr := testutil.GetAvailableLocalAddress(t)
	td := testdata.GenerateTraces(1)

	tt := componenttest.NewTelemetry()
	t.Cleanup(func() { require.NoError(t, tt.Shutdown(context.Background())) })

	sink := &errOrSinkConsumer{TracesSink: new(consumertest.TracesSink)}

	ocr := newGRPCReceiver(t, addr, tt.NewTelemetrySettings(), sink, nil)
	require.NotNil(t, ocr)
	require.NoError(t, ocr.Start(context.Background(), componenttest.NewNopHost()))
	t.Cleanup(func() { require.NoError(t, ocr.Shutdown(context.Background())) })

	cc, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	defer func() {
		assert.NoError(t, cc.Close())
	}()

	for _, ingestionState := range ingestionStates {
		if ingestionState.okToIngest {
			sink.SetConsumeError(nil)
		} else {
			sink.SetConsumeError(errors.New("consumer error"))
		}

		_, err = ptraceotlp.NewGRPCClient(cc).Export(context.Background(), ptraceotlp.NewExportRequestFromTraces(td))
		errStatus, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, ingestionState.expectedCode, errStatus.Code())
	}

	require.Len(t, sink.AllTraces(), expectedReceivedBatches)

	expectedIngestionBlockedRPCs := 1

	got, err := tt.GetMetric("otelcol_receiver_accepted_spans")
	assert.NoError(t, err)
	metricdatatest.AssertEqual(t,
		metricdata.Metrics{
			Name:        "otelcol_receiver_accepted_spans",
			Description: "Number of spans successfully pushed into the pipeline. [alpha]",
			Unit:        "{spans}",
			Data: metricdata.Sum[int64]{
				Temporality: metricdata.CumulativeTemporality,
				IsMonotonic: true,
				DataPoints: []metricdata.DataPoint[int64]{
					{
						Attributes: attribute.NewSet(
							attribute.String("receiver", testReceiverID.String()),
							attribute.String("transport", "grpc")),
						Value: int64(expectedReceivedBatches),
					},
				},
			},
		}, got, metricdatatest.IgnoreTimestamp(), metricdatatest.IgnoreExemplars())

	got, err = tt.GetMetric("otelcol_receiver_refused_spans")
	assert.NoError(t, err)
	metricdatatest.AssertEqual(t,
		metricdata.Metrics{
			Name:        "otelcol_receiver_refused_spans",
			Description: "Number of spans that could not be pushed into the pipeline. [alpha]",
			Unit:        "{spans}",
			Data: metricdata.Sum[int64]{
				Temporality: metricdata.CumulativeTemporality,
				IsMonotonic: true,
				DataPoints: []metricdata.DataPoint[int64]{
					{
						Attributes: attribute.NewSet(
							attribute.String("receiver", testReceiverID.String()),
							attribute.String("transport", "grpc")),
						Value: int64(expectedIngestionBlockedRPCs),
					},
				},
			},
		}, got, metricdatatest.IgnoreTimestamp(), metricdatatest.IgnoreExemplars())
}

func TestGRPCInvalidTLSCredentials(t *testing.T) {
	cfg := &Config{
		Protocols: Protocols{
			GRPC: configgrpc.ServerConfig{
				NetAddr: confignet.AddrConfig{
					Endpoint:  testutil.GetAvailableLocalAddress(t),
					Transport: confignet.TransportTypeTCP,
				},
				TLSSetting: &configtls.ServerConfig{
					Config: configtls.Config{
						CertFile: "willfail",
					},
				},
			},
		},
	}

	r, err := NewFactory().CreateTraces(
		context.Background(),
		receivertest.NewNopSettings(componentmetadata.Type),
		cfg,
		consumertest.NewNop())

	require.NoError(t, err)
	assert.NotNil(t, r)

	assert.EqualError(t,
		r.Start(context.Background(), componenttest.NewNopHost()),
		`failed to load TLS config: failed to load TLS cert and key: for auth via TLS, provide both certificate and key, or neither`)
}

func TestGRPCMaxRecvSize(t *testing.T) {
	addr := testutil.GetAvailableLocalAddress(t)
	sink := new(consumertest.TracesSink)

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.GRPC.NetAddr.Endpoint = addr
	tt := componenttest.NewNopTelemetrySettings()
	ocr := newReceiver(t, factory, tt, cfg, testReceiverID, sink, nil)

	require.NotNil(t, ocr)
	require.NoError(t, ocr.Start(context.Background(), componenttest.NewNopHost()))

	cc, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)

	td := testdata.GenerateTraces(50000)
	require.Error(t, exportTraces(cc, td))
	assert.NoError(t, cc.Close())
	require.NoError(t, ocr.Shutdown(context.Background()))

	cfg.GRPC.MaxRecvMsgSizeMiB = 100

	ocr = newReceiver(t, factory, tt, cfg, testReceiverID, sink, nil)

	require.NotNil(t, ocr)
	require.NoError(t, ocr.Start(context.Background(), componenttest.NewNopHost()))
	t.Cleanup(func() { require.NoError(t, ocr.Shutdown(context.Background())) })

	cc, err = grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	defer func() {
		assert.NoError(t, cc.Close())
	}()

	td = testdata.GenerateTraces(50000)
	require.NoError(t, exportTraces(cc, td))
	require.Len(t, sink.AllTraces(), 1)
	assert.Equal(t, td, sink.AllTraces()[0])
}

func newGRPCReceiver(t *testing.T, endpoint string, settings component.TelemetrySettings, tc consumer.Traces, mc consumer.Metrics) component.Component {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.GRPC.NetAddr.Endpoint = endpoint
	return newReceiver(t, factory, settings, cfg, testReceiverID, tc, mc)
}

func newReceiver(t *testing.T, factory receiver.Factory, settings component.TelemetrySettings, cfg *Config, id component.ID, tc consumer.Traces, mc consumer.Metrics) component.Component {
	set := receivertest.NewNopSettings(componentmetadata.Type)
	set.TelemetrySettings = settings
	set.ID = id
	var r component.Component
	var err error
	if tc != nil {
		r, err = factory.CreateTraces(context.Background(), set, cfg, tc)
		require.NoError(t, err)
	}
	if mc != nil {
		r, err = factory.CreateMetrics(context.Background(), set, cfg, mc)
		require.NoError(t, err)
	}
	return r
}

type senderFunc func(td ptrace.Traces)

func TestStandardShutdown(t *testing.T) {
	endpointGrpc := testutil.GetAvailableLocalAddress(t)

	nextSink := new(consumertest.TracesSink)

	// Create OTelArrow receiver
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.GRPC.NetAddr.Endpoint = endpointGrpc
	set := receivertest.NewNopSettings(componentmetadata.Type)
	set.ID = testReceiverID
	r, err := NewFactory().CreateTraces(
		context.Background(),
		set,
		cfg,
		nextSink)
	require.NoError(t, err)
	require.NotNil(t, r)
	require.NoError(t, r.Start(context.Background(), componenttest.NewNopHost()))

	conn, err := grpc.NewClient(endpointGrpc, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	defer conn.Close()

	doneSignalGrpc := make(chan bool)

	senderGrpc := func(td ptrace.Traces) {
		// Ignore error, may be executed after the receiver shutdown.
		_ = exportTraces(conn, td)
	}

	// Send traces to the receiver until we signal via done channel, and then
	// send one more trace after that.
	go generateTraces(senderGrpc, doneSignalGrpc)

	// Wait until the receiver outputs anything to the sink.
	assert.Eventually(t, func() bool {
		return nextSink.SpanCount() > 0
	}, time.Second, 10*time.Millisecond)

	// Now shutdown the receiver, while continuing sending traces to it.
	ctx, cancelFn := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelFn()
	err = r.Shutdown(ctx)
	assert.NoError(t, err)

	// Remember how many spans the sink received. This number should not change after this
	// point because after Shutdown() returns the component is not allowed to produce
	// any more data.
	sinkSpanCountAfterShutdown := nextSink.SpanCount()

	// Now signal to generateTraces to exit the main generation loop, then send
	// one more trace and stop.
	doneSignalGrpc <- true

	// Wait until all follow up traces are sent.
	<-doneSignalGrpc

	// The last, additional trace should not be received by sink, so the number of spans in
	// the sink should not change.
	assert.Equal(t, sinkSpanCountAfterShutdown, nextSink.SpanCount())
}

func TestOTelArrowShutdown(t *testing.T) {
	// In the cooperative test, the client calls CloseSend() with no
	// keepalive set.  In the non-cooperative case, the keepalive ends
	// the stream.
	for _, cooperative := range []bool{true, false} {
		t.Run(fmt.Sprint("cooperative=", cooperative), func(t *testing.T) {
			ctx := context.Background()

			endpointGrpc := testutil.GetAvailableLocalAddress(t)

			nextSink := new(consumertest.TracesSink)

			// Create OTelArrow receiver
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig().(*Config)
			cfg.GRPC.Keepalive = &configgrpc.KeepaliveServerConfig{
				ServerParameters: &configgrpc.KeepaliveServerParameters{},
			}
			if !cooperative {
				cfg.GRPC.Keepalive.ServerParameters.MaxConnectionAge = time.Second
				cfg.GRPC.Keepalive.ServerParameters.MaxConnectionAgeGrace = 5 * time.Second
			}
			cfg.GRPC.NetAddr.Endpoint = endpointGrpc
			set := receivertest.NewNopSettings(componentmetadata.Type)
			core, obslogs := observer.New(zapcore.DebugLevel)
			set.Logger = zap.New(core)

			set.ID = testReceiverID
			r, err := NewFactory().CreateTraces(
				ctx,
				set,
				cfg,
				nextSink)
			require.NoError(t, err)
			require.NotNil(t, r)
			require.NoError(t, r.Start(context.Background(), componenttest.NewNopHost()))

			conn, err := grpc.NewClient(endpointGrpc, grpc.WithTransportCredentials(insecure.NewCredentials()))
			require.NoError(t, err)
			defer func() {
				require.NoError(t, conn.Close())
			}()

			client := arrowpb.NewArrowTracesServiceClient(conn)
			stream, err := client.ArrowTraces(ctx, grpc.WaitForReady(true))
			require.NoError(t, err)
			producer := arrowRecord.NewProducer()

			start := time.Now()

			// Send traces to the receiver until we signal.
			go func() {
				for time.Since(start) < 5*time.Second {
					td := testdata.GenerateTraces(1)
					batch, batchErr := producer.BatchArrowRecordsFromTraces(td)
					assert.NoError(t, batchErr)
					assert.NoError(t, stream.Send(batch))
				}

				if cooperative {
					assert.NoError(t, stream.CloseSend())
				}
			}()

			var recvWG sync.WaitGroup
			recvWG.Add(1)

			// Receive batch responses. See the comment on
			// https://pkg.go.dev/google.golang.org/grpc#ClientConn.NewStream
			// to explain why this must be done.  We do not use the
			// return value, this just avoids leaking the stream context,
			// which can otherwise hang this test.
			go func() {
				defer recvWG.Done()
				for {
					if _, recvErr := stream.Recv(); recvErr == nil {
						continue
					}
					break
				}
			}()

			// Wait until the receiver outputs anything to the sink.
			assert.Eventually(t, func() bool {
				return nextSink.SpanCount() > 0
			}, time.Second, 10*time.Millisecond)

			// Now shutdown the receiver, while continuing sending traces to it.
			// Note that gRPC GracefulShutdown() does not actually use the context
			// for cancelation.
			err = r.Shutdown(context.Background())
			assert.NoError(t, err)

			// recvWG ensures the stream has been read before the test exits.
			recvWG.Wait()

			// Remember how many spans the sink received. This number should not change after this
			// point because after Shutdown() returns the component is not allowed to produce
			// any more data.
			sinkSpanCountAfterShutdown := nextSink.SpanCount()

			// The last, additional trace should not be received by sink, so the number of spans in
			// the sink should not change.
			assert.Equal(t, sinkSpanCountAfterShutdown, nextSink.SpanCount())

			shutdownCause := ""
		scanLogs:
			for _, log := range obslogs.All() {
				if log.Message == "arrow stream shutdown" {
					for _, f := range log.Context {
						if f.Key == "message" {
							shutdownCause = f.String
							break scanLogs
						}
					}
				}
			}
			assert.Equal(t, "EOF", shutdownCause)
		})
	}
}

// generateTraces originates from the OTLP receiver "standard" shutdown test.
func generateTraces(senderFn senderFunc, doneSignal chan bool) {
	// Continuously generate spans until signaled to stop.
loop:
	for {
		select {
		case <-doneSignal:
			break loop
		default:
		}
		senderFn(testdata.GenerateTraces(1))
	}

	// After getting the signal to stop, send one more span and then
	// finally stop. We should never receive this last span.
	senderFn(testdata.GenerateTraces(1))

	// Indicate that we are done.
	close(doneSignal)
}

func exportTraces(cc *grpc.ClientConn, td ptrace.Traces) error {
	acc := ptraceotlp.NewGRPCClient(cc)
	req := ptraceotlp.NewExportRequestFromTraces(td)
	_, err := acc.Export(context.Background(), req)

	return err
}

type errOrSinkConsumer struct {
	*consumertest.TracesSink
	*consumertest.MetricsSink
	mu           sync.Mutex
	consumeError error // to be returned by ConsumeTraces, if set
}

// SetConsumeError sets an error that will be returned by the Consume function.
func (esc *errOrSinkConsumer) SetConsumeError(err error) {
	esc.mu.Lock()
	defer esc.mu.Unlock()
	esc.consumeError = err
}

func (esc *errOrSinkConsumer) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

// ConsumeTraces stores traces to this sink.
func (esc *errOrSinkConsumer) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	esc.mu.Lock()
	defer esc.mu.Unlock()

	if esc.consumeError != nil {
		return esc.consumeError
	}

	return esc.TracesSink.ConsumeTraces(ctx, td)
}

// ConsumeMetrics stores metrics to this sink.
func (esc *errOrSinkConsumer) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	esc.mu.Lock()
	defer esc.mu.Unlock()

	if esc.consumeError != nil {
		return esc.consumeError
	}

	return esc.MetricsSink.ConsumeMetrics(ctx, md)
}

// Reset deletes any stored in the sinks, resets error to nil.
func (esc *errOrSinkConsumer) Reset() {
	esc.mu.Lock()
	defer esc.mu.Unlock()

	esc.consumeError = nil
	if esc.TracesSink != nil {
		esc.TracesSink.Reset()
	}
	if esc.MetricsSink != nil {
		esc.MetricsSink.Reset()
	}
}

type tracesSinkWithMetadata struct {
	consumertest.TracesSink

	lock sync.Mutex
	mds  []client.Metadata
}

func (ts *tracesSinkWithMetadata) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	info := client.FromContext(ctx)
	ts.lock.Lock()
	defer ts.lock.Unlock()
	ts.mds = append(ts.mds, info.Metadata)
	return ts.TracesSink.ConsumeTraces(ctx, td)
}

func (ts *tracesSinkWithMetadata) Metadatas() []client.Metadata {
	ts.lock.Lock()
	defer ts.lock.Unlock()
	return ts.mds
}

type anyStreamClient interface {
	Send(*arrowpb.BatchArrowRecords) error
	Recv() (*arrowpb.BatchStatus, error)
	grpc.ClientStream
}

func TestGRPCArrowReceiver(t *testing.T) {
	addr := testutil.GetAvailableLocalAddress(t)
	sink := new(tracesSinkWithMetadata)

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.GRPC.NetAddr.Endpoint = addr
	cfg.GRPC.IncludeMetadata = true
	id := component.NewID(componentmetadata.Type)
	tt := componenttest.NewNopTelemetrySettings()
	ocr := newReceiver(t, factory, tt, cfg, id, sink, nil)

	require.NotNil(t, ocr)
	require.NoError(t, ocr.Start(context.Background(), componenttest.NewNopHost()))

	cc, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var stream anyStreamClient
	client := arrowpb.NewArrowTracesServiceClient(cc)
	stream, err = client.ArrowTraces(ctx, grpc.WaitForReady(true))
	require.NoError(t, err)
	producer := arrowRecord.NewProducer()

	var headerBuf bytes.Buffer
	hpd := hpack.NewEncoder(&headerBuf)

	var expectTraces []ptrace.Traces
	var expectMDs []metadata.MD

	// Repeatedly send traces via arrow. Set the expected traces
	// metadata to receive.
	for i := 0; i < 10; i++ {
		td := testdata.GenerateTraces(2)
		expectTraces = append(expectTraces, td)

		headerBuf.Reset()
		err := hpd.WriteField(hpack.HeaderField{
			Name:  "seq",
			Value: strconv.Itoa(i),
		})
		require.NoError(t, err)
		err = hpd.WriteField(hpack.HeaderField{
			Name:  "test",
			Value: "value",
		})
		require.NoError(t, err)
		expectMDs = append(expectMDs, metadata.MD{
			"seq":  []string{strconv.Itoa(i)},
			"test": []string{"value"},
		})

		batch, err := producer.BatchArrowRecordsFromTraces(td)
		require.NoError(t, err)

		batch.Headers = headerBuf.Bytes()

		err = stream.Send(batch)

		require.NoError(t, err)

		resp, err := stream.Recv()
		require.NoError(t, err)
		require.Equal(t, batch.BatchId, resp.BatchId)
		require.Equal(t, arrowpb.StatusCode_OK, resp.StatusCode)
	}

	assert.NoError(t, cc.Close())
	require.NoError(t, ocr.Shutdown(context.Background()))

	assert.Equal(t, expectTraces, sink.AllTraces())

	assert.Len(t, sink.Metadatas(), len(expectMDs))
	// gRPC adds its own metadata keys, so we check for only the
	// expected ones below:
	for idx := range expectMDs {
		for key, vals := range expectMDs[idx] {
			require.Equal(t, vals, sink.Metadatas()[idx].Get(key), "for key %s", key)
		}
	}
}

type hostWithExtensions struct {
	component.Host
	exts map[component.ID]component.Component
}

func newHostWithExtensions(exts map[component.ID]component.Component) component.Host {
	return &hostWithExtensions{
		Host: componenttest.NewNopHost(),
		exts: exts,
	}
}

func (h *hostWithExtensions) GetExtensions() map[component.ID]component.Component {
	return h.exts
}

func newTestAuthExtension(t *testing.T, authFunc func(ctx context.Context, hdrs map[string][]string) (context.Context, error)) extension.Extension {
	ctrl := gomock.NewController(t)
	as := mock.NewMockServer(ctrl)
	as.EXPECT().Authenticate(gomock.Any(), gomock.Any()).AnyTimes().DoAndReturn(authFunc)
	return as
}

func TestGRPCArrowReceiverAuth(t *testing.T) {
	addr := testutil.GetAvailableLocalAddress(t)
	sink := new(tracesSinkWithMetadata)

	authID := component.NewID(component.MustNewType("testauth"))

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.GRPC.NetAddr.Endpoint = addr
	cfg.GRPC.IncludeMetadata = true
	cfg.GRPC.Auth = &configauth.Config{
		AuthenticatorID: authID,
	}
	id := component.NewID(componentmetadata.Type)
	tt := componenttest.NewNopTelemetrySettings()
	ocr := newReceiver(t, factory, tt, cfg, id, sink, nil)

	require.NotNil(t, ocr)

	const errorString = "very much not authorized"

	type inStreamCtx struct{}

	host := newHostWithExtensions(
		map[component.ID]component.Component{
			authID: newTestAuthExtension(t, func(ctx context.Context, _ map[string][]string) (context.Context, error) {
				if ctx.Value(inStreamCtx{}) != nil {
					return ctx, errors.New(errorString)
				}
				return context.WithValue(ctx, inStreamCtx{}, t), nil
			}),
		},
	)

	require.NoError(t, ocr.Start(context.Background(), host))

	cc, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client := arrowpb.NewArrowTracesServiceClient(cc)
	stream, err := client.ArrowTraces(ctx, grpc.WaitForReady(true))
	require.NoError(t, err)
	producer := arrowRecord.NewProducer()

	// Repeatedly send traces via arrow. Expect an auth error.
	for i := 0; i < 10; i++ {
		td := testdata.GenerateTraces(2)

		batch, err := producer.BatchArrowRecordsFromTraces(td)
		require.NoError(t, err)

		err = stream.Send(batch)
		require.NoError(t, err)

		resp, err := stream.Recv()
		require.NoError(t, err)
		// The stream has to be successful to get this far.  The
		// authenticator fails every data item:
		require.Equal(t, batch.BatchId, resp.BatchId)
		require.Equal(t, arrowpb.StatusCode_UNAUTHENTICATED, resp.StatusCode)
		require.Contains(t, resp.StatusMessage, errorString)
	}

	assert.NoError(t, cc.Close())
	require.NoError(t, ocr.Shutdown(context.Background()))

	assert.Empty(t, sink.AllTraces())
}

func TestConcurrentArrowReceiver(t *testing.T) {
	addr := testutil.GetAvailableLocalAddress(t)
	sink := new(tracesSinkWithMetadata)

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.GRPC.NetAddr.Endpoint = addr
	cfg.GRPC.IncludeMetadata = true
	id := component.NewID(componentmetadata.Type)
	tt := componenttest.NewNopTelemetrySettings()
	ocr := newReceiver(t, factory, tt, cfg, id, sink, nil)

	require.NotNil(t, ocr)
	require.NoError(t, ocr.Start(context.Background(), componenttest.NewNopHost()))

	cc, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	const itemsPerStream = 10
	const numStreams = 5

	var wg sync.WaitGroup
	wg.Add(numStreams)

	for j := 0; j < numStreams; j++ {
		go func() {
			defer wg.Done()

			client := arrowpb.NewArrowTracesServiceClient(cc)
			stream, err := client.ArrowTraces(ctx, grpc.WaitForReady(true))
			assert.NoError(t, err)
			producer := arrowRecord.NewProducer()

			var headerBuf bytes.Buffer
			hpd := hpack.NewEncoder(&headerBuf)

			// Repeatedly send traces via arrow. Set the expected traces
			// metadata to receive.
			for i := 0; i < itemsPerStream; i++ {
				td := testdata.GenerateTraces(2)

				headerBuf.Reset()
				err := hpd.WriteField(hpack.HeaderField{
					Name:  "seq",
					Value: strconv.Itoa(i),
				})
				assert.NoError(t, err)

				batch, err := producer.BatchArrowRecordsFromTraces(td)
				assert.NoError(t, err)

				batch.Headers = headerBuf.Bytes()

				err = stream.Send(batch)

				assert.NoError(t, err)

				resp, err := stream.Recv()
				assert.NoError(t, err)
				assert.Equal(t, batch.BatchId, resp.BatchId)
				assert.Equal(t, arrowpb.StatusCode_OK, resp.StatusCode)
			}
		}()
	}
	wg.Wait()

	assert.NoError(t, cc.Close())
	require.NoError(t, ocr.Shutdown(context.Background()))

	counts := make([]int, itemsPerStream)

	// Two spans per stream/item.
	require.Equal(t, itemsPerStream*numStreams*2, sink.SpanCount())
	require.Len(t, sink.Metadatas(), itemsPerStream*numStreams)

	for _, md := range sink.Metadatas() {
		val, err := strconv.Atoi(md.Get("seq")[0])
		require.NoError(t, err)
		counts[val]++
	}

	for i := 0; i < itemsPerStream; i++ {
		require.Equal(t, numStreams, counts[i])
	}
}

// TestOTelArrowHalfOpenShutdown exercises a known condition in which Shutdown
// can't succeed until the stream is canceled by an external signal.
func TestOTelArrowHalfOpenShutdown(t *testing.T) {
	ctx, testCancel := context.WithCancel(context.Background())
	defer testCancel()

	endpointGrpc := testutil.GetAvailableLocalAddress(t)

	nextSink := new(consumertest.TracesSink)

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.GRPC.Keepalive = &configgrpc.KeepaliveServerConfig{
		ServerParameters: &configgrpc.KeepaliveServerParameters{},
	}
	// No keepalive parameters are set
	cfg.GRPC.NetAddr.Endpoint = endpointGrpc
	set := receivertest.NewNopSettings(componentmetadata.Type)

	set.ID = testReceiverID
	r, err := NewFactory().CreateTraces(
		ctx,
		set,
		cfg,
		nextSink)
	require.NoError(t, err)
	require.NotNil(t, r)
	require.NoError(t, r.Start(context.Background(), componenttest.NewNopHost()))

	conn, err := grpc.NewClient(endpointGrpc, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	defer func() {
		require.NoError(t, conn.Close())
	}()

	client := arrowpb.NewArrowTracesServiceClient(conn)
	stream, err := client.ArrowTraces(ctx, grpc.WaitForReady(true))
	require.NoError(t, err)
	producer := arrowRecord.NewProducer()

	start := time.Now()

	// Send traces to the receiver until we signal.
	go func() {
		for time.Since(start) < 5*time.Second {
			select {
			case <-ctx.Done():
				return
			default:
			}
			td := testdata.GenerateTraces(1)
			batch, batchErr := producer.BatchArrowRecordsFromTraces(td)
			assert.NoError(t, batchErr)

			sendErr := stream.Send(batch)
			select {
			case <-ctx.Done():
				if sendErr != nil {
					assert.ErrorIs(t, sendErr, io.EOF)
				}
				return
			default:
				assert.NoError(t, sendErr)
			}
		}
	}()

	// Do not receive batch responses.

	// Wait until the receiver outputs anything to the sink.
	assert.Eventually(t, func() bool {
		return nextSink.SpanCount() > 0
	}, time.Second, 10*time.Millisecond)

	// Let more load pile up.
	time.Sleep(time.Second)

	// The receiver has wedged itself in a call to Send() that is blocked
	// and there is not a graceful way to recover.  Schedule an operation
	// that will unblock it un-gracefully.
	go func() {
		// Without this cancel, the test hangs.
		time.Sleep(3 * time.Second)
		testCancel()
	}()

	// Now shutdown the receiver, while continuing sending traces to it.
	err = r.Shutdown(context.Background())
	assert.NoError(t, err)

	// Ensure that calls to Recv() get canceled
	for {
		_, err := stream.Recv()
		if err == nil {
			continue
		}
		status, ok := status.FromError(err)
		require.True(t, ok, "is a status error")
		require.Equal(t, codes.Canceled, status.Code())
		break
	}
}
