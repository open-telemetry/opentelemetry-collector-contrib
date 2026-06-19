// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package integrationtest

import (
	"context"
	"fmt"
	"runtime"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pdata/ptrace/ptraceotlp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func BenchmarkCollectorOTLPReceiverStorageBackends(b *testing.B) {
	enableTailStorageFeatureGateForBenchmark(b)

	rateLimitDims := []struct {
		name        string
		targetReqPS int
	}{
		{name: "rps_200", targetReqPS: 200},
		{name: "rps_inf", targetReqPS: 0},
	}
	decisionWaitDims := []struct {
		name                  string
		decisionWait          time.Duration
		require2xDecisionWait bool
	}{
		{name: "dw_1s", decisionWait: time.Second, require2xDecisionWait: true},
		{name: "dw_1d", decisionWait: 24 * time.Hour, require2xDecisionWait: false},
	}

	for _, samplingRatePct := range []float64{0, 1.0, 100} {
		b.Run(fmt.Sprintf("sampling_%.0f%%", samplingRatePct), func(b *testing.B) {
			for _, dwDim := range decisionWaitDims {
				b.Run(dwDim.name, func(b *testing.B) {
					for _, rlDim := range rateLimitDims {
						shape := benchmarkShape{
							tracesPerBatch:  30,
							spansPerTrace:   7,
							payloadBytes:    4 * 1024,
							parallelism:     8,
							targetReqPerSec: rlDim.targetReqPS,
							decisionWait:    dwDim.decisionWait,
							numTraces:       1_000_000,
							policy: benchmarkPolicy{
								name:                 "probabilistic",
								kind:                 benchmarkPolicyProbabilistic,
								samplingPercentage:   samplingRatePct,
								backendNameInmemory:  "probabilistic-1pct",
								backendNameWithStore: "probabilistic",
							},
							require2xDecisionWait: dwDim.require2xDecisionWait,
						}
						b.Run(rlDim.name, func(b *testing.B) {
							for _, backend := range []string{"inmemory", "pebble"} {
								b.Run(backend, func(b *testing.B) {
									runCollectorBenchmarkCase(b, shape, backend)
								})
							}
						})
					}
				})
			}
		})
	}
}

func BenchmarkCollectorOTLPReceiverStorageBackendsDistributedTraces(b *testing.B) {
	enableTailStorageFeatureGateForBenchmark(b)

	for _, rootSamplingPct := range []float64{100, 1} {
		b.Run(fmt.Sprintf("root_%.0f%%", rootSamplingPct), func(b *testing.B) {
			for _, backend := range []string{"inmemory", "pebble"} {
				b.Run(backend, func(b *testing.B) {
					shape := benchmarkShape{
						tracesPerBatch:       30,
						spansPerTrace:        7,
						payloadBytes:         4 * 1024,
						parallelism:          8,
						targetReqPerSec:      0,
						decisionWait:         24 * time.Hour,
						numTraces:            1_000_000,
						rootSpanDelay:        time.Second,
						distributedTraceMode: true,
						policy: benchmarkPolicy{
							name:               "sample_roots",
							kind:               benchmarkPolicyRootProbabilistic,
							samplingPercentage: rootSamplingPct,
						},
						require2xDecisionWait: false,
					}
					runCollectorBenchmarkCase(b, shape, backend)
				})
			}
		})
	}
}

func runCollectorBenchmarkCase(b *testing.B, shape benchmarkShape, backend string) {
	shape.validate(b)

	if shape.require2xDecisionWait {
		requireBenchTimeAtLeast(b, 2*shape.decisionWait)
	}

	target, cleanup, storageDir := setupCollectorBenchmark(b, backend, shape)
	defer cleanup()

	conn, err := grpc.NewClient(target, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(b, err)
	defer func() { _ = conn.Close() }()
	client := ptraceotlp.NewGRPCClient(conn)

	referenceBatch := benchmarkBatch(1, shape)
	b.SetBytes(int64((&ptrace.ProtoMarshaler{}).TracesSize(referenceBatch)))
	b.ReportAllocs()
	b.SetParallelism(shape.parallelism)
	runtime.GC()
	debug.FreeOSMemory()
	baselineRSS, err := currentRSSBytes()
	require.NoError(b, err)
	stopRSSSampler := startRSSSampler(10 * time.Millisecond)
	acquire, stopRateLimiter := startFixedRateLimiter(shape.targetReqPerSec)
	defer stopRateLimiter()
	var requests atomic.Uint64
	var delayedRoots *delayedRootExporter
	if shape.distributedTraceMode {
		delayedRoots = newDelayedRootExporter(client, shape.rootSpanDelay, shape.parallelism*16)
	}
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		var seed uint64
		for pb.Next() {
			acquire()
			seed++
			if shape.distributedTraceMode {
				children, roots := benchmarkDistributedTraceBatches(seed, shape)
				_, exportErr := client.Export(b.Context(), ptraceotlp.NewExportRequestFromTraces(children))
				require.NoError(b, exportErr)
				requests.Add(2)
				delayedRoots.enqueue(b.Context(), roots)
				continue
			}

			requests.Add(1)
			_, exportErr := client.Export(b.Context(), ptraceotlp.NewExportRequestFromTraces(benchmarkBatch(seed, shape)))
			require.NoError(b, exportErr)
		}
	})
	b.StopTimer()
	if delayedRoots != nil {
		require.NoError(b, delayedRoots.wait())
	}

	elapsed := b.Elapsed()
	if elapsed > 0 {
		reqPerSec := float64(requests.Load()) / elapsed.Seconds()
		spansPerSec := reqPerSec * float64(shape.spansPerRequest()) / float64(shape.requestsPerIteration())
		b.ReportMetric(reqPerSec, "recv_request/s")
		b.ReportMetric(spansPerSec, "recv_spans/s")
	}

	peakRSS := stopRSSSampler()
	deltaRSS := max(int64(peakRSS)-int64(baselineRSS), int64(0))
	b.ReportMetric(float64(peakRSS)/(1024*1024), "peak_rss_mb")
	b.ReportMetric(float64(deltaRSS)/(1024*1024), "rss_delta_mb")
	storageBytes, err := dirSizeBytes(storageDir)
	require.NoError(b, err)
	b.ReportMetric(float64(storageBytes)/(1024*1024), "storage_mb")
}

type delayedRootExporter struct {
	client ptraceotlp.GRPCClient
	delay  time.Duration
	sem    chan struct{}

	wg    sync.WaitGroup
	errMu sync.Mutex
	err   error
}

func newDelayedRootExporter(client ptraceotlp.GRPCClient, delay time.Duration, maxInFlight int) *delayedRootExporter {
	if maxInFlight <= 0 {
		maxInFlight = 64
	}
	return &delayedRootExporter{
		client: client,
		delay:  delay,
		sem:    make(chan struct{}, maxInFlight),
	}
}

func (e *delayedRootExporter) enqueue(ctx context.Context, roots ptrace.Traces) {
	e.sem <- struct{}{}
	e.wg.Go(func() {
		defer func() { <-e.sem }()

		time.Sleep(e.delay)
		_, err := e.client.Export(ctx, ptraceotlp.NewExportRequestFromTraces(roots))
		if err != nil {
			e.setErr(err)
			return
		}
	})
}

func (e *delayedRootExporter) wait() error {
	e.wg.Wait()
	e.errMu.Lock()
	defer e.errMu.Unlock()
	return e.err
}

func (e *delayedRootExporter) setErr(err error) {
	e.errMu.Lock()
	defer e.errMu.Unlock()
	if e.err == nil {
		e.err = err
	}
}
