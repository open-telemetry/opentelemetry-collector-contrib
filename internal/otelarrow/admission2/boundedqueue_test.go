// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package admission2

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/resource"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/otelarrow/netstats"
)

const (
	expectScope        = "github.com/open-telemetry/opentelemetry-collector-contrib/internal/otelarrow"
	expectInFlightName = "otelcol_otelarrow_admission_in_flight_bytes"
	expectWaitingName  = "otelcol_otelarrow_admission_waiting_bytes"
)

type bqTest struct {
	t        *testing.T
	reader   *sdkmetric.ManualReader
	provider *sdkmetric.MeterProvider
	*BoundedQueue
}

func newBQTest(t *testing.T, maxAdmit, maxWait uint64) bqTest {
	settings := componenttest.NewNopTelemetrySettings()

	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(resource.Empty()),
		sdkmetric.WithReader(reader),
	)
	settings.MeterProvider = provider
	bq, err := NewBoundedQueue(component.MustNewID("admission_testing"), settings, maxAdmit, maxWait)
	require.NoError(t, err)
	return bqTest{
		t:            t,
		reader:       reader,
		provider:     provider,
		BoundedQueue: bq.(*BoundedQueue),
	}
}

func (bq *bqTest) startWaiter(ctx context.Context, size uint64, relp *ReleaseFunc) N {
	n := newNotification()
	go func() {
		var err error
		*relp, err = bq.Acquire(ctx, size)
		require.NoError(bq.t, err)
		n.Notify()
	}()
	return n
}

func (bq *bqTest) waitForPending(admitted, waiting uint64) {
	require.Eventually(bq.t, func() bool {
		bq.lock.Lock()
		defer bq.lock.Unlock()
		return bq.currentAdmitted == admitted && bq.currentWaiting == waiting
	}, time.Second, 20*time.Millisecond)
}

func mkRepeat(x uint64, n int) []uint64 {
	if n == 0 {
		return nil
	}
	return append(mkRepeat(x, n-1), x)
}

func mkRange(from, to uint64) []uint64 {
	if from > to {
		return nil
	}
	return append([]uint64{from}, mkRange(from+1, to)...)
}

func TestBoundedQueueLimits(t *testing.T) {
	for _, test := range []struct {
		name           string
		maxLimitAdmit  uint64
		maxLimitWait   uint64
		expectAcquired [2]int64
		requestSizes   []uint64
		timeout        time.Duration
		expectErrs     map[string]int
	}{
		{
			name:           "simple_no_waiters_25",
			maxLimitAdmit:  1000,
			maxLimitWait:   0,
			requestSizes:   mkRepeat(25, 40),
			expectAcquired: [2]int64{1000, 1000},
			timeout:        0,
			expectErrs:     map[string]int{},
		},
		{
			name:           "simple_no_waiters_1",
			maxLimitAdmit:  1000,
			maxLimitWait:   0,
			requestSizes:   mkRepeat(1, 1000),
			expectAcquired: [2]int64{1000, 1000},
			timeout:        0,
			expectErrs:     map[string]int{},
		},
		{
			name:           "without_waiting_remainder",
			maxLimitAdmit:  1000,
			maxLimitWait:   0,
			requestSizes:   mkRepeat(30, 40),
			expectAcquired: [2]int64{990, 990},
			timeout:        0,
			expectErrs: map[string]int{
				// 7 failures with a remainder of 10
				// 30 * (40 - 7) = 990
				ErrTooMuchWaiting.Error(): 7,
			},
		},
		{
			name:           "without_waiting_complete",
			maxLimitAdmit:  1000,
			maxLimitWait:   0,
			requestSizes:   append(mkRepeat(30, 40), 10),
			expectAcquired: [2]int64{1000, 1000},
			timeout:        0,
			expectErrs: map[string]int{
				// 30*33+10 succeed, 7 failures (as above)
				ErrTooMuchWaiting.Error(): 7,
			},
		},
		{
			name:           "with_waiters_timeout",
			maxLimitAdmit:  1000,
			maxLimitWait:   1000,
			requestSizes:   mkRepeat(20, 100),
			expectAcquired: [2]int64{1000, 1000},
			timeout:        time.Second,
			expectErrs: map[string]int{
				// 20*50=1000 is half of the requests timing out
				status.Error(codes.Canceled, context.DeadlineExceeded.Error()).Error(): 50,
			},
		},
		{
			name:           "with_size_exceeded",
			maxLimitAdmit:  1000,
			maxLimitWait:   2000,
			requestSizes:   []uint64{1001},
			expectAcquired: [2]int64{0, 0},
			timeout:        0,
			expectErrs: map[string]int{
				ErrRequestTooLarge.Error(): 1,
			},
		},
		{
			name:           "mixed_sizes",
			maxLimitAdmit:  45, // 45 is the exact sum of request sizes
			maxLimitWait:   0,
			requestSizes:   mkRange(1, 9),
			expectAcquired: [2]int64{45, 45},
			timeout:        0,
			expectErrs:     map[string]int{},
		},
		{
			name:          "too_many_mixed_sizes",
			maxLimitAdmit: 44, // all but one request will succeed
			maxLimitWait:  0,
			requestSizes:  mkRange(1, 9),
			// worst case is the size=9 fails, so minimum is 44-9+1
			expectAcquired: [2]int64{36, 44},
			timeout:        0,
			expectErrs: map[string]int{
				ErrTooMuchWaiting.Error(): 1,
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			bq := newBQTest(t, test.maxLimitAdmit, test.maxLimitWait)
			ctx := context.Background()

			if test.timeout != 0 {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, test.timeout)
				defer cancel()
			}

			numRequests := len(test.requestSizes)
			allErrors := make(chan error, numRequests)

			releaseChan := make(chan struct{})
			var wait1 sync.WaitGroup
			var wait2 sync.WaitGroup

			wait1.Add(numRequests)
			wait2.Add(numRequests)

			for _, requestSize := range test.requestSizes {
				go func() {
					release, err := bq.Acquire(ctx, requestSize)
					allErrors <- err

					wait1.Done()

					<-releaseChan

					release()

					wait2.Done()
				}()
			}

			wait1.Wait()

			// The in-flight bytes are in-range, none waiting.
			inflight, waiting := bq.verifyMetrics(t)
			require.LessOrEqual(t, test.expectAcquired[0], inflight)
			require.GreaterOrEqual(t, test.expectAcquired[1], inflight)
			require.Equal(t, int64(0), waiting)

			close(releaseChan)

			wait2.Wait()

			close(allErrors)

			errCounts := map[string]int{}

			for err := range allErrors {
				if err == nil {
					continue
				}
				errCounts[err.Error()]++
			}

			require.Equal(t, test.expectErrs, errCounts)

			// Make sure we can allocate the whole limit at end-of-test.
			release, err := bq.Acquire(ctx, test.maxLimitAdmit)
			assert.NoError(t, err)
			release()

			// and the final state is all 0.
			bq.waitForPending(0, 0)

			// metrics are zero
			inflight, waiting = bq.verifyMetrics(t)
			require.Equal(t, int64(0), inflight)
			require.Equal(t, int64(0), waiting)
		})
	}
}

func (bq bqTest) verifyPoint(t *testing.T, m metricdata.Metrics) int64 {
	switch a := m.Data.(type) {
	case metricdata.Sum[int64]:
		require.Len(t, a.DataPoints, 1)
		dp := a.DataPoints[0]
		for _, attr := range dp.Attributes.ToSlice() {
			if attr.Key == netstats.ReceiverKey && attr.Value.AsString() == "admission_testing" {
				return dp.Value
			}
		}
		t.Errorf("point value not found: %v", m.Data)
	default:
		t.Errorf("incorrect metric data type: %T", m.Data)
	}
	return -1
}

func (bq bqTest) verifyMetrics(t *testing.T) (inflight int64, waiting int64) {
	inflight = -1
	waiting = -1

	var rm metricdata.ResourceMetrics
	require.NoError(t, bq.reader.Collect(context.Background(), &rm))

	for _, sm := range rm.ScopeMetrics {
		if sm.Scope.Name != expectScope {
			continue
		}
		for _, m := range sm.Metrics {
			switch m.Name {
			case expectInFlightName:
				inflight = bq.verifyPoint(t, m)
			case expectWaitingName:
				waiting = bq.verifyPoint(t, m)
			}
		}
	}
	return
}

func TestBoundedQueueLIFO(t *testing.T) {
	const maxAdmit = 10

	for _, firstAcquire := range mkRange(2, 8) {
		for _, firstWait := range mkRange(2, 8) {
			t.Run(fmt.Sprint(firstAcquire, ",", firstWait), func(t *testing.T) {
				t.Parallel()

				bq := newBQTest(t, maxAdmit, maxAdmit)
				ctx := context.Background()

				// Fill the queue
				relFirst, err := bq.Acquire(ctx, firstAcquire)
				require.NoError(t, err)
				bq.waitForPending(firstAcquire, 0)

				relSecond, err := bq.Acquire(ctx, maxAdmit-firstAcquire-1)
				require.NoError(t, err)
				bq.waitForPending(maxAdmit-1, 0)

				relOne, err := bq.Acquire(ctx, 1)
				require.NoError(t, err)
				bq.waitForPending(maxAdmit, 0)

				// Create two half-size waiters
				var relW0 ReleaseFunc
				notW0 := bq.startWaiter(ctx, firstWait, &relW0)
				bq.waitForPending(maxAdmit, firstWait)

				var relW1 ReleaseFunc
				secondWait := maxAdmit - firstWait
				notW1 := bq.startWaiter(ctx, secondWait, &relW1)
				bq.waitForPending(maxAdmit, maxAdmit)

				// The in-flight and waiting bytes are counted.
				inflight, waiting := bq.verifyMetrics(t)
				require.Equal(t, int64(maxAdmit), inflight)
				require.Equal(t, int64(maxAdmit), waiting)

				relFirst()

				// early is true when releasing the first acquired
				// will not make enough room for the first waiter
				early := firstAcquire < secondWait
				if early {
					relSecond()
				}

				// Expect notifications in LIFO order, i.e., W1 before W0.
				select {
				case <-notW0.Chan():
					t.Fatalf("FIFO order -- incorrect")
				case <-notW1.Chan():
					if !early {
						relSecond()
					}
				}
				relOne()

				<-notW0.Chan()

				relW0()
				relW1()

				bq.waitForPending(0, 0)

				inflight, waiting = bq.verifyMetrics(t)
				require.Equal(t, int64(0), inflight)
				require.Equal(t, int64(0), waiting)
			})
		}
	}
}

func TestBoundedQueueCancelation(t *testing.T) {
	// this test attempts to exercise the race condition between
	// the Acquire slow path and context cancelation.
	const (
		repetition = 100
		maxAdmit   = 10
	)
	bq := newBQTest(t, maxAdmit, maxAdmit)

	for number := range repetition {
		ctx, cancel := context.WithCancel(context.Background())

		tester := func() {
			// This acquire either succeeds or is canceled.
			testrel, err := bq.Acquire(ctx, maxAdmit)
			defer testrel()
			if err == nil {
				return
			}
			serr, ok := status.FromError(err)
			require.True(t, ok, "has gRPC status")
			require.Equal(t, codes.Canceled, serr.Code())
		}

		release, err := bq.Acquire(ctx, maxAdmit)
		require.NoError(t, err)

		go tester()

		if number%2 == 0 {
			go cancel()
			go release()
		} else {
			go release()
			go cancel()
		}

		bq.waitForPending(0, 0)
	}
}

func TestBoundedQueueNoop(t *testing.T) {
	nq := NewUnboundedQueue()
	for _, i := range mkRange(1, 100) {
		rel, err := nq.Acquire(context.Background(), i<<20)
		require.NoError(t, err)
		defer rel()
	}
}
