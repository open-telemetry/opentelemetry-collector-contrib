// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package integrationtest

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/elastic/go-elasticsearch/v8/esutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/plog/plogotlp"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func TestExporter(t *testing.T) {
	for _, tc := range []struct {
		name string
		// restartCollector restarts the OTEL collector. Restarting
		// the collector allows durability testing of the ES exporter
		// based on the OTEL config used for testing.
		restartCollector bool
		mockESFailure    bool
	}{
		{name: "basic"},
		{name: "es_intermittent_failure", mockESFailure: true},
		/* Below tests should be enabled after #30792 is fixed
		{name: "collector_restarts", restartCollector: true},
		{name: "collector_restart_with_es_intermittent_failure", mockESFailure: true, restartCollector: true},
		*/
	} {
		t.Run(tc.name, func(t *testing.T) {
			runner(t, tc.restartCollector, tc.mockESFailure)
		})
	}
}

func runner(t *testing.T, restartCollector, mockESFailure bool) {
	t.Helper()

	cfg := loadConfig(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	flushIvl := time.Second
	mockES := newMockESClient(t, cfg.Debug)
	collector := newTestCollector(t, cfg, mockES.ServerURL, flushIvl)

	var g errgroup.Group
	cancelRun := runTestCollectorWithWait(ctx, t, collector, &g)

	if mockESFailure {
		mockES.SetReturnStatusCode(http.StatusServiceUnavailable)
		if !restartCollector {
			// If restartCollector is not set then recover ES service
			// after a few failed bulk requests otherwise ensure that
			// both collector and ES are unavailable for the same
			// duration considering worst case scenario. ES recovers
			// after a certain number of bulk requests have happened.
			var bulkCalls atomic.Int32
			mockES.SetOnBulkRequest(func(_ esutil.BulkIndexerResponse) {
				if count := bulkCalls.Add(1); count == 10 {
					mockES.SetReturnStatusCode(http.StatusOK)
				}
			})
		}
	}

	count := sendLogs(t, cfg.GRPCEndpoint, "batch_1", 500, 10) // total=logs*agents

	if restartCollector {
		// Restart the collector after all data is sent to the collector.
		// Note that the log count should be high to ensure that the collector
		// stops before all data is shipped to Elasticsearch.
		require.NoError(t, collector.Recreate())
		cancelRun()
		// Ensure that the previous collector instance is shut down and then
		// run the recreated collector.
		require.NoError(t, g.Wait())
		runTestCollectorWithWait(ctx, t, collector, &g)
		// Recover ES if it was unavailable.
		mockES.SetReturnStatusCode(http.StatusOK)
	}

	count += sendLogs(t, cfg.GRPCEndpoint, "batch_2", 500, 10) // total=logs*agents

	assert.Eventually(
		t, func() bool {
			resp, err := mockES.Count(mockES.Count.WithIndex(cfg.ESLogsIndex))
			require.NoError(t, err)
			defer resp.Body.Close()

			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)

			result := gjson.GetBytes(body, "count")
			return result.Int() == int64(count)
		}, time.Minute, time.Second,
	)
}

// runTestCollectorWithWait runs the test OTEL collector in a goroutine.
// It returns a cancel func that allows the caller to stop the collector
// without waiting for graceful shutdown.
func runTestCollectorWithWait(
	ctx context.Context, t *testing.T, collector *otelCol, g *errgroup.Group,
) context.CancelFunc {
	t.Helper()

	rCtx, rCancel := context.WithCancel(ctx)
	g.Go(func() error { return collector.Run(rCtx) })
	// Wait for otelcollector to be in running state
	require.Eventually(t, func() bool {
		return collector.IsRunning()
	}, time.Second, 10*time.Millisecond)
	return rCancel
}

// sendLogs sends the specified number of logs for each agent. For example
// with 10 logs and 10 agents we will have 100 total logs sent to the
// target server. The total number of logs sent is returned.
func sendLogs(t *testing.T, target, uid string, logs, agents int) int {
	t.Helper()

	g, ctx := errgroup.WithContext(context.Background())
	g.SetLimit(10)

	for i := 0; i < agents; i++ {
		id := fmt.Sprintf("%s-%d", uid, i)
		g.Go(func() error {
			conn, err := grpc.DialContext(ctx, target, grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				return err
			}
			defer conn.Close()
			client := plogotlp.NewGRPCClient(conn)

			for j := 0; j < logs; j++ {
				logs := plog.NewLogs()
				res := logs.ResourceLogs().AppendEmpty().Resource()
				res.Attributes().PutStr("source", "otel-esexporter-test")
				log := logs.ResourceLogs().At(0).ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
				log.Body().SetStr(fmt.Sprintf("test log %d with uid %s", j, id))
				log.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
				log.SetDroppedAttributesCount(1)
				log.SetSeverityNumber(plog.SeverityNumberInfo)

				_, err = client.Export(ctx, plogotlp.NewExportRequestFromLogs(logs))
				if err != nil {
					return err
				}
			}
			return nil
		})
	}

	require.NoError(t, g.Wait())
	return logs * agents
}
