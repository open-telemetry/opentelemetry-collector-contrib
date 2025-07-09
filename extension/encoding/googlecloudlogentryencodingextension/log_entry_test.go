// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package googlecloudlogentryencodingextension

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
)

type log struct {
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

func generateLog(t *testing.T, log log) (pcommon.Resource, plog.LogRecord, error) {
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

func TestJsonPayload(t *testing.T) {
	tests := []struct {
		scenario string
		input    string
		config   Config
		expected log
	}{
		{
			"AsJSON",
			"{\"jsonPayload\": { \"foo\": \"bar\", \"other\": 42} }",
			Config{
				HandleJSONPayloadAs:  HandleAsJSON,
				HandleProtoPayloadAs: HandleAsJSON,
			},
			log{
				Body: map[string]any{
					"foo":   string("bar"),
					"other": float64(42),
				},
			},
		},
		{
			"AsText",
			"{\"jsonPayload\": { \"foo\": \"bar\", \"other\": 42} }",
			Config{
				HandleJSONPayloadAs:  HandleAsText,
				HandleProtoPayloadAs: HandleAsJSON,
			},
			log{
				Body: "{ \"foo\": \"bar\", \"other\": 42}",
			},
		},
	}
	for _, tt := range tests {
		fn := func(t *testing.T, cfg *Config, want log) {
			extension := newConfiguredExtension(t, cfg)
			defer assert.NoError(t, extension.Shutdown(context.Background()))

			var errs error
			wantRes, wantLr, err := generateLog(t, want)
			errs = multierr.Append(errs, err)

			gotRes, gotLr, err := extension.translateLogEntry([]byte(tt.input))
			errs = multierr.Append(errs, err)
			errs = multierr.Combine(errs, compareResources(wantRes, gotRes), compareLogRecords(wantLr, gotLr))

			require.NoError(t, errs)
		}
		t.Run(tt.scenario, func(t *testing.T) {
			fn(t, &tt.config, tt.expected)
		})
	}
}
