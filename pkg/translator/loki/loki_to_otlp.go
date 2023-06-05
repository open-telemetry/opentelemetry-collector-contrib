// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package loki // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/loki"

import (
	"fmt"
	"strings"
	"time"

	"github.com/grafana/loki/pkg/push"
	"github.com/prometheus/common/model"
	promql_parser "github.com/prometheus/prometheus/promql/parser"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
)

// PushRequestToLogs converts loki push request to logs pipeline data
func PushRequestToLogs(pushRequest *push.PushRequest, keepTimestamp bool) (plog.Logs, error) {
	logs := plog.NewLogs()
	// Return early if request does not contain any streams
	if len(pushRequest.Streams) == 0 {
		return logs, nil
	}
	rls := logs.ResourceLogs().AppendEmpty()
	logSlice := rls.ScopeLogs().AppendEmpty().LogRecords()

	var lastErr error
	var errNumber int64
	for _, stream := range pushRequest.Streams {
		// Return early if stream does not contain any entries
		if len(stream.Entries) == 0 {
			continue
		}
		// Get stream labels
		// Stream contains labels in string format: `{label1="value1", label2="value2"}`
		// Here we parse such a string into labels.Labels
		ls, err := promql_parser.ParseMetric(stream.Labels)
		if err != nil {
			lastErr = err
			errNumber++
			continue
		}

		// Convert to model.LabelSet
		filtered := model.LabelSet{}
		for _, label := range ls {
			// Labels started from __ are considered internal and should be ignored
			if strings.HasPrefix(label.Name, "__") {
				continue
			}
			filtered[model.LabelName(label.Name)] = model.LabelValue(label.Value)
		}

		for i := range stream.Entries {
			lr := logSlice.AppendEmpty()
			ConvertEntryToLogRecord(&stream.Entries[i], &lr, filtered, keepTimestamp)
		}
	}

	if lastErr != nil {
		lastErr = fmt.Errorf("%d entries failed to process, the last error: %w", errNumber, lastErr)
	}

	return logs, lastErr
}

// ConvertEntryToLogRecord converts loki log entry to otlp log record
func ConvertEntryToLogRecord(entry *push.Entry, lr *plog.LogRecord, labelSet model.LabelSet, keepTimestamp bool) {
	observedTimestamp := pcommon.NewTimestampFromTime(time.Now())
	lr.SetObservedTimestamp(observedTimestamp)
	if keepTimestamp && !entry.Timestamp.IsZero() {
		lr.SetTimestamp(pcommon.NewTimestampFromTime(entry.Timestamp))
	} else {
		lr.SetTimestamp(observedTimestamp)
	}
	lr.Body().SetStr(entry.Line)
	for key, value := range labelSet {
		lr.Attributes().PutStr(string(key), string(value))
	}
}
