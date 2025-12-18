// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awslogsencodingextension

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/klauspost/compress/gzip"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/extension/extensiontest"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension/internal/constants"
)

func TestNewLogsStreamDecoder_WAFLog(t *testing.T) {
	ext, err := newExtension(&Config{Format: constants.FormatWAFLog}, extensiontest.NewNopSettings(extensiontest.NopType))
	require.NoError(t, err)

	// Create test data: multiple WAF log lines
	wafLogLines := []string{
		`{"timestamp":1234567890000,"webaclId":"arn:aws:wafv2:us-east-1:123456789012:regional/webacl/test/abc123","terminatingRuleId":"test-rule","terminatingRuleType":"REGULAR","action":"ALLOW","httpSourceName":"ALB","httpSourceId":"arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/test/abc123","httpRequest":{"clientIp":"1.2.3.4","country":"US","headers":[],"uri":"/test","args":"","httpVersion":"HTTP/1.1","httpMethod":"GET","requestID":"test-request-id","fragment":"","scheme":"https","host":"example.com"}}`,
		`{"timestamp":1234567891000,"webaclId":"arn:aws:wafv2:us-east-1:123456789012:regional/webacl/test/abc123","terminatingRuleId":"test-rule-2","terminatingRuleType":"REGULAR","action":"BLOCK","httpSourceName":"ALB","httpSourceId":"arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/test/abc123","httpRequest":{"clientIp":"5.6.7.8","country":"CA","headers":[],"uri":"/test2","args":"","httpVersion":"HTTP/1.1","httpMethod":"POST","requestID":"test-request-id-2","fragment":"","scheme":"https","host":"example.com"}}`,
	}
	testData := strings.Join(wafLogLines, "\n")

	decoder, err := ext.NewLogsStreamDecoder(context.Background(), strings.NewReader(testData))
	require.NoError(t, err)
	require.NotNil(t, decoder)

	// Decode first batch
	logs := plog.NewLogs()
	err = decoder.Decode(context.Background(), logs)
	require.NoError(t, err)
	assert.Greater(t, logs.LogRecordCount(), int(0))
	assert.Equal(t, encoding.StreamOffset(2), decoder.Offset()) // Should have consumed 2 log records

	// Decode second batch (should be EOF)
	logs2 := plog.NewLogs()
	err = decoder.Decode(context.Background(), logs2)
	assert.ErrorIs(t, err, io.EOF)
}

func TestNewLogsStreamDecoder_CloudWatchSubscriptionFilter(t *testing.T) {
	ext, err := newExtension(&Config{Format: constants.FormatCloudWatchLogsSubscriptionFilter}, extensiontest.NewNopSettings(extensiontest.NopType))
	require.NoError(t, err)

	// Read test data
	testData := readAndCompressLogFile(t, "testdata/cloudwatch_log.json")

	decoder, err := ext.NewLogsStreamDecoder(context.Background(), bytes.NewReader(testData))
	require.NoError(t, err)
	require.NotNil(t, decoder)

	// Decode the document
	logs := plog.NewLogs()
	err = decoder.Decode(context.Background(), logs)
	require.NoError(t, err)
	assert.Greater(t, logs.LogRecordCount(), int(0))

	// Should be EOF on next decode
	logs2 := plog.NewLogs()
	err = decoder.Decode(context.Background(), logs2)
	assert.ErrorIs(t, err, io.EOF)
}

func TestNewLogsStreamDecoder_WithFlushItems(t *testing.T) {
	ext, err := newExtension(&Config{Format: constants.FormatWAFLog}, extensiontest.NewNopSettings(extensiontest.NopType))
	require.NoError(t, err)

	// Create 5 WAF log lines
	wafLogLines := make([]string, 5)
	for i := 0; i < 5; i++ {
		wafLogLines[i] = `{"timestamp":1234567890000,"webaclId":"arn:aws:wafv2:us-east-1:123456789012:regional/webacl/test/abc123","terminatingRuleId":"test-rule","terminatingRuleType":"REGULAR","action":"ALLOW","httpSourceName":"ALB","httpSourceId":"arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/test/abc123","httpRequest":{"clientIp":"1.2.3.4","country":"US","headers":[],"uri":"/test","args":"","httpVersion":"HTTP/1.1","httpMethod":"GET","requestID":"test-request-id","fragment":"","scheme":"https","host":"example.com"}}`
	}
	testData := strings.Join(wafLogLines, "\n")

	decoder, err := ext.NewLogsStreamDecoder(
		context.Background(),
		strings.NewReader(testData),
		encoding.WithFlushItems(2),
	)
	require.NoError(t, err)

	// First decode should return 2 items
	logs1 := plog.NewLogs()
	err = decoder.Decode(context.Background(), logs1)
	require.NoError(t, err)
	assert.Equal(t, encoding.StreamOffset(2), decoder.Offset())

	// Second decode should return 2 more items
	logs2 := plog.NewLogs()
	err = decoder.Decode(context.Background(), logs2)
	require.NoError(t, err)
	assert.Equal(t, encoding.StreamOffset(4), decoder.Offset())

	// Third decode should return the last item
	logs3 := plog.NewLogs()
	err = decoder.Decode(context.Background(), logs3)
	require.NoError(t, err)
	assert.Equal(t, encoding.StreamOffset(5), decoder.Offset())

	// Fourth decode should be EOF
	logs4 := plog.NewLogs()
	err = decoder.Decode(context.Background(), logs4)
	assert.ErrorIs(t, err, io.EOF)
}

func TestNewLogsStreamDecoder_WithInitialOffset(t *testing.T) {
	ext, err := newExtension(&Config{Format: constants.FormatWAFLog}, extensiontest.NewNopSettings(extensiontest.NopType))
	require.NoError(t, err)

	// Create 5 WAF log lines
	wafLogLines := make([]string, 5)
	for i := 0; i < 5; i++ {
		wafLogLines[i] = `{"timestamp":1234567890000,"webaclId":"arn:aws:wafv2:us-east-1:123456789012:regional/webacl/test/abc123","terminatingRuleId":"test-rule","terminatingRuleType":"REGULAR","action":"ALLOW","httpSourceName":"ALB","httpSourceId":"arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/test/abc123","httpRequest":{"clientIp":"1.2.3.4","country":"US","headers":[],"uri":"/test","args":"","httpVersion":"HTTP/1.1","httpMethod":"GET","requestID":"test-request-id","fragment":"","scheme":"https","host":"example.com"}}`
	}
	testData := strings.Join(wafLogLines, "\n")

	// Start at offset 2
	decoder, err := ext.NewLogsStreamDecoder(
		context.Background(),
		strings.NewReader(testData),
		encoding.WithInitialOffset(2),
	)
	require.NoError(t, err)
	assert.Equal(t, encoding.StreamOffset(2), decoder.Offset())

	// Decode should start from the 3rd item
	logs := plog.NewLogs()
	err = decoder.Decode(context.Background(), logs)
	require.NoError(t, err)
	assert.Greater(t, logs.LogRecordCount(), int(0))
	assert.Equal(t, encoding.StreamOffset(5), decoder.Offset()) // Should have consumed remaining 3 items
}

func TestNewLogsStreamDecoder_WithGzip(t *testing.T) {
	ext, err := newExtension(&Config{Format: constants.FormatWAFLog}, extensiontest.NewNopSettings(extensiontest.NopType))
	require.NoError(t, err)

	// Create test data and compress it
	wafLogLine := `{"timestamp":1234567890000,"webaclId":"arn:aws:wafv2:us-east-1:123456789012:regional/webacl/test/abc123","terminatingRuleId":"test-rule","terminatingRuleType":"REGULAR","action":"ALLOW","httpSourceName":"ALB","httpSourceId":"arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/test/abc123","httpRequest":{"clientIp":"1.2.3.4","country":"US","headers":[],"uri":"/test","args":"","httpVersion":"HTTP/1.1","httpMethod":"GET","requestID":"test-request-id","fragment":"","scheme":"https","host":"example.com"}}`
	var compressed bytes.Buffer
	gz := gzip.NewWriter(&compressed)
	_, err = gz.Write([]byte(wafLogLine))
	require.NoError(t, err)
	require.NoError(t, gz.Close())

	decoder, err := ext.NewLogsStreamDecoder(context.Background(), &compressed)
	require.NoError(t, err)

	logs := plog.NewLogs()
	err = decoder.Decode(context.Background(), logs)
	require.NoError(t, err)
	assert.Greater(t, logs.LogRecordCount(), int(0))
}

func TestNewLogsStreamDecoder_WithFlushTimeout(t *testing.T) {
	ext, err := newExtension(&Config{Format: constants.FormatWAFLog}, extensiontest.NewNopSettings(extensiontest.NopType))
	require.NoError(t, err)

	wafLogLine := `{"timestamp":1234567890000,"webaclId":"arn:aws:wafv2:us-east-1:123456789012:regional/webacl/test/abc123","terminatingRuleId":"test-rule","terminatingRuleType":"REGULAR","action":"ALLOW","httpSourceName":"ALB","httpSourceId":"arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/test/abc123","httpRequest":{"clientIp":"1.2.3.4","country":"US","headers":[],"uri":"/test","args":"","httpVersion":"HTTP/1.1","httpMethod":"GET","requestID":"test-request-id","fragment":"","scheme":"https","host":"example.com"}}`

	decoder, err := ext.NewLogsStreamDecoder(
		context.Background(),
		strings.NewReader(wafLogLine),
		encoding.WithFlushTimeout(100*time.Millisecond),
	)
	require.NoError(t, err)

	// Decode immediately should work
	logs := plog.NewLogs()
	err = decoder.Decode(context.Background(), logs)
	require.NoError(t, err)

	// Wait for timeout and decode again (should be EOF)
	time.Sleep(150 * time.Millisecond)
	logs2 := plog.NewLogs()
	err = decoder.Decode(context.Background(), logs2)
	assert.ErrorIs(t, err, io.EOF)
}

func TestNewLogsStreamDecoder_EmptyStream(t *testing.T) {
	ext, err := newExtension(&Config{Format: constants.FormatWAFLog}, extensiontest.NewNopSettings(extensiontest.NopType))
	require.NoError(t, err)

	decoder, err := ext.NewLogsStreamDecoder(context.Background(), strings.NewReader(""))
	require.NoError(t, err)

	logs := plog.NewLogs()
	err = decoder.Decode(context.Background(), logs)
	assert.ErrorIs(t, err, io.EOF)
}

func TestNewLogsStreamDecoder_ContextCancellation(t *testing.T) {
	ext, err := newExtension(&Config{Format: constants.FormatWAFLog}, extensiontest.NewNopSettings(extensiontest.NopType))
	require.NoError(t, err)

	wafLogLine := `{"timestamp":1234567890000,"webaclId":"arn:aws:wafv2:us-east-1:123456789012:regional/webacl/test/abc123","terminatingRuleId":"test-rule","terminatingRuleType":"REGULAR","action":"ALLOW","httpSourceName":"ALB","httpSourceId":"arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/test/abc123","httpRequest":{"clientIp":"1.2.3.4","country":"US","headers":[],"uri":"/test","args":"","httpVersion":"HTTP/1.1","httpMethod":"GET","requestID":"test-request-id","fragment":"","scheme":"https","host":"example.com"}}`

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	decoder, err := ext.NewLogsStreamDecoder(ctx, strings.NewReader(wafLogLine))
	require.NoError(t, err)

	logs := plog.NewLogs()
	err = decoder.Decode(ctx, logs)
	assert.ErrorIs(t, err, context.Canceled)
}


