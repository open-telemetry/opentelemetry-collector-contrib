// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package text

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

func TestTextUnmarshaler(t *testing.T) {
	t.Parallel()
	codec, err := newLogCodec("utf8")
	require.NoError(t, err)
	ld, err := codec.UnmarshalLogs([]byte("foo\nbar\n"))
	require.NoError(t, err)
	assert.Equal(t, 1, ld.LogRecordCount())
	for i := 0; i < ld.ResourceLogs().Len(); i++ {
		for j := 0; j < ld.ResourceLogs().At(i).ScopeLogs().Len(); j++ {
			for k := 0; k < ld.ResourceLogs().At(i).ScopeLogs().At(j).LogRecords().Len(); k++ {
				lr := ld.ResourceLogs().At(i).ScopeLogs().At(j).LogRecords().At(k)
				lr.SetObservedTimestamp(pcommon.NewTimestampFromTime(time.Unix(0, 0)))
			}
		}
	}
	b, err := codec.MarshalLogs(ld)
	require.NoError(t, err)
	require.Equal(t, `{"resourceLogs":[{"resource":{},"scopeLogs":[{"scope":{},"logRecords":[{"body":{"stringValue":"foo\nbar\n"},"traceId":"","spanId":""}]}]}]}`, string(b))
}
