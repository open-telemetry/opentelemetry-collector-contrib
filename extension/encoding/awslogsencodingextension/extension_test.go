// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awslogsencodingextension

import (
	"bytes"
	"os"
	"sync"
	"testing"

	"github.com/klauspost/compress/gzip"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension/extensiontest"

	subscriptionfilter "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension/internal/unmarshaler/subscription-filter"
)

func TestNew_CloudWatchLogsSubscriptionFilter(t *testing.T) {
	e, err := newExtension(&Config{Format: formatCloudWatchLogsSubscriptionFilter}, extensiontest.NewNopSettings(extensiontest.NopType))
	require.NoError(t, err)
	require.NotNil(t, e)

	_, err = e.UnmarshalLogs([]byte("invalid"))
	require.ErrorContains(t, err, `failed to get reader for "cloudwatch_logs_subscription_filter" logs`)
}

func TestNew_VPCFlowLog(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Format = formatVPCFlowLog
	e, err := newExtension(cfg, extensiontest.NewNopSettings(extensiontest.NopType))
	require.NoError(t, err)
	require.NotNil(t, e)

	_, err = e.UnmarshalLogs([]byte("invalid"))
	require.ErrorContains(t, err, `failed to get reader for "vpc_flow_log" logs`)
}

func TestNew_S3AccessLog(t *testing.T) {
	e, err := newExtension(&Config{Format: formatS3AccessLog}, extensiontest.NewNopSettings(extensiontest.NopType))
	require.NoError(t, err)
	require.NotNil(t, e)

	_, err = e.UnmarshalLogs([]byte("invalid"))
	require.ErrorContains(t, err, `failed to unmarshal logs as "s3_access_log" format`)
}

func TestNew_WAFLog(t *testing.T) {
	e, err := newExtension(&Config{Format: formatWAFLog}, extensiontest.NewNopSettings(extensiontest.NopType))
	require.NoError(t, err)
	require.NotNil(t, e)

	_, err = e.UnmarshalLogs([]byte("invalid"))
	require.ErrorContains(t, err, `failed to get reader for "waf_log" logs`)
}

func TestNew_Unimplemented(t *testing.T) {
	e, err := newExtension(&Config{Format: "invalid"}, extensiontest.NewNopSettings(extensiontest.NopType))
	require.Error(t, err)
	require.Nil(t, e)
	assert.EqualError(t, err, `unimplemented format "invalid"`)
}

func TestGetReaderFromFormat(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		format      string
		buf         []byte
		expectedErr string
	}{
		"invalid_gzip_reader": {
			format:      formatWAFLog,
			buf:         []byte("invalid"),
			expectedErr: "failed to decompress content",
		},
		"valid_gzip_reader": {
			format: formatWAFLog,
			buf: func() []byte {
				var buf bytes.Buffer
				gz := gzip.NewWriter(&buf)
				_, err := gz.Write([]byte("valid"))
				require.NoError(t, err)
				_ = gz.Close()
				return buf.Bytes()
			}(),
		},
		"valid_bytes_reader": {
			format: formatS3AccessLog,
			buf:    []byte("valid"),
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			e := &encodingExtension{format: test.format}
			_, reader, err := e.getReaderFromFormat(test.buf)
			if test.expectedErr != "" {
				require.ErrorContains(t, err, test.expectedErr)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, reader)
		})
	}
}

// readAndCompressLogFile reads the data inside it, compresses it
// and returns a GZIP reader for it.
func readAndCompressLogFile(t *testing.T, file string) []byte {
	data, err := os.ReadFile(file)
	require.NoError(t, err)
	var compressedData bytes.Buffer
	gzipWriter := gzip.NewWriter(&compressedData)
	_, err = gzipWriter.Write(data)
	require.NoError(t, err)
	err = gzipWriter.Close()
	require.NoError(t, err)
	return compressedData.Bytes()
}

func TestConcurrentGzipReaderUsage(t *testing.T) {
	// Create an encoding extension for cloudwatch format to test the
	// gzip reader and check that it works as expected for non concurrent
	// and concurrent usage
	ext := &encodingExtension{
		unmarshaler: subscriptionfilter.NewSubscriptionFilterUnmarshaler(component.BuildInfo{}),
		format:      formatCloudWatchLogsSubscriptionFilter,
		gzipPool:    sync.Pool{},
	}

	cloudwatchData := readAndCompressLogFile(t, "testdata/cloudwatch_log.json")
	testUnmarshall := func() {
		_, err := ext.UnmarshalLogs(cloudwatchData)
		require.NoError(t, err)
	}

	// non concurrent
	testUnmarshall()

	// concurrent usage
	concurrent := 20
	wg := sync.WaitGroup{}
	for i := 0; i < concurrent; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			testUnmarshall()
		}()
	}
	wg.Wait()
}
