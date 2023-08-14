// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sqlqueryreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sqlqueryreceiver"

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

func TestLogsQueryReceiver_Collect(t *testing.T) {
	now := time.Now()

	fakeClient := &fakeDBClient{
		stringMaps: [][]stringMap{
			{{"col1": "42"}, {"col1": "63"}},
		},
	}
	queryReceiver := logsQueryReceiver{
		client: fakeClient,
		query: Query{
			Logs: []LogsCfg{
				{
					BodyColumn: "col1",
				},
			},
		},
	}
	logs, err := queryReceiver.collect(context.Background())
	assert.NoError(t, err)
	assert.NotNil(t, logs)
	assert.Equal(t, 2, logs.LogRecordCount())

	logRecord := logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
	assert.Equal(t, "42", logRecord.Body().Str())
	assert.GreaterOrEqual(t, logRecord.ObservedTimestamp(), pcommon.NewTimestampFromTime(now))

	logRecord = logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(1)
	assert.Equal(t, "63", logRecord.Body().Str())
	assert.NotEqual(t, logRecord.ObservedTimestamp(), pcommon.NewTimestampFromTime(now))

	assert.Equal(t,
		logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).ObservedTimestamp(),
		logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(1).ObservedTimestamp(),
		"Observed timestamps of all log records collected in a single scrape should be equal",
	)
}
