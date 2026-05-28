// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package integrationtest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/aws/aws-lambda-go/events"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/johannesboyne/gofakes3"
	"github.com/johannesboyne/gofakes3/backend/s3mem"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/consumer/consumertest"
	collectorextension "go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/receiver/receivertest"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.uber.org/zap"

	awslogsencodingextension "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension"
	awslambdareceiver "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awslambdareceiver"
)

// testHost implements component.Host with a fixed set of extensions.
type testHost struct {
	extensions map[component.ID]component.Component
}

func (h *testHost) GetExtensions() map[component.ID]component.Component {
	return h.extensions
}

// TestParquetVPCFlow is an end-to-end integration test that exercises the
// awslambdareceiver parsing VPC flow logs in Parquet format.
//
// The test runs entirely in-process with no AWS credentials, no Docker, and no
// Lambda runtime emulator binary:
//
//   - gofakes3 (in-memory) serves as the S3 backend.
//   - An in-process HTTP server implements the Lambda Runtime API so that
//     lambda.Start (started by receiver.Start) can complete one invocation cycle.
//   - An in-memory tracetest.SpanRecorder captures all otelaws spans, letting us
//     assert that the chunk-cached range-GET path in s3ObjectReader was exercised.
func TestParquetVPCFlow(t *testing.T) {
	ctx := t.Context()

	// 1. Generate a large Parquet file in memory (100 000 rows → typically > 4 MB with Snappy).
	parquetData := generateVPCFlowParquet(100_000)
	fileSize := int64(len(parquetData))
	t.Logf("generated Parquet file: %d bytes (%.1f MB)", fileSize, float64(fileSize)/1e6)
	require.Greater(t, fileSize, int64(4*1024*1024),
		"Parquet file must exceed the 4 MB chunk size to trigger multi-chunk range GETs; "+
			"increase row count or disable compression in generateVPCFlowParquet")

	// 2. Start gofakes3 in default (path-style) mode.
	// httptest.NewServer binds to 127.0.0.1, producing an IP-literal URL
	// (http://127.0.0.1:PORT). The AWS SDK automatically uses path-style
	// addressing when the endpoint is an IP address, so both the test upload
	// client and the receiver's internal client send requests to
	// http://127.0.0.1:PORT/{bucket}/{key} without any DNS lookup.
	// This is cross-platform — no *.localhost wildcard DNS needed.
	fakeS3 := gofakes3.New(s3mem.New())
	s3Server := httptest.NewServer(fakeS3.Server())
	t.Cleanup(s3Server.Close)

	s3EndpointURL := s3Server.URL // http://127.0.0.1:PORT — IP literal forces S3 path-style

	// 3. Set env vars before creating any AWS SDK clients so that all clients
	// (test upload client + receiver's internal client) read the same config.
	const (
		bucketName = "test-bucket"
		s3Key      = "AWSLogs/123456789012/vpcflowlogs/us-east-1/2024/01/01/sample.parquet"
	)
	t.Setenv("AWS_ENDPOINT_URL_S3", s3EndpointURL)
	t.Setenv("AWS_REGION", "us-east-1")
	t.Setenv("AWS_ACCESS_KEY_ID", "test-access-key")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test-secret-key")
	// gofakes3 does not compute per-range checksums; tell the SDK to only
	// validate checksums when the caller explicitly requests them.
	t.Setenv("AWS_RESPONSE_CHECKSUM_VALIDATION", "when_required")
	t.Setenv("AWS_REQUEST_CHECKSUM_CALCULATION", "when_required")

	// 4. Upload the Parquet file to gofakes3. Both this client and the receiver's
	// internal client use path-style addressing because the endpoint is an IP.
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx)
	require.NoError(t, err)
	s3Client := s3.NewFromConfig(awsCfg)

	bucket := bucketName
	key := s3Key
	_, err = s3Client.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: &bucket})
	require.NoError(t, err)
	_, err = s3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        &bucket,
		Key:           &key,
		Body:          bytes.NewReader(parquetData),
		ContentLength: &fileSize,
	})
	require.NoError(t, err)

	// 5. Start the in-process Lambda Runtime API server and point the lambda
	// library at it.  The server must NOT be closed before the test binary exits
	// (see runtimeapi.go comment); the t.Setenv Cleanup restores the env var.
	t.Setenv("AWS_EXECUTION_ENV", "AWS_Lambda_go1.x")
	rapi := startRuntimeAPI(t)
	t.Setenv("AWS_LAMBDA_RUNTIME_API", rapi.Addr)

	// 6. In-memory tracer provider that records every finished span.
	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))
	t.Cleanup(func() { _ = tp.Shutdown(ctx) })

	// 7. Build and start the awslogs_encoding extension configured for VPC flow
	// logs in Parquet format.  The VPCFlowLogConfig type lives in an internal
	// package, so we configure via confmap rather than direct struct access.
	extFactory := awslogsencodingextension.NewFactory()
	extID := component.NewID(extFactory.Type())
	extCfg := extFactory.CreateDefaultConfig()
	conf := confmap.NewFromStringMap(map[string]any{
		"format":  "vpcflow",
		"vpcflow": map[string]any{"file_format": "parquet"},
	})
	require.NoError(t, conf.Unmarshal(extCfg))
	extSet := collectorextension.Settings{
		ID: extID,
		TelemetrySettings: component.TelemetrySettings{
			Logger: zap.NewNop(),
		},
		BuildInfo: component.NewDefaultBuildInfo(),
	}
	ext, err := extFactory.Create(ctx, extSet, extCfg)
	require.NoError(t, err)
	require.NoError(t, ext.Start(ctx, &testHost{}))
	t.Cleanup(func() { _ = ext.Shutdown(ctx) })

	// 8. Build and start the receiver via its public factory interface.
	recvFactory := awslambdareceiver.NewFactory()
	recvCfg := recvFactory.CreateDefaultConfig().(*awslambdareceiver.Config)
	recvCfg.S3.Encoding = extID.String()
	set := receivertest.NewNopSettings(recvFactory.Type())
	set.TracerProvider = tp
	sink := new(consumertest.LogsSink)
	r, err := recvFactory.CreateLogs(ctx, set, recvCfg, sink)
	require.NoError(t, err)
	fakeHost := &testHost{extensions: map[component.ID]component.Component{extID: ext}}
	require.NoError(t, r.Start(ctx, fakeHost))
	t.Cleanup(func() { _ = r.Shutdown(ctx) })

	// 9. Push an S3 event referencing the uploaded Parquet file.
	// URLDecodedKey is populated automatically by aws-lambda-go's UnmarshalJSON
	// when the event is received by the handler; we only need to set Key here.
	evt := events.S3Event{
		Records: []events.S3EventRecord{{
			S3: events.S3Entity{
				Bucket: events.S3Bucket{
					Name: bucketName,
					Arn:  fmt.Sprintf("arn:aws:s3:::%s", bucketName),
				},
				Object: events.S3Object{
					Key:  s3Key,
					Size: fileSize,
				},
			},
		}},
	}
	evtJSON, err := json.Marshal(evt)
	require.NoError(t, err)
	rapi.Events <- evtJSON

	// 10. Wait for the lambda handler to report success.
	select {
	case result := <-rapi.Done:
		require.Nil(t, result.Err,
			"lambda handler reported an error: %s", string(result.Err))
	case <-time.After(60 * time.Second):
		t.Fatal("timed out waiting for lambda invocation result")
	}

	// 11. Assert that VPC flow log records were decoded.
	require.Positive(t, sink.LogRecordCount(),
		"expected VPC flow log records to be decoded from the Parquet file")
	t.Logf("decoded %d log records", sink.LogRecordCount())

	// 12. Assert ≥ 2 S3.GetObject spans: Parquet opens the file by reading its
	// footer (a small range GET near the end), then reads row groups (one or more
	// additional range GETs). A file > 4 MB spans at least two 4 MB chunks,
	// guaranteeing ≥ 2 distinct GetObject calls.
	spans := sr.Ended()
	var getObjectSpans int
	for _, span := range spans {
		if span.Name() == "S3.GetObject" {
			getObjectSpans++
		}
	}
	t.Logf("observed %d S3.GetObject spans", getObjectSpans)
	require.GreaterOrEqual(t, getObjectSpans, 2,
		"expected ≥ 2 S3.GetObject spans (Parquet footer range GET + at least one row-group range GET)")
}
