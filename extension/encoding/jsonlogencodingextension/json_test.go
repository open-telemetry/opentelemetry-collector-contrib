// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package jsonlogencodingextension

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
)

func TestMarshalUnmarshal(t *testing.T) {
	t.Parallel()
	e := &jsonExtension{}
	json := `{"example":"example valid json to test that the unmarshaler is correctly returning a plog value"}`
	ld, err := e.UnmarshalLogs([]byte(json))
	assert.NoError(t, err)
	assert.Equal(t, 1, ld.LogRecordCount())
	for i := 0; i < ld.ResourceLogs().Len(); i++ {
		for j := 0; j < ld.ResourceLogs().At(i).ScopeLogs().Len(); j++ {
			for k := 0; k < ld.ResourceLogs().At(i).ScopeLogs().At(j).LogRecords().Len(); k++ {
				lr := ld.ResourceLogs().At(i).ScopeLogs().At(j).LogRecords().At(k)
				lr.SetObservedTimestamp(pcommon.NewTimestampFromTime(time.Unix(0, 0)))
			}
		}
	}

	buf, err := e.MarshalLogs(ld)
	assert.NoError(t, err)
	assert.True(t, len(buf) > 0)
	assert.Equal(t, json, string(buf))
}

func TestInvalidMarshal(t *testing.T) {
	e := &jsonExtension{}
	p := plog.NewLogs()
	p.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetStr("NOT A MAP")
	_, err := e.MarshalLogs(p)
	assert.ErrorContains(t, err, "Marshal: Expected 'Map' found 'Str'")
}

func TestInvalidUnmarshal(t *testing.T) {
	e := &jsonExtension{}
	_, err := e.UnmarshalLogs([]byte("NOT A JSON"))
	assert.ErrorContains(t, err, "ReadMapCB: expect { or n, but found N")
}
