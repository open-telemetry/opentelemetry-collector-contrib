// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package jsonlogencodingextension

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/plog"
)

func TestMarshalUnmarshal(t *testing.T) {
	t.Parallel()
	e := &jsonLogExtension{
		config: &Config{
			Mode: JSONEncodingModeBody,
		},
	}
	json := `[{"example":"example valid json to test that the unmarshaler is correctly returning a plog value"}]`
	ld, err := e.UnmarshalLogs([]byte(json))
	assert.NoError(t, err)
	assert.Equal(t, 1, ld.LogRecordCount())

	buf, err := e.MarshalLogs(ld)
	assert.NoError(t, err)
	assert.NotEmpty(t, buf)
	assert.JSONEq(t, json, string(buf))
}

func TestInvalidMarshal(t *testing.T) {
	e := &jsonLogExtension{
		config: &Config{
			Mode: JSONEncodingModeBody,
		},
	}
	p := plog.NewLogs()
	p.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetStr("NOT A MAP")
	_, err := e.MarshalLogs(p)
	assert.ErrorContains(t, err, "marshal: expected 'Map' found 'Str'")
}

func TestInvalidUnmarshal(t *testing.T) {
	e := &jsonLogExtension{
		config: &Config{
			Mode: JSONEncodingModeBody,
		},
	}
	_, err := e.UnmarshalLogs([]byte("NOT A JSON"))
	assert.ErrorContains(t, err, "json: slice unexpected end of JSON input")
}

func TestPrettyLogProcessor(t *testing.T) {
	j := &jsonLogExtension{
		config: &Config{
			Mode: JSONEncodingModeBodyWithInlineAttributes,
		},
	}
	lp, err := j.logProcessor(sampleLog())
	assert.NoError(t, err)
	assert.NotNil(t, lp)
	assert.JSONEq(t, `[{"body":{"log":"test"},"logAttributes":{"foo":"bar"},"resourceAttributes":{"test":"logs-test"}},{"body":"log testing","resourceAttributes":{"test":"logs-test"}}]`, string(lp))
}

func sampleLog() plog.Logs {
	l := plog.NewLogs()
	rl := l.ResourceLogs().AppendEmpty()
	rl.Resource().Attributes().PutStr("test", "logs-test")
	rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetEmptyMap().PutStr("log", "test")
	rl.ScopeLogs().At(0).LogRecords().At(0).Attributes().PutStr("foo", "bar")
	rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetStr("log testing")
	return l
}
