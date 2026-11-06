// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package arrow

import (
	"context"
	"errors"
	"testing"
	"testing/synctest"
	"time"

	arrowpb "github.com/open-telemetry/otel-arrow/go/api/experimental/arrow/v1"
	arrowRecord "github.com/open-telemetry/otel-arrow/go/pkg/otel/arrow_record"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/configretry"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func testStreamRestartConfig() configretry.BackOffConfig {
	return configretry.BackOffConfig{
		Enabled:         true,
		InitialInterval: time.Second,
		MaxInterval:     4 * time.Second,
		Multiplier:      2,
	}
}

func TestStreamRestartConfigDefaults(t *testing.T) {
	expected := configretry.NewDefaultBackOffConfig()
	expected.MaxElapsedTime = 0
	require.Equal(t, expected, NewDefaultStreamRestartConfig())
}

// A successful gRPC constructor is not proof of a healthy stream: failures
// commonly arrive at Recv instead. Both paths must retain exponential backoff.
func TestArrowExporterStreamRestartBackoff(t *testing.T) {
	for _, failOnRecv := range []bool{false, true} {
		t.Run(map[bool]string{false: "connect", true: "recv"}[failOnRecv], func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				tc := newSingleStreamTestCase(t, DefaultPrioritizer)
				tc.exporter.streamRestart = testStreamRestartConfig()
				ctx, cancel := context.WithCancel(t.Context())
				defer cancel()
				var attempts []time.Time
				tc.traceCall.AnyTimes().DoAndReturn(func(ctx context.Context, _ ...grpc.CallOption) (arrowpb.ArrowTracesService_ArrowTracesClient, error) {
					attempts = append(attempts, time.Now())
					if len(attempts) == 5 {
						cancel()
					}
					if !failOnRecv {
						return nil, status.Error(codes.Unavailable, "connection refused")
					}
					str := tc.newMockStream(ctx)
					str.recvCall.Times(1).Return(nil, status.Error(codes.Unavailable, "server unavailable"))
					str.closeSendCall.Times(1).Return(nil)
					return str.anyStreamClient.(arrowpb.ArrowTracesService_ArrowTracesClient), nil
				})
				require.NoError(t, tc.exporter.Start(ctx))
				<-ctx.Done()
				require.NoError(t, tc.exporter.Shutdown(ctx))
				synctest.Wait()
				require.Len(t, attempts, 5)
				for i, delay := range []time.Duration{time.Second, 2 * time.Second, 4 * time.Second, 4 * time.Second} {
					require.Equal(t, delay, attempts[i+1].Sub(attempts[i]))
				}
			})
		})
	}
}

func TestArrowExporterShutdownDuringBackoff(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		tc := newSingleStreamTestCase(t, DefaultPrioritizer)
		tc.exporter.streamRestart = testStreamRestartConfig()
		attempts := 0
		allocations := 0
		newProducer := tc.exporter.newProducer
		tc.exporter.newProducer = func() arrowRecord.ProducerAPI {
			allocations++
			return newProducer()
		}
		tc.traceCall.AnyTimes().DoAndReturn(func(context.Context, ...grpc.CallOption) (arrowpb.ArrowTracesService_ArrowTracesClient, error) {
			attempts++
			return nil, errors.New("connection refused")
		})
		require.NoError(t, tc.exporter.Start(t.Context()))
		synctest.Wait()
		require.Equal(t, 1, attempts)
		require.Equal(t, 1, allocations, "a producer should not be allocated during backoff")
		started := time.Now()
		require.NoError(t, tc.exporter.Shutdown(t.Context()))
		synctest.Wait()
		require.Equal(t, time.Duration(0), time.Since(started))
		require.Equal(t, 1, attempts)
		require.Equal(t, 1, allocations)
	})
}

func TestArrowExporterNormalRotationSkipsBackoff(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		tc := newExporterTestCaseCommon(t, DefaultPrioritizer, NotNoisy, time.Second, 1, false, nil)
		tc.exporter.streamRestart = testStreamRestartConfig()
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		healthy := newHealthyTestChannel()
		go func() {
			// CloseSend marks the end of the normal lifetime. The server
			// responds with EOF, as it would for a graceful stream rotation.
			<-healthy.sendChannel()
			close(healthy.recv)
		}()
		var attempts []time.Time
		tc.traceCall.AnyTimes().DoAndReturn(func(ctx context.Context, opts ...grpc.CallOption) (arrowpb.ArrowTracesService_ArrowTracesClient, error) {
			attempts = append(attempts, time.Now())
			if len(attempts) == 1 {
				return tc.returnNewStream(healthy)(ctx, opts...)
			}
			cancel()
			return nil, errors.New("test complete")
		})
		require.NoError(t, tc.exporter.Start(ctx))
		<-ctx.Done()
		require.NoError(t, tc.exporter.Shutdown(ctx))
		synctest.Wait()
		require.Len(t, attempts, 2)
		require.Less(t, attempts[1].Sub(attempts[0]), 1500*time.Millisecond)
	})
}

func TestArrowExporterBackoffResetsAfterResponse(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		tc := newSingleStreamTestCase(t, DefaultPrioritizer)
		tc.exporter.streamRestart = testStreamRestartConfig()
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		healthy := newHealthyTestChannel()
		var attempts []time.Time
		tc.traceCall.AnyTimes().DoAndReturn(func(ctx context.Context, opts ...grpc.CallOption) (arrowpb.ArrowTracesService_ArrowTracesClient, error) {
			attempts = append(attempts, time.Now())
			if len(attempts) == 3 {
				return tc.returnNewStream(healthy)(ctx, opts...)
			}
			if len(attempts) == 5 {
				cancel()
			}
			return nil, errors.New("connection refused")
		})
		go func() {
			data := <-healthy.sendChannel()
			healthy.recv <- statusOKFor(data.BatchId)
			close(healthy.recv)
		}()
		require.NoError(t, tc.exporter.Start(ctx))
		sent, err := tc.exporter.SendAndWait(ctx, twoTraces)
		require.NoError(t, err)
		require.True(t, sent)
		<-ctx.Done()
		require.NoError(t, tc.exporter.Shutdown(ctx))
		synctest.Wait()
		require.Len(t, attempts, 5)
		for i, delay := range []time.Duration{time.Second, 2 * time.Second, time.Second, 2 * time.Second} {
			require.Equal(t, delay, attempts[i+1].Sub(attempts[i]))
		}
	})
}

func TestArrowExporterBackoffDoesNotBlockOtherStreams(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		tc := newExporterTestCaseCommon(t, DefaultPrioritizer, NotNoisy, time.Minute, 2, false, nil)
		tc.exporter.streamRestart = testStreamRestartConfig()
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		attempts := make(chan time.Time, 4)
		tc.traceCall.AnyTimes().DoAndReturn(func(context.Context, ...grpc.CallOption) (arrowpb.ArrowTracesService_ArrowTracesClient, error) {
			attempts <- time.Now()
			return nil, errors.New("connection refused")
		})
		require.NoError(t, tc.exporter.Start(ctx))
		synctest.Wait()
		first := <-attempts
		require.Equal(t, first, <-attempts)
		time.Sleep(time.Second)
		synctest.Wait()
		for range 2 {
			require.Equal(t, time.Second, (<-attempts).Sub(first))
		}
		require.NoError(t, tc.exporter.Shutdown(ctx))
	})
}

func TestArrowExporterUnsupportedBackoff(t *testing.T) {
	for _, disableDowngrade := range []bool{false, true} {
		t.Run(map[bool]string{false: "downgrade", true: "retry"}[disableDowngrade], func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				tc := newSingleStreamTestCase(t, DefaultPrioritizer)
				tc.exporter.streamRestart = testStreamRestartConfig()
				tc.exporter.disableDowngrade = disableDowngrade
				attempts := 0
				tc.traceCall.AnyTimes().DoAndReturn(func(context.Context, ...grpc.CallOption) (arrowpb.ArrowTracesService_ArrowTracesClient, error) {
					attempts++
					return nil, status.Error(codes.Unimplemented, "no Arrow support")
				})
				require.NoError(t, tc.exporter.Start(t.Context()))
				synctest.Wait()
				if disableDowngrade {
					time.Sleep(time.Second)
					synctest.Wait()
					require.Equal(t, 2, attempts)
				} else {
					sent, err := tc.exporter.SendAndWait(t.Context(), twoTraces)
					require.NoError(t, err)
					require.False(t, sent)
					require.Equal(t, 1, attempts)
				}
				require.NoError(t, tc.exporter.Shutdown(t.Context()))
			})
		})
	}
}
