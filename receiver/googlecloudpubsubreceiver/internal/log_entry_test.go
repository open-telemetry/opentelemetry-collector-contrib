// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal

import (
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

type Log struct {
	Timestamp          string
	ObservedTimestamp  string
	Body               any
	SeverityText       string
	Attributes         map[string]any
	ResourceAttributes map[string]any
	SpanID             string
	TraceID            string
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

	lr.SetSpanID(spanIDStrToSpanIDBytes(log.SpanID))
	lr.SetTraceID(traceIDStrTotraceIDBytes(log.TraceID))

	return res, lr, nil
}

func TestTranslateLogEntry(t *testing.T) {
	tests := []struct {
		input string
		want  Log
	}{
		// TODO: Add publicly shareable log test data.
	}
	for _, tt := range tests {
		var errs error
		wantRes, wantLr, err := generateLog(t, tt.want)
		errs = multierr.Append(errs, err)

		gotRes, gotLr, err := TranslateLogEntry([]byte(tt.input))
		errs = multierr.Append(errs, err)
		errs = multierr.Combine(errs, compareResources(wantRes, gotRes), compareLogRecords(wantLr, gotLr))

		require.NoError(t, errs)
	}
}

func compareResources(expected, actual pcommon.Resource) error {
	return compare("Resource.Attributes", expected.Attributes().AsRaw(), actual.Attributes().AsRaw())
}

func compareLogRecords(expected, actual plog.LogRecord) error {
	return multierr.Combine(
		compare("LogRecord.Timestamp", expected.Timestamp(), actual.Timestamp()),
		compare("LogRecord.Attributes", expected.Attributes().AsRaw(), actual.Attributes().AsRaw()),
		compare("LogRecord.Body", expected.Body().AsRaw(), actual.Body().AsRaw()),
	)
}

func compare(ty string, expected, actual any) error {
	if diff := cmp.Diff(expected, actual); diff != "" {
		return fmt.Errorf("%s mismatch (-expected +actual):\n%s", ty, diff)
	}
	return nil
}
