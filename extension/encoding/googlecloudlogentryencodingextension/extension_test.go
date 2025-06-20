// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package googlecloudlogentryencodingextension

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/multierr"
)

func newInitialisedExtension(t *testing.T) *ext {
	factory := NewFactory()
	extension := newExtension(factory.CreateDefaultConfig().(*Config))
	assert.NoError(t, extension.Start(context.Background(), componenttest.NewNopHost()))
	return extension
}

func newConfiguredExtension(t *testing.T, config *Config) *ext {
	extension := newExtension(config)
	assert.NoError(t, extension.Start(context.Background(), componenttest.NewNopHost()))
	return extension
}

func TestUnmarshalLogs(t *testing.T) {
	tests := []struct {
		scenario string
		input    string
		want     log
	}{
		{
			"RealLog",
			"{\"httpRequest\":{\"latency\":\"0.001901s\",\"protocol\":\"H2C\",\"referer\":\"https://console.cloud.google.com/\",\"remoteIp\":\"1802:2a14:1188:9898:34ab:b5c5:86eb:a142\",\"requestMethod\":\"GET\",\"requestSize\":\"1220\",\"requestUrl\":\"https://example.a.run.app/\",\"responseSize\":\"963\",\"serverIp\":\"2024:6048:3248:42::42\",\"status\":503,\"userAgent\":\"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36\"},\"insertId\":\"663760330006df7c63521728\",\"labels\":{\"instanceId\":\"00f46b9285fdad63aa1a6951b9b01126bd635517f264c8f231a40248b473fcdf266eed0fdfebd05fb338203c0e3e5bf6422f0b480445cacc4ec75bd4ebc86a72\"},\"logName\":\"projects/gcp-project/logs/run.googleapis.com%2Frequests\",\"receiveTimestamp\":\"2024-05-05T10:32:19.45570687Z\",\"resource\":{\"labels\":{\"configuration_name\":\"otelcol\",\"location\":\"europe-west1\",\"project_id\":\"gcp-project\",\"revision_name\":\"example-00007-sun\",\"service_name\":\"otelcol\"},\"type\":\"cloud_run_revision\"},\"severity\":\"ERROR\",\"spanId\":\"3e3a5741b18f0710\",\"timestamp\":\"2024-05-05T10:32:19.440152Z\",\"trace\":\"projects/gcp-project/traces/1dbe317eb73eb6e3bbb51a2bc3a41e09\",\"traceSampled\":true}",
			log{
				Timestamp:         "2024-05-05T10:32:19.440152Z",
				ObservedTimestamp: "",
				Body:              nil,
				SeverityText:      "ERROR",
				SeverityNumber:    plog.SeverityNumberError,
				Attributes: map[string]any{
					"gcp.http_request": map[string]any{
						"latency":        "0.001901s",
						"protocol":       "H2C",
						"referer":        "https://console.cloud.google.com/",
						"remote_ip":      "1802:2a14:1188:9898:34ab:b5c5:86eb:a142",
						"request_method": "GET",
						"request_size":   int64(1220),
						"request_url":    "https://example.a.run.app/",
						"response_size":  int64(963),
						"server_ip":      "2024:6048:3248:42::42",
						"status":         int64(503),
						"user_agent":     "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36",
					},
					"gcp.instance_id": "00f46b9285fdad63aa1a6951b9b01126bd635517f264c8f231a40248b473fcdf266eed0fdfebd05fb338203c0e3e5bf6422f0b480445cacc4ec75bd4ebc86a72",
					"gcp.log_name":    "projects/gcp-project/logs/run.googleapis.com%2Frequests",
					"log.record.uid":  "663760330006df7c63521728",
				},
				ResourceAttributes: map[string]any{
					"gcp.configuration_name": "otelcol",
					"gcp.location":           "europe-west1",
					"gcp.project_id":         "gcp-project",
					"gcp.revision_name":      "example-00007-sun",
					"gcp.service_name":       "otelcol",
					"gcp.resource_type":      "cloud_run_revision",
				},
				SpanID:  "3e3a5741b18f0710",
				TraceID: "1dbe317eb73eb6e3bbb51a2bc3a41e09",
			},
		},
		// This is the same log as above but one field (span) that causes an error, parsing will continue put added to attribute
		{
			"InvalidSpan",
			"{\"httpRequest\":{\"latency\":\"0.001901s\",\"protocol\":\"H2C\",\"referer\":\"https://console.cloud.google.com/\",\"remoteIp\":\"1802:2a14:1188:9898:34ab:b5c5:86eb:a142\",\"requestMethod\":\"GET\",\"requestSize\":\"1220\",\"requestUrl\":\"https://example.a.run.app/\",\"responseSize\":\"963\",\"serverIp\":\"2024:6048:3248:42::42\",\"status\":503,\"userAgent\":\"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36\"},\"insertId\":\"663760330006df7c63521728\",\"labels\":{\"instanceId\":\"00f46b9285fdad63aa1a6951b9b01126bd635517f264c8f231a40248b473fcdf266eed0fdfebd05fb338203c0e3e5bf6422f0b480445cacc4ec75bd4ebc86a72\"},\"logName\":\"projects/gcp-project/logs/run.googleapis.com%2Frequests\",\"receiveTimestamp\":\"2024-05-05T10:32:19.45570687Z\",\"resource\":{\"labels\":{\"configuration_name\":\"otelcol\",\"location\":\"europe-west1\",\"project_id\":\"gcp-project\",\"revision_name\":\"example-00007-sun\",\"service_name\":\"otelcol\"},\"type\":\"cloud_run_revision\"},\"severity\":\"ERROR\",\"spanId\":\"13210305202245662348\",\"timestamp\":\"2024-05-05T10:32:19.440152Z\",\"trace\":\"projects/gcp-project/traces/1dbe317eb73eb6e3bbb51a2bc3a41e09\",\"traceSampled\":true}",
			log{
				Timestamp:         "2024-05-05T10:32:19.440152Z",
				ObservedTimestamp: "",
				Body:              nil,
				SeverityText:      "ERROR",
				SeverityNumber:    plog.SeverityNumberError,
				Attributes: map[string]any{
					"gcp.http_request": map[string]any{
						"latency":        "0.001901s",
						"protocol":       "H2C",
						"referer":        "https://console.cloud.google.com/",
						"remote_ip":      "1802:2a14:1188:9898:34ab:b5c5:86eb:a142",
						"request_method": "GET",
						"request_size":   int64(1220),
						"request_url":    "https://example.a.run.app/",
						"response_size":  int64(963),
						"server_ip":      "2024:6048:3248:42::42",
						"status":         int64(503),
						"user_agent":     "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36",
					},
					"gcp.instance_id": "00f46b9285fdad63aa1a6951b9b01126bd635517f264c8f231a40248b473fcdf266eed0fdfebd05fb338203c0e3e5bf6422f0b480445cacc4ec75bd4ebc86a72",
					"gcp.log_name":    "projects/gcp-project/logs/run.googleapis.com%2Frequests",
					// this attribute added
					"gcp.span_id":    "13210305202245662348",
					"log.record.uid": "663760330006df7c63521728",
				},
				ResourceAttributes: map[string]any{
					"gcp.configuration_name": "otelcol",
					"gcp.location":           "europe-west1",
					"gcp.project_id":         "gcp-project",
					"gcp.revision_name":      "example-00007-sun",
					"gcp.service_name":       "otelcol",
					"gcp.resource_type":      "cloud_run_revision",
				},
				SpanID:  "",
				TraceID: "1dbe317eb73eb6e3bbb51a2bc3a41e09",
			},
		},
		// Payloads
		{
			"TextPayload",
			"{\"textPayload\":\"Hello World\"}",
			log{
				Body: "Hello World",
			},
		},
		{
			"JsonPayloadParse",
			"{\"jsonPayload\":{\"foo\":\"bar\"}}",
			log{
				Body: map[string]any{"foo": "bar"},
			},
		},
		// Severity
		{
			"SeverityNumberDebug",
			"{\"severity\":\"DEBUG\"}",
			log{
				SeverityText:   "DEBUG",
				SeverityNumber: plog.SeverityNumberDebug,
			},
		},
		{
			"SeverityNumberInfo",
			"{\"severity\":\"INFO\"}",
			log{
				SeverityText:   "INFO",
				SeverityNumber: plog.SeverityNumberInfo,
			},
		},
		{
			"SeverityNumberInfo2",
			"{\"severity\":\"NOTICE\"}",
			log{
				SeverityText:   "NOTICE",
				SeverityNumber: plog.SeverityNumberInfo2,
			},
		},
		{
			"SeverityNumberWarn",
			"{\"severity\":\"WARNING\"}",
			log{
				SeverityText:   "WARNING",
				SeverityNumber: plog.SeverityNumberWarn,
			},
		},
		{
			"SeverityNumberError",
			"{\"severity\":\"ERROR\"}",
			log{
				SeverityText:   "ERROR",
				SeverityNumber: plog.SeverityNumberError,
			},
		},
		{
			"SeverityNumberFatal",
			"{\"severity\":\"CRITICAL\"}",
			log{
				SeverityText:   "CRITICAL",
				SeverityNumber: plog.SeverityNumberFatal,
			},
		},
		{
			"SeverityNumberFatal2",
			"{\"severity\":\"ALERT\"}",
			log{
				SeverityText:   "ALERT",
				SeverityNumber: plog.SeverityNumberFatal2,
			},
		},
		{
			"SeverityNumberFatal4",
			"{\"severity\":\"EMERGENCY\"}",
			log{
				SeverityText:   "EMERGENCY",
				SeverityNumber: plog.SeverityNumberFatal4,
			},
		},
	}
	extension := newInitialisedExtension(t)
	for _, tt := range tests {
		t.Run(tt.scenario, func(t *testing.T) {
			var errs error
			wantRes, wantLr, err := generateLog(t, tt.want)
			errs = multierr.Append(errs, err)

			logs, err := extension.UnmarshalLogs([]byte(tt.input))

			errs = multierr.Append(errs, err)
			errs = multierr.Combine(errs,
				compareResources(wantRes, logs.ResourceLogs().At(0).Resource()),
				compareLogRecords(wantLr, logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)))
			require.NoError(t, errs)
		})
	}
}

func TestUnmarshalErrors(t *testing.T) {
	tests := []struct {
		scenario string
		input    string
		error    string
	}{
		{
			"TextPlayloadNoString",
			"{\"textPayload\":42}",
			"ReadString: expects",
		},
	}

	config := Config{
		HandleProtoPayloadAs: HandleAsProtobuf,
	}
	for _, tt := range tests {
		fn := func(t *testing.T) {
			extension := newConfiguredExtension(t, &config)
			defer assert.NoError(t, extension.Shutdown(context.Background()))

			var errs error
			_, _, translateError := extension.translateLogEntry([]byte(tt.input))

			assert.ErrorContains(t, translateError, tt.error)
			require.NoError(t, errs)
		}
		t.Run(tt.scenario, func(t *testing.T) {
			fn(t)
		})
	}
}
