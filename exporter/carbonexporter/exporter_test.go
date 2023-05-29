// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package carbonexporter

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	conventions "go.opentelemetry.io/collector/semconv/v1.9.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
)

func TestNew(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	got, err := newCarbonExporter(cfg, exportertest.NewNopCreateSettings())
	assert.NotNil(t, got)
	assert.NoError(t, err)
}

func TestConsumeMetricsData(t *testing.T) {
	t.Skip("skipping flaky test, see https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/396")
	smallBatch := pmetric.NewMetrics()
	m := smallBatch.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
	m.SetName("test_gauge")
	dp := m.SetEmptyGauge().DataPoints().AppendEmpty()
	dp.Attributes().PutStr("k0", "v0")
	dp.Attributes().PutStr("k1", "v1")
	dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	dp.SetDoubleValue(123)
	largeBatch := generateLargeBatch()

	tests := []struct {
		name         string
		md           pmetric.Metrics
		acceptClient bool
		createServer bool
	}{
		{
			name: "small_batch",
			md:   smallBatch,
		},
		{
			name:         "small_batch",
			md:           smallBatch,
			createServer: true,
		},
		{
			name:         "small_batch",
			md:           smallBatch,
			createServer: true,
			acceptClient: true,
		},
		{
			name: "large_batch",
			md:   largeBatch,
		},
		{
			name:         "large_batch",
			md:           largeBatch,
			createServer: true,
		},
		{
			name:         "large_batch",
			md:           largeBatch,
			createServer: true,
			acceptClient: true,
		},
	}
	for _, tt := range tests {
		testName := fmt.Sprintf(
			"%s_createServer_%t_acceptClient_%t", tt.name, tt.createServer, tt.acceptClient)
		t.Run(testName, func(t *testing.T) {
			addr := testutil.GetAvailableLocalAddress(t)
			var ln *net.TCPListener
			if tt.createServer {
				laddr, err := net.ResolveTCPAddr("tcp", addr)
				require.NoError(t, err)
				ln, err = net.ListenTCP("tcp", laddr)
				require.NoError(t, err)
				defer ln.Close()
			}

			config := &Config{Endpoint: addr, Timeout: 1000 * time.Millisecond}
			exp, err := newCarbonExporter(config, exportertest.NewNopCreateSettings())
			require.NoError(t, err)

			require.NoError(t, exp.Start(context.Background(), componenttest.NewNopHost()))

			if !tt.createServer {
				require.Error(t, exp.ConsumeMetrics(context.Background(), tt.md))
				assert.NoError(t, exp.Shutdown(context.Background()))
				return
			}

			if !tt.acceptClient {
				// Due to differences between platforms is not certain if the call to ConsumeMetrics below will produce error or not.
				// See comment about recvfrom at connPool.Write for detailed information.
				_ = exp.ConsumeMetrics(context.Background(), tt.md)
				assert.NoError(t, exp.Shutdown(context.Background()))
				return
			}

			// Each metric point will generate one Carbon line, set up the wait
			// for all of them.
			var wg sync.WaitGroup
			wg.Add(tt.md.DataPointCount())
			go func() {
				assert.NoError(t, ln.SetDeadline(time.Now().Add(time.Second)))
				conn, err := ln.AcceptTCP()
				require.NoError(t, err)
				defer conn.Close()

				reader := bufio.NewReader(conn)
				for {
					// Actual metric validation is done by other tests, here it
					// is just flow.
					_, err := reader.ReadBytes(byte('\n'))
					if err != nil && !errors.Is(err, io.EOF) {
						assert.NoError(t, err) // Just to print any error
					}

					if errors.Is(err, io.EOF) {
						break
					}
					wg.Done()
				}
			}()

			<-time.After(100 * time.Millisecond)

			require.NoError(t, exp.ConsumeMetrics(context.Background(), tt.md))
			assert.NoError(t, exp.Shutdown(context.Background()))

			wg.Wait()
		})
	}
}

// Other tests didn't for the concurrency aspect of connPool, this test
// is designed to force that.
func Test_connPool_Concurrency(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping test on windows, see https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/10147")
	}
	addr := testutil.GetAvailableLocalAddress(t)
	laddr, err := net.ResolveTCPAddr("tcp", addr)
	require.NoError(t, err)
	ln, err := net.ListenTCP("tcp", laddr)
	require.NoError(t, err)
	defer ln.Close()

	startCh := make(chan struct{})

	cp := newTCPConnPool(addr, 500*time.Millisecond)
	sender := carbonSender{connPool: cp}
	ctx := context.Background()
	md := generateLargeBatch()
	concurrentWriters := 3
	writesPerRoutine := 3

	doneFlag := &atomic.Bool{}
	defer func() {
		doneFlag.Store(true)
	}()

	var recvWG sync.WaitGroup
	recvWG.Add(concurrentWriters * writesPerRoutine * md.MetricCount())
	go func() {
		for {
			conn, err := ln.AcceptTCP()
			if doneFlag.Load() {
				// Close is expected to cause error.
				return
			}
			require.NoError(t, err)
			go func(conn *net.TCPConn) {
				defer conn.Close()

				reader := bufio.NewReader(conn)
				for {
					// Actual metric validation is done by other tests, here it
					// is just flow.
					_, err := reader.ReadBytes(byte('\n'))
					if err != nil && !errors.Is(err, io.EOF) {
						assert.NoError(t, err) // Just to print any error
					}

					if errors.Is(err, io.EOF) {
						break
					}
					recvWG.Done()
				}
			}(conn)
		}
	}()

	var writersWG sync.WaitGroup
	for i := 0; i < concurrentWriters; i++ {
		writersWG.Add(1)
		go func() {
			<-startCh
			for i := 0; i < writesPerRoutine; i++ {
				assert.NoError(t, sender.pushMetricsData(ctx, md))
			}
			writersWG.Done()
		}()
	}

	close(startCh) // Release all workers
	writersWG.Wait()
	assert.NoError(t, sender.Shutdown(context.Background()))

	recvWG.Wait()
}

func generateLargeBatch() pmetric.Metrics {
	ts := time.Now()
	metrics := pmetric.NewMetrics()
	rm := metrics.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().PutStr(conventions.AttributeServiceName, "test_carbon")
	ms := rm.ScopeMetrics().AppendEmpty().Metrics()

	for i := 0; i < 65000; i++ {
		m := ms.AppendEmpty()
		m.SetName("test_" + strconv.Itoa(i))
		dp := m.SetEmptyGauge().DataPoints().AppendEmpty()
		dp.Attributes().PutStr("k0", "v0")
		dp.Attributes().PutStr("k1", "v1")
		dp.SetTimestamp(pcommon.NewTimestampFromTime(ts))
		dp.SetIntValue(int64(i))
	}

	return metrics
}
