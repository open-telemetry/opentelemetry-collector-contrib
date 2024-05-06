// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/multierr"
	"go.uber.org/zap"
)

type Log struct {
	Timestamp          string
	ObservedTimestamp  string
	Body               any
	SeverityText       string
	SeverityNumber     plog.SeverityNumber
	Attributes         map[string]any
	ResourceAttributes map[string]any
	SpanID             string
	TraceID            string
}

type testOptions struct {
}

func generateLog(t *testing.T, log Log) (pcommon.Resource, plog.LogRecord, error) {
	res := pcommon.NewResource()
	assert.NoError(t, res.Attributes().FromRaw(log.ResourceAttributes))

	lr := plog.NewLogRecord()
	err := lr.Attributes().FromRaw(log.Attributes)
	if err != nil {
		return res, lr, err
	}
	if log.Timestamp != "" {
		ts, err := time.Parse(time.RFC3339, log.Timestamp)
		if err != nil {
			return res, lr, err
		}
		lr.SetTimestamp(pcommon.NewTimestampFromTime(ts))
	}

	if log.ObservedTimestamp != "" {
		ots, err := time.Parse(time.RFC3339, log.ObservedTimestamp)
		if err != nil {
			return res, lr, err
		}
		lr.SetObservedTimestamp(pcommon.NewTimestampFromTime(ots))
	}

	assert.NoError(t, lr.Body().FromRaw(log.Body))
	lr.SetSeverityText(log.SeverityText)
	lr.SetSeverityNumber(log.SeverityNumber)

	spanID, _ := spanIDStrToSpanIDBytes(log.SpanID)
	lr.SetSpanID(spanID)
	traceID, _ := traceIDStrToTraceIDBytes(log.TraceID)
	lr.SetTraceID(traceID)

	return res, lr, nil
}

func TestCloudLoggingTraceToTraceIDBytes(t *testing.T) {
	for _, test := range []struct {
		scenario,
		in string
		out [16]byte
		err string
	}{
		// A valid trace
		{
			"ValidTrace",
			"projects/my-gcp-project/traces/1dbe317eb73eb6e3bbb51a2bc3a41e09",
			[16]uint8{0x1d, 0xbe, 0x31, 0x7e, 0xb7, 0x3e, 0xb6, 0xe3, 0xbb, 0xb5, 0x1a, 0x2b, 0xc3, 0xa4, 0x1e, 0x9},
			"",
		},
		// An invalid traces
		{
			"InvalidTrace",
			"1dbe317eb73eb6e3bbb51a2bc3a41e09",
			invalidTraceID,
			errorParsingLogItem.Error(),
		},
		{
			"InvalidHexTrace",
			"projects/my-gcp-project/traces/xyze317eb73eb6e3bbb51a2bc3a41e09",
			invalidTraceID,
			"encoding/hex: invalid byte: U+0078 'x'",
		},
		{
			"InvalidLengthTooLongTrace",
			"projects/my-gcp-project/traces/1dbe317eb73eb6e3bbb51a2bc3a41e09aa",
			invalidTraceID,
			errorParsingLogItem.Error(),
		},
		{
			"InvalidLengthTooShortTrace",
			"projects/my-gcp-project/traces/1dbe317eb73eb6e3bbb51a2bc3a41e",
			invalidTraceID,
			errorParsingLogItem.Error(),
		},
	} {
		t.Run(test.scenario, func(t *testing.T) {
			out, err := cloudLoggingTraceToTraceIDBytes(test.in)
			assert.Equal(t, test.out, out)
			if err != nil {
				assert.Equal(t, test.err, err.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}

}

func TestSpanIDStrToSpanIDBytes(t *testing.T) {
	for _, test := range []struct {
		scenario,
		in string
		out [8]byte
		err string
	}{
		// A valid span
		{
			"ValidSpan",
			"3e3a5741b18f0710",
			[8]uint8{0x3e, 0x3a, 0x57, 0x41, 0xb1, 0x8f, 0x7, 0x10},
			"",
		},
		// Cloud Run sends invalid span id's, make sure we're not crashing,
		// see https://issuetracker.google.com/issues/338634230?pli=1
		{
			"InvalidNumberSpan",
			"13210305202245662348",
			invalidSpanID,
			errorParsingLogItem.Error(),
		},
		// A invalid span
		{
			"ValidHexTooLongSpan",
			"3e3a5741b18f0710ab",
			invalidSpanID,
			errorParsingLogItem.Error(),
		},
		{
			"ValidHexTooShortSpan",
			"3e3a5741b18f07",
			invalidSpanID,
			errorParsingLogItem.Error(),
		},
	} {
		t.Run(test.scenario, func(t *testing.T) {
			out, err := spanIDStrToSpanIDBytes(test.in)
			assert.Equal(t, test.out, out)
			if err != nil {
				assert.Equal(t, test.err, err.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}

}

func TestTranslateLogEntry(t *testing.T) {

	tests := []struct {
		scenario string
		input    string
		want     Log
	}{
		{
			"RealLog",
			"{\"httpRequest\":{\"latency\":\"0.001901s\",\"protocol\":\"H2C\",\"referer\":\"https://console.cloud.google.com/\",\"remoteIp\":\"1802:2a14:1188:9898:34ab:b5c5:86eb:a142\",\"requestMethod\":\"GET\",\"requestSize\":\"1220\",\"requestUrl\":\"https://example.a.run.app/\",\"responseSize\":\"963\",\"serverIp\":\"2024:6048:3248:42::42\",\"status\":503,\"userAgent\":\"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36\"},\"insertId\":\"663760330006df7c63521728\",\"labels\":{\"instanceId\":\"00f46b9285fdad63aa1a6951b9b01126bd635517f264c8f231a40248b473fcdf266eed0fdfebd05fb338203c0e3e5bf6422f0b480445cacc4ec75bd4ebc86a72\"},\"logName\":\"projects/gcp-project/logs/run.googleapis.com%2Frequests\",\"receiveTimestamp\":\"2024-05-05T10:32:19.45570687Z\",\"resource\":{\"labels\":{\"configuration_name\":\"otelcol\",\"location\":\"europe-west1\",\"project_id\":\"gcp-project\",\"revision_name\":\"example-00007-sun\",\"service_name\":\"otelcol\"},\"type\":\"cloud_run_revision\"},\"severity\":\"ERROR\",\"spanId\":\"3e3a5741b18f0710\",\"timestamp\":\"2024-05-05T10:32:19.440152Z\",\"trace\":\"projects/gcp-project/traces/1dbe317eb73eb6e3bbb51a2bc3a41e09\",\"traceSampled\":true}",
			Log{
				Timestamp:         "2024-05-05T10:32:19.440152Z",
				ObservedTimestamp: "",
				Body:              nil,
				SeverityText:      "ERROR",
				SeverityNumber:    plog.SeverityNumberError,
				Attributes: map[string]any{
					"gcp.http_request": map[string]any{
						"latency":        string("0.001901s"),
						"protocol":       string("H2C"),
						"referer":        string("https://console.cloud.google.com/"),
						"remote_ip":      string("1802:2a14:1188:9898:34ab:b5c5:86eb:a142"),
						"request_method": string("GET"),
						"request_size":   int64(1220),
						"request_url":    string("https://example.a.run.app/"),
						"response_size":  int64(963),
						"server_ip":      string("2024:6048:3248:42::42"),
						"status":         int64(503),
						"user_agent":     string("Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36"),
					},
					"gcp.instance_id": string("00f46b9285fdad63aa1a6951b9b01126bd635517f264c8f231a40248b473fcdf266eed0fdfebd05fb338203c0e3e5bf6422f0b480445cacc4ec75bd4ebc86a72"),
					"gcp.log_name":    string("projects/gcp-project/logs/run.googleapis.com%2Frequests"),
					"log.record.uid":  string("663760330006df7c63521728"),
				},
				ResourceAttributes: map[string]any{
					"gcp.configuration_name": string("otelcol"),
					"gcp.location":           string("europe-west1"),
					"gcp.project_id":         string("gcp-project"),
					"gcp.revision_name":      string("example-00007-sun"),
					"gcp.service_name":       string("otelcol"),
					"gcp.resource_type":      string("cloud_run_revision"),
				},
				SpanID:  "3e3a5741b18f0710",
				TraceID: "1dbe317eb73eb6e3bbb51a2bc3a41e09",
			},
		},
		// This is the same log as above but one field (span) that causes an error, parsing will continue put added to attribute
		{
			"InvalidSpan",
			"{\"httpRequest\":{\"latency\":\"0.001901s\",\"protocol\":\"H2C\",\"referer\":\"https://console.cloud.google.com/\",\"remoteIp\":\"1802:2a14:1188:9898:34ab:b5c5:86eb:a142\",\"requestMethod\":\"GET\",\"requestSize\":\"1220\",\"requestUrl\":\"https://example.a.run.app/\",\"responseSize\":\"963\",\"serverIp\":\"2024:6048:3248:42::42\",\"status\":503,\"userAgent\":\"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36\"},\"insertId\":\"663760330006df7c63521728\",\"labels\":{\"instanceId\":\"00f46b9285fdad63aa1a6951b9b01126bd635517f264c8f231a40248b473fcdf266eed0fdfebd05fb338203c0e3e5bf6422f0b480445cacc4ec75bd4ebc86a72\"},\"logName\":\"projects/gcp-project/logs/run.googleapis.com%2Frequests\",\"receiveTimestamp\":\"2024-05-05T10:32:19.45570687Z\",\"resource\":{\"labels\":{\"configuration_name\":\"otelcol\",\"location\":\"europe-west1\",\"project_id\":\"gcp-project\",\"revision_name\":\"example-00007-sun\",\"service_name\":\"otelcol\"},\"type\":\"cloud_run_revision\"},\"severity\":\"ERROR\",\"spanId\":\"13210305202245662348\",\"timestamp\":\"2024-05-05T10:32:19.440152Z\",\"trace\":\"projects/gcp-project/traces/1dbe317eb73eb6e3bbb51a2bc3a41e09\",\"traceSampled\":true}",
			Log{
				Timestamp:         "2024-05-05T10:32:19.440152Z",
				ObservedTimestamp: "",
				Body:              nil,
				SeverityText:      "ERROR",
				SeverityNumber:    plog.SeverityNumberError,
				Attributes: map[string]any{
					"gcp.http_request": map[string]any{
						"latency":        string("0.001901s"),
						"protocol":       string("H2C"),
						"referer":        string("https://console.cloud.google.com/"),
						"remote_ip":      string("1802:2a14:1188:9898:34ab:b5c5:86eb:a142"),
						"request_method": string("GET"),
						"request_size":   int64(1220),
						"request_url":    string("https://example.a.run.app/"),
						"response_size":  int64(963),
						"server_ip":      string("2024:6048:3248:42::42"),
						"status":         int64(503),
						"user_agent":     string("Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36"),
					},
					"gcp.instance_id": string("00f46b9285fdad63aa1a6951b9b01126bd635517f264c8f231a40248b473fcdf266eed0fdfebd05fb338203c0e3e5bf6422f0b480445cacc4ec75bd4ebc86a72"),
					"gcp.log_name":    string("projects/gcp-project/logs/run.googleapis.com%2Frequests"),
					// this attribute added
					"gcp.span_id":    string("13210305202245662348"),
					"log.record.uid": string("663760330006df7c63521728"),
				},
				ResourceAttributes: map[string]any{
					"gcp.configuration_name": string("otelcol"),
					"gcp.location":           string("europe-west1"),
					"gcp.project_id":         string("gcp-project"),
					"gcp.revision_name":      string("example-00007-sun"),
					"gcp.service_name":       string("otelcol"),
					"gcp.resource_type":      string("cloud_run_revision"),
				},
				SpanID:  "",
				TraceID: "1dbe317eb73eb6e3bbb51a2bc3a41e09",
			},
		},
		// Payloads
		{
			"TextPayload",
			"{\"textPayload\":\"Hello World\"}",
			Log{
				Body: string("Hello World"),
			},
		},
		{
			"TextPayloadParseError",
			"{\"textPayload\":42}",
			Log{
				Attributes: map[string]any{
					"gcp.text_payload": nil,
				},
			},
		},
		{
			"JsonPayloadParse",
			"{\"jsonPayload\":{\"foo\":\"bar\"}}",
			Log{
				Body: map[string]any{"foo": string("bar")},
			},
		},
		// Severity
		{
			"SeverityNumberDebug",
			"{\"severity\":\"DEBUG\"}",
			Log{
				SeverityText:   "DEBUG",
				SeverityNumber: plog.SeverityNumberDebug,
			},
		},
		{
			"SeverityNumberInfo",
			"{\"severity\":\"INFO\"}",
			Log{
				SeverityText:   "INFO",
				SeverityNumber: plog.SeverityNumberInfo,
			},
		},
		{
			"SeverityNumberInfo2",
			"{\"severity\":\"NOTICE\"}",
			Log{
				SeverityText:   "NOTICE",
				SeverityNumber: plog.SeverityNumberInfo2,
			},
		},
		{
			"SeverityNumberWarn",
			"{\"severity\":\"WARNING\"}",
			Log{
				SeverityText:   "WARNING",
				SeverityNumber: plog.SeverityNumberWarn,
			},
		},
		{
			"SeverityNumberError",
			"{\"severity\":\"ERROR\"}",
			Log{
				SeverityText:   "ERROR",
				SeverityNumber: plog.SeverityNumberError,
			},
		},
		{
			"SeverityNumberFatal",
			"{\"severity\":\"CRITICAL\"}",
			Log{
				SeverityText:   "CRITICAL",
				SeverityNumber: plog.SeverityNumberFatal,
			},
		},
		{
			"SeverityNumberFatal2",
			"{\"severity\":\"ALERT\"}",
			Log{
				SeverityText:   "ALERT",
				SeverityNumber: plog.SeverityNumberFatal2,
			},
		},
		{
			"SeverityNumberFatal4",
			"{\"severity\":\"EMERGENCY\"}",
			Log{
				SeverityText:   "EMERGENCY",
				SeverityNumber: plog.SeverityNumberFatal4,
			},
		},
	}
	logger, _ := zap.NewDevelopment()
	var options testOptions
	for _, tt := range tests {
		t.Run(tt.scenario, func(t *testing.T) {
			var errs error
			wantRes, wantLr, err := generateLog(t, tt.want)
			errs = multierr.Append(errs, err)

			gotRes, gotLr, err := TranslateLogEntry(context.TODO(), logger, []byte(tt.input), options)
			errs = multierr.Append(errs, err)
			errs = multierr.Combine(errs, compareResources(wantRes, gotRes), compareLogRecords(wantLr, gotLr))

			require.NoError(t, errs)
		})
	}
}

func compareResources(expected, actual pcommon.Resource) error {
	return compare("Resource.Attributes", expected.Attributes().AsRaw(), actual.Attributes().AsRaw())
}

func compareLogRecords(expected, actual plog.LogRecord) error {
	return multierr.Combine(
		compare("LogRecord.Timestamp", expected.Timestamp(), actual.Timestamp()),
		compare("LogRecord.TraceID", expected.TraceID(), actual.TraceID()),
		compare("LogRecord.SpanID", expected.SpanID(), actual.SpanID()),
		compare("LogRecord.Attributes", expected.Attributes().AsRaw(), actual.Attributes().AsRaw()),
		compare("LogRecord.Body", expected.Body().AsRaw(), actual.Body().AsRaw()),
		compare("LogRecord.SeverityText", expected.SeverityText(), actual.SeverityText()),
		compare("LogRecord.SeverityNumber", expected.SeverityNumber(), actual.SeverityNumber()),
	)
}

func compare(ty string, expected, actual any) error {
	if diff := cmp.Diff(expected, actual); diff != "" {
		return fmt.Errorf("%s mismatch (-expected +actual):\n%s", ty, diff)
	}
	return nil
}

func TestTranslateJsonPayloadLogEntry(t *testing.T) {

	tests := []struct {
		scenario   string
		input      string
		wantAsJSON Log
	}{
		{
			"ProtoPayload",
			"{\"jsonPayload\": { \"foo\": \"bar\", \"other\": 42} }",
			Log{
				Body: map[string]any{
					"foo":   string("bar"),
					"other": float64(42),
				},
			},
		},
	}
	logger, _ := zap.NewDevelopment()
	for _, tt := range tests {
		fn := func(t *testing.T, options testOptions, want Log) {
			var errs error
			wantRes, wantLr, err := generateLog(t, want)
			errs = multierr.Append(errs, err)

			gotRes, gotLr, err := TranslateLogEntry(context.TODO(), logger, []byte(tt.input), options)
			errs = multierr.Append(errs, err)
			errs = multierr.Combine(errs, compareResources(wantRes, gotRes), compareLogRecords(wantLr, gotLr))

			require.NoError(t, errs)
		}
		var options testOptions
		t.Run(tt.scenario, func(t *testing.T) {
			fn(t, options, tt.wantAsJSON)
		})
	}
}
