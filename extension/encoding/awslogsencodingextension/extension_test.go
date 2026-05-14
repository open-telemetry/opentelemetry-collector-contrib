// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awslogsencodingextension

import (
	"bytes"
	"io"
	"os"
	"sync"
	"testing"

	"github.com/klauspost/compress/gzip"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/extension/extensiontest"
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension/internal/constants"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension/internal/metadata"
	subscriptionfilter "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension/internal/unmarshaler/subscription-filter"
)

func TestNew_CloudWatchLogsSubscriptionFilter(t *testing.T) {
	e, err := newExtension(&Config{Format: constants.FormatCloudWatchLogsSubscriptionFilter}, extensiontest.NewNopSettings(extensiontest.NopType))
	require.NoError(t, err)
	require.NotNil(t, e)

	_, err = e.UnmarshalLogs([]byte("invalid"))
	require.ErrorContains(t, err, `failed to unmarshal logs as "cloudwatch" format`)
}

func TestNew_CloudTrailLog(t *testing.T) {
	e, err := newExtension(&Config{Format: constants.FormatCloudTrailLog}, extensiontest.NewNopSettings(extensiontest.NopType))
	require.NoError(t, err)
	require.NotNil(t, e)

	_, err = e.UnmarshalLogs([]byte("invalid"))
	require.ErrorContains(t, err, `failed to unmarshal logs as "cloudtrail" format`)
}

func TestNew_VPCFlowLog(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Format = constants.FormatVPCFlowLog
	e, err := newExtension(cfg, extensiontest.NewNopSettings(extensiontest.NopType))
	require.NoError(t, err)
	require.NotNil(t, e)

	_, err = e.UnmarshalLogs([]byte("some test input"))
	require.ErrorContains(t, err, "failed to read first line of VPC logs from S3")
}

func TestNew_S3AccessLog(t *testing.T) {
	e, err := newExtension(&Config{Format: constants.FormatS3AccessLog}, extensiontest.NewNopSettings(extensiontest.NopType))
	require.NoError(t, err)
	require.NotNil(t, e)

	_, err = e.UnmarshalLogs([]byte("invalid"))
	require.ErrorContains(t, err, `failed to unmarshal logs as "s3access" format`)
}

func TestNew_WAFLog(t *testing.T) {
	e, err := newExtension(&Config{Format: constants.FormatWAFLog}, extensiontest.NewNopSettings(extensiontest.NopType))
	require.NoError(t, err)
	require.NotNil(t, e)

	_, err = e.UnmarshalLogs([]byte("invalid"))
	require.ErrorContains(t, err, `failed to unmarshal logs as "waf" format`)
}

func TestNew_ELBAcessLog(t *testing.T) {
	e, err := newExtension(&Config{Format: constants.FormatELBAccessLog}, extensiontest.NewNopSettings(extensiontest.NopType))
	require.NoError(t, err)
	require.NotNil(t, e)

	_, err = e.UnmarshalLogs([]byte("invalid"))
	require.ErrorContains(t, err, `failed to unmarshal logs as "elbaccess" format`)
}

func TestNew_Unimplemented(t *testing.T) {
	e, err := newExtension(&Config{Format: "invalid"}, extensiontest.NewNopSettings(extensiontest.NopType))
	require.Error(t, err)
	require.Nil(t, e)
	assert.EqualError(t, err, `unimplemented format "invalid"`)
}

func TestValidateFeatureGates(t *testing.T) {
	registry := featuregate.GlobalRegistry()
	require.NoError(t, registry.Set(metadata.ExtensionEncodingAwslogsencodingDontEmitV0RPCConventionsFeatureGate.ID(), true))
	require.NoError(t, registry.Set(metadata.ExtensionEncodingAwslogsencodingEmitV1RPCConventionsFeatureGate.ID(), false))
	t.Cleanup(func() {
		require.NoError(t, registry.Set(metadata.ExtensionEncodingAwslogsencodingDontEmitV0RPCConventionsFeatureGate.ID(), false))
		require.NoError(t, registry.Set(metadata.ExtensionEncodingAwslogsencodingEmitV1RPCConventionsFeatureGate.ID(), false))
	})

	e, err := newExtension(&Config{Format: constants.FormatCloudTrailLog}, extensiontest.NewNopSettings(extensiontest.NopType))
	require.Nil(t, e)
	require.ErrorContains(t, err, "extension.encoding.awslogsencoding.DontEmitV0RPCConventions requires extension.encoding.awslogsencoding.EmitV1RPCConventions to be enabled")
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
			e := &encodingExtension{
				format: test.format,
				cfg:    createDefaultConfig().(*Config),
			}
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
	for range concurrent {
		wg.Go(func() {
			testUnmarshall()
		})
	}
	wg.Wait()
}

// fakeInnerExtension is a minimal LogsUnmarshalerExtension used to stand in
// for inner targets that the router dispatches to in the Start tests.
type fakeInnerExtension struct {
	component.StartFunc
	component.ShutdownFunc
}

func (*fakeInnerExtension) UnmarshalLogs(_ []byte) (plog.Logs, error) {
	return plog.NewLogs(), nil
}

func (*fakeInnerExtension) NewLogsDecoder(_ io.Reader, _ ...encoding.DecoderOption) (encoding.LogsDecoder, error) {
	return nil, nil
}

// hostWithExtensions wraps a NopHost so component.Host.GetExtensions returns
// a custom map populated by tests.
type hostWithExtensions struct {
	component.Host
	extensions map[component.ID]component.Component
}

func (h *hostWithExtensions) GetExtensions() map[component.ID]component.Component {
	return h.extensions
}

func TestStart_NoRoutes(t *testing.T) {
	t.Parallel()

	e, err := newExtension(
		&Config{Format: constants.FormatCloudWatchLogsSubscriptionFilter},
		extensiontest.NewNopSettings(extensiontest.NopType),
	)
	require.NoError(t, err)

	require.NoError(t, e.Start(t.Context(), componenttest.NewNopHost()))
}

func TestStart_RoutesWithValidHost(t *testing.T) {
	t.Parallel()

	innerID := component.MustNewIDWithName("aws_logs_encoding", "inner")
	settings := extensiontest.NewNopSettings(extensiontest.NopType)

	e, err := newExtension(
		&Config{
			Format: constants.FormatCloudWatchLogsSubscriptionFilter,
			CloudWatch: CloudWatchConfig{
				Streams: []subscriptionfilter.CloudWatchStream{
					{Name: "vpcflow", Encoding: innerID},
				},
			},
		},
		settings,
	)
	require.NoError(t, err)

	host := &hostWithExtensions{
		Host: componenttest.NewNopHost(),
		extensions: map[component.ID]component.Component{
			innerID: &fakeInnerExtension{},
		},
	}
	require.NoError(t, e.Start(t.Context(), host))
}

func TestStart_RoutesWithMissingInner(t *testing.T) {
	t.Parallel()

	innerID := component.MustNewIDWithName("aws_logs_encoding", "missing")

	e, err := newExtension(
		&Config{
			Format: constants.FormatCloudWatchLogsSubscriptionFilter,
			CloudWatch: CloudWatchConfig{
				Streams: []subscriptionfilter.CloudWatchStream{
					{Name: "vpcflow", Encoding: innerID},
				},
			},
		},
		extensiontest.NewNopSettings(extensiontest.NopType),
	)
	require.NoError(t, err)

	err = e.Start(t.Context(), componenttest.NewNopHost())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestStart_RoutesWithSelfReference(t *testing.T) {
	t.Parallel()

	settings := extensiontest.NewNopSettings(extensiontest.NopType)
	selfID := settings.ID

	e, err := newExtension(
		&Config{
			Format: constants.FormatCloudWatchLogsSubscriptionFilter,
			CloudWatch: CloudWatchConfig{
				Streams: []subscriptionfilter.CloudWatchStream{
					{Name: "vpcflow", Encoding: selfID},
				},
			},
		},
		settings,
	)
	require.NoError(t, err)

	// Register the extension under its own selfID so resolution would otherwise
	// succeed; Start must still reject the cycle.
	host := &hostWithExtensions{
		Host: componenttest.NewNopHost(),
		extensions: map[component.ID]component.Component{
			selfID: e,
		},
	}
	err = e.Start(t.Context(), host)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "refers back to this extension")
}

// recordingInner records the bytes of every UnmarshalLogs call so tests can
// assert which inner was dispatched and what it received.
type recordingInner struct {
	component.StartFunc
	component.ShutdownFunc
	received [][]byte
}

func (r *recordingInner) UnmarshalLogs(buf []byte) (plog.Logs, error) {
	r.received = append(r.received, buf)
	logs := plog.NewLogs()
	logs.ResourceLogs().AppendEmpty()
	return logs, nil
}

func (*recordingInner) NewLogsDecoder(_ io.Reader, _ ...encoding.DecoderOption) (encoding.LogsDecoder, error) {
	return nil, nil
}

// startRoutedExtension is a small helper that builds a CW-routing extension
// wired to two inner extensions (cloudtrail and lambda routes) and starts
// it against a fake host. Returns the started extension plus the two inner
// instances so tests can inspect what was dispatched.
func startRoutedExtension(t *testing.T) (*encodingExtension, *recordingInner, *recordingInner) {
	t.Helper()

	cloudtrailID := component.MustNewIDWithName("aws_logs_encoding", "cloudtrail_inner")
	lambdaID := component.MustNewIDWithName("aws_logs_encoding", "lambda_inner")

	cloudtrailInner := &recordingInner{}
	lambdaInner := &recordingInner{}

	e, err := newExtension(
		&Config{
			Format: constants.FormatCloudWatchLogsSubscriptionFilter,
			CloudWatch: CloudWatchConfig{
				Streams: []subscriptionfilter.CloudWatchStream{
					{Name: "cloudtrail", Encoding: cloudtrailID}, // default *_CloudTrail_*
					{Name: "lambda", Encoding: lambdaID},         // default /aws/lambda/*
				},
			},
		},
		extensiontest.NewNopSettings(extensiontest.NopType),
	)
	require.NoError(t, err)

	host := &hostWithExtensions{
		Host: componenttest.NewNopHost(),
		extensions: map[component.ID]component.Component{
			cloudtrailID: cloudtrailInner,
			lambdaID:     lambdaInner,
		},
	}
	require.NoError(t, e.Start(t.Context(), host))

	return e, cloudtrailInner, lambdaInner
}

func TestUnmarshalLogs_RoutesToMatchedInner(t *testing.T) {
	t.Parallel()

	e, cloudtrailInner, lambdaInner := startRoutedExtension(t)

	// testdata/cloudwatch_log.json:
	//   logGroup: /aws/cloudtrail/management
	//   logStream: 123456789012_CloudTrail_us-east-1
	// Should match the "cloudtrail" route via its default *_CloudTrail_* pattern.
	gzipped := readAndCompressLogFile(t, "testdata/cloudwatch_log.json")
	_, err := e.UnmarshalLogs(gzipped)
	require.NoError(t, err)

	require.Len(t, cloudtrailInner.received, 1, "cloudtrail inner should have been called once")
	assert.Empty(t, lambdaInner.received, "lambda inner should not have been called")

	// Inner received decompressed JSON envelope, not the gzipped bytes.
	assert.NotEmpty(t, cloudtrailInner.received[0])
	assert.False(t, isGzipData(cloudtrailInner.received[0]), "inner should receive decompressed bytes")
}

func TestNewLogsDecoder_RoutesToMatchedInner(t *testing.T) {
	t.Parallel()

	e, cloudtrailInner, lambdaInner := startRoutedExtension(t)

	gzipped := readAndCompressLogFile(t, "testdata/cloudwatch_log.json")
	// NewLogsDecoder takes an io.Reader; the routing path reads it fully,
	// dispatches once, and returns a one-shot decoder.
	gz, err := gzip.NewReader(bytes.NewReader(gzipped))
	require.NoError(t, err)
	defer gz.Close()

	dec, err := e.NewLogsDecoder(gz)
	require.NoError(t, err)
	require.NotNil(t, dec)

	_, err = dec.DecodeLogs()
	require.NoError(t, err)

	// Second call yields EOF.
	_, err = dec.DecodeLogs()
	assert.ErrorIs(t, err, io.EOF)

	require.Len(t, cloudtrailInner.received, 1)
	assert.Empty(t, lambdaInner.received)
}
