// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package unmarshaler

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/encoding/japanese"
)

func TestNewTextLogsUnmarshaler(t *testing.T) {
	u, err := NewTextLogsUnmarshaler("utf8")
	require.NoError(t, err)
	assert.NotNil(t, u)

	u, err = NewTextLogsUnmarshaler("invalid_encoding")
	require.EqualError(t, err, "unsupported encoding 'invalid_encoding'")
	assert.Nil(t, u)
}

func TestTextLogsUnmarshaler(t *testing.T) {
	inputUTF8 := "こんにちは、世界"
	encoder := japanese.EUCJP.NewEncoder()
	eucjpEncoded, err := encoder.String(inputUTF8)
	require.NoError(t, err)

	u, err := NewTextLogsUnmarshaler("euc-jp")
	require.NoError(t, err)
	logs, err := u.UnmarshalLogs([]byte(eucjpEncoded))
	require.NoError(t, err)

	require.Equal(t, 1, logs.LogRecordCount())
	body := logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Body()
	assert.Equal(t, inputUTF8, body.Str())
}
