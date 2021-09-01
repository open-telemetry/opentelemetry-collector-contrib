// Copyright 2019, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package carbonexporter

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	commonpb "github.com/census-instrumentation/opencensus-proto/gen-go/agent/common/v1"
	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	resourcepb "github.com/census-instrumentation/opencensus-proto/gen-go/resource/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/model/pdata"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/metricstestutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/testutil"
	internaldata "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/opencensus"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name:   "default_config",
			config: createDefaultConfig().(*Config),
		},
		{
			name: "invalid_tcp_addr",
			config: &Config{
				ExporterSettings: config.NewExporterSettings(config.NewID(typeStr)),
				Endpoint:         "http://localhost:2003",
			},
			wantErr: true,
		},
		{
			name: "invalid_timeout",
			config: &Config{
				ExporterSettings: config.NewExporterSettings(config.NewID(typeStr)),
				Timeout:          -5 * time.Second,
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := newCarbonExporter(tt.config, componenttest.NewNopExporterCreateSettings())
			if tt.wantErr {
				assert.Nil(t, got)
				assert.Error(t, err)
				return
			}

			assert.NotNil(t, got)
			assert.NoError(t, err)
		})
	}
}

func TestConsumeMetricsData(t *testing.T) {
	t.Skip("skipping flaky test, see https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/396")
	smallBatch := internaldata.OCToMetrics(nil, nil, []*metricspb.Metric{
		metricstestutil.Gauge(
			"test_gauge",
			[]string{"k0", "k1"},
			metricstestutil.Timeseries(
				time.Now(),
				[]string{"v0", "v1"},
				metricstestutil.Double(time.Now(), 123))),
	})

	largeBatch := generateLargeBatch()

	tests := []struct {
		name         string
		md           pdata.Metrics
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
			exp, err := newCarbonExporter(config, componenttest.NewNopExporterCreateSettings())
			require.NoError(t, err)

			require.NoError(t, exp.Start(context.Background(), componenttest.NewNopHost()))

			if !tt.createServer {
				require.Error(t, exp.ConsumeMetrics(context.Background(), tt.md))
				assert.NoError(t, exp.Shutdown(context.Background()))
				return
			}

			if !tt.acceptClient {
				// Due to differences between platforms is not certain if the
				// call to ConsumeMetricsData below will produce error or not.
				// See comment about recvfrom at connPool.Write for detailed
				// information.
				exp.ConsumeMetrics(context.Background(), tt.md)
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
					if err != nil && err != io.EOF {
						assert.NoError(t, err) // Just to print any error
					}

					if err == io.EOF {
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

	var doneFlag int64
	defer func(flag *int64) {
		atomic.StoreInt64(flag, 1)
	}(&doneFlag)

	var recvWG sync.WaitGroup
	recvWG.Add(concurrentWriters * writesPerRoutine * md.MetricCount())
	go func() {
		for {
			conn, err := ln.AcceptTCP()
			if atomic.LoadInt64(&doneFlag) != 0 {
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
					if err != nil && err != io.EOF {
						assert.NoError(t, err) // Just to print any error
					}

					if err == io.EOF {
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
	sender.Shutdown(context.Background())

	recvWG.Wait()
}

func generateLargeBatch() pdata.Metrics {
	var metrics []*metricspb.Metric
	ts := time.Now()
	for i := 0; i < 65000; i++ {
		metrics = append(metrics,
			metricstestutil.Gauge(
				"test_"+strconv.Itoa(i),
				[]string{"k0", "k1"},
				metricstestutil.Timeseries(
					time.Now(),
					[]string{"v0", "v1"},
					&metricspb.Point{
						Timestamp: timestamppb.New(ts),
						Value:     &metricspb.Point_Int64Value{Int64Value: int64(i)},
					},
				),
			),
		)
	}

	return internaldata.OCToMetrics(
		&commonpb.Node{
			ServiceInfo: &commonpb.ServiceInfo{Name: "test_carbon"},
		},
		&resourcepb.Resource{Type: "test"},
		metrics)
}
