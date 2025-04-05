// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsxray

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/xray"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"
)

func TestUserAgent(t *testing.T) {
	// Set up test HTTP server
	var capturedUserAgent string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedUserAgent = r.Header.Get("User-Agent")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	logger := zap.NewNop()
	buildInfo := component.BuildInfo{
		Command: "test-collector-contrib",
		Version: "1.0",
	}

	ctx := context.Background()

	// Create a custom endpoint resolver that points to our test server
	// Using the non-deprecated API
	customResolver := aws.EndpointResolverFunc(func(service, region string) (aws.Endpoint, error) {
		return aws.Endpoint{
			URL:           srv.URL,
			SigningRegion: "us-west-2",
		}, nil
	})

	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion("us-west-2"),
		config.WithEndpointResolver(customResolver),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("AKID", "SECRET", "SESSION")),
	)
	require.NoError(t, err)

	client := NewXRayClient(ctx, logger, cfg, buildInfo).(*xrayClient)
	require.NotNil(t, client)

	// Send an empty request to capture the User-Agent header
	_, err = client.PutTraceSegments(ctx, &xray.PutTraceSegmentsInput{
		TraceSegmentDocuments: []string{"{}"}, // Simple empty segment
	})
	require.NoError(t, err)

	// Verify that the captured User-Agent contains the expected information
	assert.Contains(t, capturedUserAgent, "test-collector-contrib/1.0")
	assert.Contains(t, capturedUserAgent, "xray-otel-exporter/")
	assert.Contains(t, capturedUserAgent, "exec-env/")
	assert.Contains(t, capturedUserAgent, "OS/")
}