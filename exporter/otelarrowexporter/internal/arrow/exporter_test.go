// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package arrow

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	arrowpb "github.com/open-telemetry/otel-arrow/api/experimental/arrow/v1"
	arrowRecord "github.com/open-telemetry/otel-arrow/pkg/otel/arrow_record"
	arrowRecordMock "github.com/open-telemetry/otel-arrow/pkg/otel/arrow_record/mock"
	otelAssert "github.com/open-telemetry/otel-arrow/pkg/otel/assert"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap/zaptest"
	"golang.org/x/net/http2/hpack"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/grpcutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/otelarrow/netstats"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/otelarrow/testdata"
)

var AllPrioritizers = []PrioritizerName{LeastLoadedPrioritizer, LeastLoadedTwoPrioritizer}

const defaultMaxStreamLifetime = 11 * time.Second

type (
	compareJSONTraces  struct{ ptrace.Traces }
	compareJSONMetrics struct{ pmetric.Metrics }
	compareJSONLogs    struct{ plog.Logs }
)

func (c compareJSONTraces) MarshalJSON() ([]byte, error) {
	var m ptrace.JSONMarshaler
	return m.MarshalTraces(c.Traces)
}

func (c compareJSONMetrics) MarshalJSON() ([]byte, error) {
	var m pmetric.JSONMarshaler
	return m.MarshalMetrics(c.Metrics)
}

func (c compareJSONLogs) MarshalJSON() ([]byte, error) {
	var m plog.JSONMarshaler
	return m.MarshalLogs(c.Logs)
}

type exporterTestCase struct {
	*commonTestCase
	exporter *Exporter
}

func newSingleStreamTestCase(t *testing.T, pname PrioritizerName) *exporterTestCase {
	return newExporterTestCaseCommon(t, pname, NotNoisy, defaultMaxStreamLifetime, 1, false, nil)
}

func newShortLifetimeStreamTestCase(t *testing.T, pname PrioritizerName, numStreams int) *exporterTestCase {
	return newExporterTestCaseCommon(t, pname, NotNoisy, time.Second/2, numStreams, false, nil)
}

func newSingleStreamDowngradeDisabledTestCase(t *testing.T, pname PrioritizerName) *exporterTestCase {
	return newExporterTestCaseCommon(t, pname, NotNoisy, defaultMaxStreamLifetime, 1, true, nil)
}

func newSingleStreamMetadataTestCase(t *testing.T) *exporterTestCase {
	var count int
	return newExporterTestCaseCommon(t, DefaultPrioritizer, NotNoisy, defaultMaxStreamLifetime, 1, false, func(_ context.Context) (map[string]string, error) {
		defer func() { count++ }()
		if count%2 == 0 {
			return nil, nil
		}
		return map[string]string{
			"expected1": "metadata1",
			"expected2": fmt.Sprint(count),
		}, nil
	})
}

func newExporterNoisyTestCase(t *testing.T, numStreams int) *exporterTestCase {
	return newExporterTestCaseCommon(t, DefaultPrioritizer, Noisy, defaultMaxStreamLifetime, numStreams, false, nil)
}

func copyBatch[T any](recordFunc func(T) (*arrowpb.BatchArrowRecords, error)) func(T) (*arrowpb.BatchArrowRecords, error) {
	// Because Arrow-IPC uses zero copy, we have to copy inside the test
	// instead of sharing pointers to BatchArrowRecords.
	return func(data T) (*arrowpb.BatchArrowRecords, error) {
		in, err := recordFunc(data)
		if err != nil {
			return nil, err
		}

		hcpy := make([]byte, len(in.Headers))
		copy(hcpy, in.Headers)

		pays := make([]*arrowpb.ArrowPayload, len(in.ArrowPayloads))

		for i, inp := range in.ArrowPayloads {
			rcpy := make([]byte, len(inp.Record))
			copy(rcpy, inp.Record)
			pays[i] = &arrowpb.ArrowPayload{
				SchemaId: inp.SchemaId,
				Type:     inp.Type,
				Record:   rcpy,
			}
		}

		return &arrowpb.BatchArrowRecords{
			BatchId:       in.BatchId,
			Headers:       hcpy,
			ArrowPayloads: pays,
		}, nil
	}
}

func mockArrowProducer(ctc *commonTestCase) func() arrowRecord.ProducerAPI {
	return func() arrowRecord.ProducerAPI {
		// Mock the close function, use a real producer for testing dataflow.
		mock := arrowRecordMock.NewMockProducerAPI(ctc.ctrl)
		prod := arrowRecord.NewProducer()

		mock.EXPECT().BatchArrowRecordsFromTraces(gomock.Any()).AnyTimes().DoAndReturn(
			copyBatch(prod.BatchArrowRecordsFromTraces))
		mock.EXPECT().BatchArrowRecordsFromLogs(gomock.Any()).AnyTimes().DoAndReturn(
			copyBatch(prod.BatchArrowRecordsFromLogs))
		mock.EXPECT().BatchArrowRecordsFromMetrics(gomock.Any()).AnyTimes().DoAndReturn(
			copyBatch(prod.BatchArrowRecordsFromMetrics))
		mock.EXPECT().Close().Times(1).Return(nil)
		return mock
	}
}

func newExporterTestCaseCommon(t zaptest.TestingT, pname PrioritizerName, noisy noisyTest, maxLifetime time.Duration, numStreams int, disableDowngrade bool, metadataFunc func(ctx context.Context) (map[string]string, error)) *exporterTestCase {
	ctc := newCommonTestCase(t, noisy)

	if metadataFunc == nil {
		ctc.requestMetadataCall.AnyTimes().Return(nil, nil)
	} else {
		ctc.requestMetadataCall.AnyTimes().DoAndReturn(func(ctx context.Context, _ ...string) (map[string]string, error) {
			return metadataFunc(ctx)
		})
	}

	exp := NewExporter(maxLifetime, numStreams, pname, disableDowngrade, ctc.telset, nil, mockArrowProducer(ctc), ctc.traceClient, ctc.perRPCCredentials, netstats.Noop{})

	return &exporterTestCase{
		commonTestCase: ctc,
		exporter:       exp,
	}
}

func statusOKFor(id int64) *arrowpb.BatchStatus {
	return &arrowpb.BatchStatus{
		BatchId:    id,
		StatusCode: arrowpb.StatusCode_OK,
	}
}

func statusUnavailableFor(id int64) *arrowpb.BatchStatus {
	return &arrowpb.BatchStatus{
		BatchId:       id,
		StatusCode:    arrowpb.StatusCode_UNAVAILABLE,
		StatusMessage: "test unavailable",
	}
}

func statusInvalidFor(id int64) *arrowpb.BatchStatus {
	return &arrowpb.BatchStatus{
		BatchId:       id,
		StatusCode:    arrowpb.StatusCode_INVALID_ARGUMENT,
		StatusMessage: "test invalid",
	}
}

func statusUnrecognizedFor(id int64) *arrowpb.BatchStatus {
	return &arrowpb.BatchStatus{
		BatchId:       id,
		StatusCode:    1 << 20,
		StatusMessage: "test unrecognized",
	}
}

// TestArrowExporterSuccess tests a single Send through a healthy channel.
func TestArrowExporterSuccess(t *testing.T) {
	stdTesting := otelAssert.NewStdUnitTest(t)
	for _, pname := range AllPrioritizers {
		t.Run(string(pname), func(t *testing.T) {
			for _, inputData := range []any{twoTraces, twoMetrics, twoLogs} {
				t.Run(fmt.Sprintf("%T", inputData), func(t *testing.T) {
					tc := newSingleStreamTestCase(t, pname)
					channel := newHealthyTestChannel()

					tc.traceCall.Times(1).DoAndReturn(tc.returnNewStream(channel))

					ctx := context.Background()
					require.NoError(t, tc.exporter.Start(ctx))

					var wg sync.WaitGroup
					var outputData *arrowpb.BatchArrowRecords
					wg.Add(1)
					go func() {
						defer wg.Done()
						outputData = <-channel.sendChannel()
						channel.recv <- statusOKFor(outputData.BatchId)
					}()

					sent, err := tc.exporter.SendAndWait(ctx, inputData)
					require.NoError(t, err)
					require.True(t, sent)

					wg.Wait()

					testCon := arrowRecord.NewConsumer()
					switch testData := inputData.(type) {
					case ptrace.Traces:
						traces, err := testCon.TracesFrom(outputData)
						require.NoError(t, err)
						require.Len(t, traces, 1)
						otelAssert.Equiv(stdTesting, []json.Marshaler{
							compareJSONTraces{testData},
						}, []json.Marshaler{
							compareJSONTraces{traces[0]},
						})
					case plog.Logs:
						logs, err := testCon.LogsFrom(outputData)
						require.NoError(t, err)
						require.Len(t, logs, 1)
						otelAssert.Equiv(stdTesting, []json.Marshaler{
							compareJSONLogs{testData},
						}, []json.Marshaler{
							compareJSONLogs{logs[0]},
						})
					case pmetric.Metrics:
						metrics, err := testCon.MetricsFrom(outputData)
						require.NoError(t, err)
						require.Len(t, metrics, 1)
						otelAssert.Equiv(stdTesting, []json.Marshaler{
							compareJSONMetrics{testData},
						}, []json.Marshaler{
							compareJSONMetrics{metrics[0]},
						})
					}

					require.NoError(t, tc.exporter.Shutdown(ctx))
				})
			}
		})
	}
}

// TestArrowExporterTimeout tests that single slow Send leads to context canceled.
func TestArrowExporterTimeout(t *testing.T) {
	for _, pname := range AllPrioritizers {
		t.Run(string(pname), func(t *testing.T) {
			tc := newSingleStreamTestCase(t, pname)
			channel := newUnresponsiveTestChannel()

			tc.traceCall.Times(1).DoAndReturn(tc.returnNewStream(channel))

			ctx, cancel := context.WithCancel(context.Background())
			require.NoError(t, tc.exporter.Start(ctx))

			go func() {
				time.Sleep(200 * time.Millisecond)
				cancel()
			}()
			sent, err := tc.exporter.SendAndWait(ctx, twoTraces)
			require.True(t, sent)
			require.Error(t, err)

			stat, is := status.FromError(err)
			require.True(t, is, "is a gRPC status")
			require.Equal(t, codes.Canceled, stat.Code())

			// Repeat the request, will get immediate timeout.
			sent, err = tc.exporter.SendAndWait(ctx, twoTraces)
			require.False(t, sent)
			stat, is = status.FromError(err)
			require.True(t, is, "is a gRPC status error: %v", err)
			require.Equal(t, "context done before send: context canceled", stat.Message())
			require.Equal(t, codes.Canceled, stat.Code())

			require.NoError(t, tc.exporter.Shutdown(ctx))
		})
	}
}

// TestConnectError tests that if the connetions fail fast the
// stream object for some reason is nil.  This causes downgrade.
func TestArrowExporterStreamConnectError(t *testing.T) {
	for _, pname := range AllPrioritizers {
		t.Run(string(pname), func(t *testing.T) {
			tc := newSingleStreamTestCase(t, pname)
			channel := newConnectErrorTestChannel()

			tc.traceCall.AnyTimes().DoAndReturn(tc.returnNewStream(channel))

			bg := context.Background()
			require.NoError(t, tc.exporter.Start(bg))

			sent, err := tc.exporter.SendAndWait(bg, twoTraces)
			require.False(t, sent)
			require.NoError(t, err)

			require.NoError(t, tc.exporter.Shutdown(bg))

			require.NotEmpty(t, tc.observedLogs.All(), "should have at least one log: %v", tc.observedLogs.All())
			require.Equal(t, "cannot start arrow stream", tc.observedLogs.All()[0].Message)
		})
	}
}

// TestArrowExporterDowngrade tests that if the Recv() returns an
// Unimplemented code (as gRPC does) that the connection is downgraded
// without error.
func TestArrowExporterDowngrade(t *testing.T) {
	for _, pname := range AllPrioritizers {
		t.Run(string(pname), func(t *testing.T) {
			tc := newSingleStreamTestCase(t, pname)
			channel := newArrowUnsupportedTestChannel()

			tc.traceCall.AnyTimes().DoAndReturn(tc.returnNewStream(channel))

			bg := context.Background()
			require.NoError(t, tc.exporter.Start(bg))

			sent, err := tc.exporter.SendAndWait(bg, twoTraces)
			require.False(t, sent)
			require.NoError(t, err)

			require.NoError(t, tc.exporter.Shutdown(bg))

			require.Less(t, 1, len(tc.observedLogs.All()), "should have at least two logs: %v", tc.observedLogs.All())
			require.Equal(t, "arrow is not supported", tc.observedLogs.All()[0].Message)
			require.Contains(t, tc.observedLogs.All()[1].Message, "downgrading")
		})
	}
}

// TestArrowExporterDisableDowngrade tests that if the Recv() returns
// any error downgrade still does not occur amd that the connection is
// retried without error.
func TestArrowExporterDisableDowngrade(t *testing.T) {
	for _, pname := range AllPrioritizers {
		t.Run(string(pname), func(t *testing.T) {
			tc := newSingleStreamDowngradeDisabledTestCase(t, pname)
			badChannel := newArrowUnsupportedTestChannel()
			goodChannel := newHealthyTestChannel()

			fails := 0
			tc.traceCall.AnyTimes().DoAndReturn(func(ctx context.Context, opts ...grpc.CallOption) (
				arrowpb.ArrowTracesService_ArrowTracesClient,
				error,
			) {
				defer func() { fails++ }()

				if fails < 3 {
					return tc.returnNewStream(badChannel)(ctx, opts...)
				}
				return tc.returnNewStream(goodChannel)(ctx, opts...)
			})

			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				defer wg.Done()
				outputData := <-goodChannel.sendChannel()
				goodChannel.recv <- statusOKFor(outputData.BatchId)
			}()

			bg := context.Background()
			require.NoError(t, tc.exporter.Start(bg))

			sent, err := tc.exporter.SendAndWait(bg, twoTraces)
			require.True(t, sent)
			require.NoError(t, err)

			wg.Wait()

			require.NoError(t, tc.exporter.Shutdown(bg))

			require.Less(t, 1, len(tc.observedLogs.All()), "should have at least two logs: %v", tc.observedLogs.All())
			require.Equal(t, "arrow is not supported", tc.observedLogs.All()[0].Message)
			require.NotContains(t, tc.observedLogs.All()[1].Message, "downgrading")
		})
	}
}

// TestArrowExporterConnectTimeout tests that an error is returned to
// the caller if the response does not arrive in time.
func TestArrowExporterConnectTimeout(t *testing.T) {
	for _, pname := range AllPrioritizers {
		t.Run(string(pname), func(t *testing.T) {
			tc := newSingleStreamTestCase(t, pname)
			channel := newDisconnectedTestChannel()

			tc.traceCall.AnyTimes().DoAndReturn(tc.returnNewStream(channel))

			bg := context.Background()
			ctx, cancel := context.WithCancel(bg)
			require.NoError(t, tc.exporter.Start(bg))

			go func() {
				time.Sleep(200 * time.Millisecond)
				cancel()
			}()
			_, err := tc.exporter.SendAndWait(ctx, twoTraces)
			require.Error(t, err)

			stat, is := status.FromError(err)
			require.True(t, is, "is a gRPC status error: %v", err)
			require.Equal(t, codes.Canceled, stat.Code())

			require.NoError(t, tc.exporter.Shutdown(bg))
		})
	}
}

// TestArrowExporterStreamFailure tests that a single stream failure
// followed by a healthy stream.
func TestArrowExporterStreamFailure(t *testing.T) {
	for _, pname := range AllPrioritizers {
		t.Run(string(pname), func(t *testing.T) {
			tc := newSingleStreamTestCase(t, pname)
			channel0 := newUnresponsiveTestChannel()
			channel1 := newHealthyTestChannel()

			tc.traceCall.AnyTimes().DoAndReturn(tc.returnNewStream(channel0, channel1))

			bg := context.Background()
			require.NoError(t, tc.exporter.Start(bg))

			go func() {
				time.Sleep(200 * time.Millisecond)
				channel0.unblock()
			}()

			var wg sync.WaitGroup
			var outputData *arrowpb.BatchArrowRecords
			wg.Add(1)
			go func() {
				defer wg.Done()
				outputData = <-channel1.sendChannel()
				channel1.recv <- statusOKFor(outputData.BatchId)
			}()

			sent, err := tc.exporter.SendAndWait(bg, twoTraces)
			require.NoError(t, err)
			require.True(t, sent)

			wg.Wait()

			require.NoError(t, tc.exporter.Shutdown(bg))
		})
	}
}

// TestArrowExporterStreamRace reproduces the situation needed for a
// race between stream send and stream cancel, causing it to fully
// exercise the removeReady() code path.
func TestArrowExporterStreamRace(t *testing.T) {
	// This creates the conditions likely to produce a
	// stream race in prioritizer.go.
	tc := newExporterNoisyTestCase(t, 20)

	var tries atomic.Int32

	tc.traceCall.AnyTimes().DoAndReturn(tc.repeatedNewStream(func() testChannel {
		noResponse := newUnresponsiveTestChannel()
		// Immediately unblock to return the EOF to the stream
		// receiver and shut down the stream.
		go noResponse.unblock()
		tries.Add(1)
		return noResponse
	}))

	var wg sync.WaitGroup

	bg := context.Background()
	require.NoError(t, tc.exporter.Start(bg))

	callctx, cancel := context.WithCancel(bg)

	// These goroutines will repeatedly try for an available
	// stream, but none will become available.  Eventually the
	// context will be canceled and cause these goroutines to
	// return.
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// This blocks until the cancelation.
			_, err := tc.exporter.SendAndWait(callctx, twoTraces)
			assert.Error(t, err)

			stat, is := status.FromError(err)
			assert.True(t, is, "is a gRPC status error: %v", err)
			assert.Equal(t, codes.Canceled, stat.Code())
		}()
	}

	// Wait until 1000 streams have started.
	assert.Eventually(t, func() bool {
		return tries.Load() >= 1000
	}, 10*time.Second, 5*time.Millisecond)

	cancel()
	wg.Wait()
	require.NoError(t, tc.exporter.Shutdown(bg))
}

// TestArrowExporterStreaming tests 10 sends in a row.
func TestArrowExporterStreaming(t *testing.T) {
	for _, pname := range AllPrioritizers {
		t.Run(string(pname), func(t *testing.T) {
			tc := newSingleStreamTestCase(t, pname)
			channel := newHealthyTestChannel()

			tc.traceCall.AnyTimes().DoAndReturn(tc.returnNewStream(channel))

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			require.NoError(t, tc.exporter.Start(ctx))

			var expectOutput []ptrace.Traces
			var actualOutput []ptrace.Traces
			testCon := arrowRecord.NewConsumer()

			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				defer wg.Done()
				for data := range channel.sendChannel() {
					traces, err := testCon.TracesFrom(data)
					assert.NoError(t, err)
					assert.Len(t, traces, 1)
					actualOutput = append(actualOutput, traces[0])
					channel.recv <- statusOKFor(data.BatchId)
				}
			}()

			for times := 0; times < 10; times++ {
				input := testdata.GenerateTraces(2)

				sent, err := tc.exporter.SendAndWait(context.Background(), input)
				require.NoError(t, err)
				require.True(t, sent)

				expectOutput = append(expectOutput, input)
			}
			// Stop the test conduit started above.
			cancel()
			wg.Wait()

			// As this equality check doesn't support out of order slices,
			// we sort the slices directly in the GenerateTraces function.
			require.Equal(t, expectOutput, actualOutput)
			require.NoError(t, tc.exporter.Shutdown(ctx))
		})
	}
}

// TestArrowExporterHeaders tests a mix of outgoing context headers.
func TestArrowExporterHeaders(t *testing.T) {
	for _, withDeadline := range []bool{true, false} {
		t.Run(fmt.Sprint("with_deadline=", withDeadline), func(t *testing.T) {
			tc := newSingleStreamMetadataTestCase(t)
			channel := newHealthyTestChannel()

			tc.traceCall.AnyTimes().DoAndReturn(tc.returnNewStream(channel))

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			require.NoError(t, tc.exporter.Start(ctx))

			var expectOutput []metadata.MD
			var actualOutput []metadata.MD

			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				defer wg.Done()
				md := metadata.MD{}
				hpd := hpack.NewDecoder(4096, func(f hpack.HeaderField) {
					md[f.Name] = append(md[f.Name], f.Value)
				})
				for data := range channel.sendChannel() {
					if len(data.Headers) == 0 {
						actualOutput = append(actualOutput, nil)
					} else {
						_, err := hpd.Write(data.Headers)
						assert.NoError(t, err)
						actualOutput = append(actualOutput, md)
						md = metadata.MD{}
					}
					channel.recv <- statusOKFor(data.BatchId)
				}
			}()

			for times := 0; times < 10; times++ {
				input := testdata.GenerateTraces(2)

				if times%2 == 1 {
					md := metadata.MD{
						"expected1":       []string{"metadata1"},
						"expected2":       []string{fmt.Sprint(times)},
						"otlp-pdata-size": []string{"329"},
					}
					expectOutput = append(expectOutput, md)
				} else {
					expectOutput = append(expectOutput, metadata.MD{
						"otlp-pdata-size": []string{"329"},
					})
				}

				sendCtx := ctx
				if withDeadline {
					var sendCancel context.CancelFunc
					sendCtx, sendCancel = context.WithTimeout(sendCtx, time.Second)
					defer sendCancel()
				}

				sent, err := tc.exporter.SendAndWait(sendCtx, input)
				require.NoError(t, err)
				require.True(t, sent)
			}
			// Stop the test conduit started above.
			cancel()
			wg.Wait()

			// Manual check for proper deadline propagation.  Since the test
			// is timed we don't expect an exact match.
			if withDeadline {
				for _, out := range actualOutput {
					dead := out.Get("grpc-timeout")
					require.Len(t, dead, 1)
					require.NotEmpty(t, dead[0])
					to, err := grpcutil.DecodeTimeout(dead[0])
					require.NoError(t, err)
					// Allow the test to lapse for 0.5s.
					require.Less(t, time.Second/2, to)
					require.GreaterOrEqual(t, time.Second, to)
					out.Delete("grpc-timeout")
				}
			}

			require.Equal(t, expectOutput, actualOutput)
			require.NoError(t, tc.exporter.Shutdown(ctx))
		})
	}
}

// TestArrowExporterIsTraced tests whether trace and span ID are
// propagated.
func TestArrowExporterIsTraced(t *testing.T) {
	otel.SetTextMapPropagator(propagation.TraceContext{})

	for _, pname := range AllPrioritizers {
		t.Run(string(pname), func(t *testing.T) {
			tc := newSingleStreamTestCase(t, pname)
			channel := newHealthyTestChannel()

			tc.traceCall.AnyTimes().DoAndReturn(tc.returnNewStream(channel))

			ctx, cancel := context.WithCancel(context.Background())
			require.NoError(t, tc.exporter.Start(ctx))

			var expectOutput []metadata.MD
			var actualOutput []metadata.MD

			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				defer wg.Done()
				md := metadata.MD{}
				hpd := hpack.NewDecoder(4096, func(f hpack.HeaderField) {
					md[f.Name] = append(md[f.Name], f.Value)
				})
				for data := range channel.sendChannel() {
					if len(data.Headers) == 0 {
						actualOutput = append(actualOutput, nil)
					} else {
						_, err := hpd.Write(data.Headers)
						assert.NoError(t, err)
						actualOutput = append(actualOutput, md)
						md = metadata.MD{}
					}
					channel.recv <- statusOKFor(data.BatchId)
				}
			}()

			for times := 0; times < 10; times++ {
				input := testdata.GenerateTraces(2)
				callCtx := context.Background()

				if times%2 == 1 {
					callCtx = trace.ContextWithSpanContext(callCtx,
						trace.NewSpanContext(trace.SpanContextConfig{
							TraceID: [16]byte{byte(times), 1, 2, 3, 4, 5, 6, 7, 8, 9, 0xa, 0xb, 0xc, 0xd, 0xe, 0xf},
							SpanID:  [8]byte{byte(times), 1, 2, 3, 4, 5, 6, 7},
						}),
					)
					expectMap := map[string]string{}
					propagation.TraceContext{}.Inject(callCtx, propagation.MapCarrier(expectMap))

					md := metadata.MD{
						"traceparent":     []string{expectMap["traceparent"]},
						"otlp-pdata-size": []string{"329"},
					}
					expectOutput = append(expectOutput, md)
				} else {
					expectOutput = append(expectOutput, metadata.MD{
						"otlp-pdata-size": []string{"329"},
					})
				}

				sent, err := tc.exporter.SendAndWait(callCtx, input)
				require.NoError(t, err)
				require.True(t, sent)
			}
			// Stop the test conduit started above.
			cancel()
			wg.Wait()

			require.Equal(t, expectOutput, actualOutput)
			require.NoError(t, tc.exporter.Shutdown(ctx))
		})
	}
}

func TestAddJitter(t *testing.T) {
	require.Equal(t, time.Duration(0), addJitter(0))

	// Expect no more than 5% less in each trial.
	for i := 0; i < 100; i++ {
		x := addJitter(20 * time.Minute)
		require.LessOrEqual(t, 19*time.Minute, x)
		require.Less(t, x, 20*time.Minute)
	}
}

// TestArrowExporterStreamLifetimeAndShutdown exercises multiple
// stream lifetimes and then shuts down, inspects the logs for
// legibility.
func TestArrowExporterStreamLifetimeAndShutdown(t *testing.T) {
	for _, pname := range AllPrioritizers {
		t.Run(string(pname), func(t *testing.T) {
			for _, numStreams := range []int{1, 2, 8} {
				t.Run(fmt.Sprint(numStreams), func(t *testing.T) {
					tc := newShortLifetimeStreamTestCase(t, pname, numStreams)
					ctx, cancel := context.WithCancel(context.Background())
					defer cancel()

					var wg sync.WaitGroup

					var expectCount uint64
					var actualCount uint64

					tc.traceCall.AnyTimes().DoAndReturn(func(ctx context.Context, opts ...grpc.CallOption) (
						arrowpb.ArrowTracesService_ArrowTracesClient,
						error,
					) {
						wg.Add(1)
						channel := newHealthyTestChannel()

						go func() {
							defer wg.Done()
							testCon := arrowRecord.NewConsumer()

							for data := range channel.sendChannel() {
								traces, err := testCon.TracesFrom(data)
								assert.NoError(t, err)
								assert.Len(t, traces, 1)
								atomic.AddUint64(&actualCount, 1)
								channel.recv <- statusOKFor(data.BatchId)
							}

							// Closing the recv channel causes the exporter to see EOF.
							close(channel.recv)
						}()

						return tc.returnNewStream(channel)(ctx, opts...)
					})

					require.NoError(t, tc.exporter.Start(ctx))

					start := time.Now()
					// This is 10 stream lifetimes using the "ShortLifetime" test.
					for time.Since(start) < 5*time.Second {
						input := testdata.GenerateTraces(2)

						sent, err := tc.exporter.SendAndWait(ctx, input)
						require.NoError(t, err)
						require.True(t, sent)

						expectCount++
					}

					require.NoError(t, tc.exporter.Shutdown(ctx))

					require.Equal(t, expectCount, actualCount)

					cancel()
					wg.Wait()

					require.Empty(t, tc.observedLogs.All())
				})
			}
		})
	}
}

func BenchmarkLeastLoadedTwo4(b *testing.B) {
	benchmarkPrioritizer(b, 4, LeastLoadedTwoPrioritizer)
}

func BenchmarkLeastLoadedTwo8(b *testing.B) {
	benchmarkPrioritizer(b, 8, LeastLoadedTwoPrioritizer)
}

func BenchmarkLeastLoadedTwo16(b *testing.B) {
	benchmarkPrioritizer(b, 16, LeastLoadedTwoPrioritizer)
}

func BenchmarkLeastLoadedTwo32(b *testing.B) {
	benchmarkPrioritizer(b, 32, LeastLoadedTwoPrioritizer)
}

func BenchmarkLeastLoadedTwo64(b *testing.B) {
	benchmarkPrioritizer(b, 64, LeastLoadedTwoPrioritizer)
}

func BenchmarkLeastLoadedTwo128(b *testing.B) {
	benchmarkPrioritizer(b, 128, LeastLoadedTwoPrioritizer)
}

func BenchmarkLeastLoadedFour4(b *testing.B) {
	benchmarkPrioritizer(b, 4, LeastLoadedFourPrioritizer)
}

func BenchmarkLeastLoadedFour8(b *testing.B) {
	benchmarkPrioritizer(b, 8, LeastLoadedFourPrioritizer)
}

func BenchmarkLeastLoadedFour16(b *testing.B) {
	benchmarkPrioritizer(b, 16, LeastLoadedFourPrioritizer)
}

func BenchmarkLeastLoadedFour32(b *testing.B) {
	benchmarkPrioritizer(b, 32, LeastLoadedFourPrioritizer)
}

func BenchmarkLeastLoadedFour64(b *testing.B) {
	benchmarkPrioritizer(b, 64, LeastLoadedFourPrioritizer)
}

func BenchmarkLeastLoadedFour128(b *testing.B) {
	benchmarkPrioritizer(b, 128, LeastLoadedFourPrioritizer)
}

func benchmarkPrioritizer(b *testing.B, numStreams int, pname PrioritizerName) {
	tc := newExporterTestCaseCommon(z2m{b}, pname, Noisy, defaultMaxStreamLifetime, numStreams, true, nil)

	var wg sync.WaitGroup
	var cnt atomic.Int32

	tc.traceCall.AnyTimes().DoAndReturn(func(ctx context.Context, opts ...grpc.CallOption) (
		arrowpb.ArrowTracesService_ArrowTracesClient,
		error,
	) {
		wg.Add(1)
		num := cnt.Add(1)
		channel := newHealthyTestChannel()

		delay := time.Duration(num) * time.Millisecond

		go func() {
			defer wg.Done()
			var mine sync.WaitGroup
			for data := range channel.sendChannel() {
				mine.Add(1)
				go func(<-chan time.Time) {
					defer mine.Done()
					channel.recv <- statusOKFor(data.BatchId)
				}(time.After(delay))
			}

			mine.Wait()

			close(channel.recv)
		}()

		return tc.returnNewStream(channel)(ctx, opts...)
	})

	bg, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := tc.exporter.Start(bg); err != nil {
		b.Errorf("start failed: %v", err)
		return
	}

	input := testdata.GenerateTraces(2)

	wg.Add(1)
	defer func() {
		assert.NoError(b, tc.exporter.Shutdown(bg), "shutdown failed")
		wg.Done()
		wg.Wait()
	}()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		sent, err := tc.exporter.SendAndWait(bg, input)
		if err != nil || !sent {
			b.Errorf("send failed: %v: %v", sent, err)
		}
	}
}
