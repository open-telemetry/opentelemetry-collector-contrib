// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsfirehosereceiver

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func BenchmarkLogsConsumer_cwlogs(b *testing.B) {
	// numLogGroups is the maximum number of unique log groups
	// to use across the generated logs, using a random generator
	// with fixed seeds for repeatability.
	const numLogGroups = 10
	rng := rand.New(rand.NewPCG(1, 2))

	// numRecords is the number of records in the Firehose envelope.
	for _, numRecords := range []int{10, 100} {
		// numLogs is the number of CoudWatch log records within a Firehose record.
		for _, numLogs := range []int{1, 10} {
			b.Run(fmt.Sprintf("%dresources_%drecords_%dlogs", numLogGroups, numRecords, numLogs), func(b *testing.B) {

				config := createDefaultConfig().(*Config)
				config.Endpoint = "localhost:0"
				r, err := createLogsReceiver(
					context.Background(),
					receivertest.NewNopSettings(),
					config,
					consumertest.NewNop(),
				)
				require.NoError(b, err)

				err = r.Start(context.Background(), componenttest.NewNopHost())
				require.NoError(b, err)
				b.Cleanup(func() {
					err = r.Shutdown(context.Background())
					assert.NoError(b, err)
				})

				records := make([]firehoseRecord, numRecords)
				for i := range records {
					records[i] = firehoseRecord{
						Data: base64.StdEncoding.EncodeToString(
							makeCloudWatchLogRecord(rng, numLogs, numLogGroups),
						),
					}
				}
				fr := testFirehoseRequest(testFirehoseRequestID, records)
				body, err := json.Marshal(fr)
				require.NoError(b, err)

				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					req := newTestRequest(body)
					recorder := httptest.NewRecorder()
					r.(http.Handler).ServeHTTP(recorder, req)
					if recorder.Code != http.StatusOK {
						b.Fatalf("expected status code 200, got %d", recorder.Code)
					}
				}
			})
		}
	}
}

func BenchmarkMetricsConsumer_cwmetrics(b *testing.B) {
	// numStreams is the maximum number of unique metric streams
	// to use across the generated metrics, using a random generator
	// with fixed seeds for repeatability.
	const numStreams = 10
	rng := rand.New(rand.NewPCG(1, 2))

	// numRecords is the number of records in the Firehose envelope.
	for _, numRecords := range []int{10, 100} {
		// numMetrics is the number of CoudWatch metrics within a Firehose record.
		for _, numMetrics := range []int{1, 10} {
			b.Run(fmt.Sprintf("%dresources_%drecords_%dmetrics", numStreams, numRecords, numMetrics), func(b *testing.B) {

				config := createDefaultConfig().(*Config)
				config.Endpoint = "localhost:0"
				r, err := createMetricsReceiver(
					context.Background(),
					receivertest.NewNopSettings(),
					config,
					consumertest.NewNop(),
				)
				require.NoError(b, err)

				err = r.Start(context.Background(), componenttest.NewNopHost())
				require.NoError(b, err)
				b.Cleanup(func() {
					err = r.Shutdown(context.Background())
					assert.NoError(b, err)
				})

				records := make([]firehoseRecord, numRecords)
				for i := range records {
					records[i] = firehoseRecord{
						Data: base64.StdEncoding.EncodeToString(
							makeCloudWatchMetricRecord(rng, numMetrics, numStreams),
						),
					}
				}

				fr := testFirehoseRequest(testFirehoseRequestID, records)
				body, err := json.Marshal(fr)
				require.NoError(b, err)

				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					req := newTestRequest(body)
					recorder := httptest.NewRecorder()
					r.(http.Handler).ServeHTTP(recorder, req)
					if recorder.Code != http.StatusOK {
						b.Fatalf("expected status code 200, got %d", recorder.Code)
					}
				}
			})
		}
	}
}

func makeCloudWatchLogRecord(rng *rand.Rand, numLogs, numLogGroups int) []byte {
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	for i := 0; i < numLogs; i++ {
		group := rng.IntN(numLogGroups)
		fmt.Fprintf(w,
			`{"messageType":"DATA_MESSAGE","owner":"123","logGroup":"group_%d","logStream":"stream","logEvents":[{"id":"the_id","timestamp":1725594035523,"message":"message %d"}]}`,
			group, i,
		)
		fmt.Fprintln(w)
	}
	if err := w.Close(); err != nil {
		panic(err)
	}
	return buf.Bytes()
}

func makeCloudWatchMetricRecord(rng *rand.Rand, numMetrics, numStreams int) []byte {
	var buf bytes.Buffer
	for i := 0; i < numMetrics; i++ {
		stream := rng.IntN(numStreams)
		fmt.Fprintf(&buf,
			`{"metric_stream_name":"stream_%d","account_id":"1234567890","region":"us-east-1","namespace":"AWS/NATGateway","metric_name":"metric_%d","dimensions":{"NatGatewayId":"nat-01a4160dfb995b990"},"timestamp":1643916720000,"value":{"max":0.0,"min":0.0,"sum":0.0,"count":2.0},"unit":"Count"}`,
			stream, i,
		)
		fmt.Fprintln(&buf)
	}
	return buf.Bytes()
}
