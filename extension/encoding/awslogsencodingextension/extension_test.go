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

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension/internal/constants"
	subscriptionfilter "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension/internal/unmarshaler/subscription-filter"
)

func TestNew_CloudWatchLogsSubscriptionFilter(t *testing.T) {
	e, err := newExtension(&Config{Format: constants.FormatCloudWatchLogsSubscriptionFilter}, extensiontest.NewNopSettings(extensiontest.NopType))
	require.NoError(t, err)
	require.NotNil(t, e)

	_, err = e.UnmarshalLogs([]byte("invalid"))
	require.ErrorContains(t, err, `failed to unmarshal logs as "cloudwatch_logs_subscription_filter" format`)
}

func TestNew_CloudTrailLog(t *testing.T) {
	e, err := newExtension(&Config{Format: constants.FormatCloudTrailLog}, extensiontest.NewNopSettings(extensiontest.NopType))
	require.NoError(t, err)
	require.NotNil(t, e)

	_, err = e.UnmarshalLogs([]byte("invalid"))
	require.ErrorContains(t, err, `failed to unmarshal logs as "cloudtrail_log" format`)
}

func TestNew_VPCFlowLog(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Format = constants.FormatVPCFlowLog
	e, err := newExtension(cfg, extensiontest.NewNopSettings(extensiontest.NopType))
	require.NoError(t, err)
	require.NotNil(t, e)

	// VPC Flow Log unmarshaler handles non-log input gracefully
	logs, err := e.UnmarshalLogs([]byte("some test input"))
	require.NoError(t, err)
	require.NotNil(t, logs)
}

func TestNew_S3AccessLog(t *testing.T) {
	e, err := newExtension(&Config{Format: constants.FormatS3AccessLog}, extensiontest.NewNopSettings(extensiontest.NopType))
	require.NoError(t, err)
	require.NotNil(t, e)

	_, err = e.UnmarshalLogs([]byte("invalid"))
	require.ErrorContains(t, err, `failed to unmarshal logs as "s3_access_log" format`)
}

func TestNew_WAFLog(t *testing.T) {
	e, err := newExtension(&Config{Format: constants.FormatWAFLog}, extensiontest.NewNopSettings(extensiontest.NopType))
	require.NoError(t, err)
	require.NotNil(t, e)

	_, err = e.UnmarshalLogs([]byte("invalid"))
	require.ErrorContains(t, err, `failed to unmarshal logs as "waf_log" format`)
}

func TestNew_ELBAcessLog(t *testing.T) {
	e, err := newExtension(&Config{Format: constants.FormatELBAccessLog}, extensiontest.NewNopSettings(extensiontest.NopType))
	require.NoError(t, err)
	require.NotNil(t, e)

	_, err = e.UnmarshalLogs([]byte("invalid"))
	require.ErrorContains(t, err, `failed to unmarshal logs as "elb_access_log" format`)
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
		format string
		buf    []byte
	}{
		"non_gzip_data_waf_log": {
			format: constants.FormatWAFLog,
			buf:    []byte("invalid"),
		},
		"valid_gzip_reader": {
			format: constants.FormatWAFLog,
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
			format: constants.FormatS3AccessLog,
			buf:    []byte("valid"),
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			e := &encodingExtension{format: test.format}
			_, reader, err := e.getReaderFromFormat(test.buf)
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
		format:      constants.FormatCloudWatchLogsSubscriptionFilter,
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
