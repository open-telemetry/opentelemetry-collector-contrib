// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package internal

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
	"go.uber.org/multierr"
	"go.uber.org/zap"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
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

func generateLog(log Log) (pcommon.Resource, plog.LogRecord, error) {
	res := pcommon.NewResource()
	res.Attributes().FromRaw(log.ResourceAttributes)

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

	lr.Body().FromRaw(log.Body)
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

	logger, _ := zap.NewDevelopment()

	for _, tt := range tests {
		var errs error
		wantRes, wantLr, err := generateLog(tt.want)
		errs = multierr.Append(errs, err)

		gotRes, gotLr, err := TranslateLogEntry(context.TODO(), logger, []byte(tt.input))
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

func compare(ty string, expected, actual any, opts ...cmp.Option) error {
	if diff := cmp.Diff(expected, actual, opts...); diff != "" {
		return fmt.Errorf("%s mismatch (-expected +actual):\n%s", ty, diff)
	}
	return nil
}
