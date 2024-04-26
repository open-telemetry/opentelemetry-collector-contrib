// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package jsonlogencodingextension

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/plog"
)

func TestExtension_Start_Shutdown(t *testing.T) {
	j := &jsonLogExtension{}
	err := j.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)
	err = j.Shutdown(context.Background())
	require.NoError(t, err)
}

// write tests for the LogProcessor function
func TestExtension_LogProcessor(t *testing.T) {
	j := &jsonLogExtension{}
	lp, err := j.LogProcessor(sampleLog())
	require.NoError(t, err)
	require.NotNil(t, lp)
	require.Equal(t, string(lp), `[{"body":"{\"log\": \"test\"}","resource":{"test":"logs-test"}},{"body":"log testing","resource":{"test":"logs-test"}}]`)
}

func sampleLog() plog.Logs {
	l := plog.NewLogs()
	rl := l.ResourceLogs().AppendEmpty()
	rl.Resource().Attributes().PutStr("test", "logs-test")
	rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetStr(`{"log": "test"}`)
	rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetStr("log testing")
	return l
}
