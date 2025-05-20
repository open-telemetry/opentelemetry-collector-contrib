// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package unmarshaler

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
)

func TestRawLogsUnmarshaler(t *testing.T) {
	var u plog.Unmarshaler = RawLogsUnmarshaler{}
	input := "test\123abc"
	logs, err := u.UnmarshalLogs([]byte(input))
	require.NoError(t, err)

	require.Equal(t, 1, logs.LogRecordCount())
	body := logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Body()
	assert.Equal(t, pcommon.ValueTypeBytes, body.Type())
	assert.Equal(t, input, string(body.Bytes().AsRaw()))
}
