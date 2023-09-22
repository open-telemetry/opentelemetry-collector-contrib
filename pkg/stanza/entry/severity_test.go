// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package entry

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStringer(t *testing.T) {
	require.Equal(t, "DEFAULT", Default.String())
	require.Equal(t, "TRACE", Trace.String())
	require.Equal(t, "TRACE2", Trace2.String())
	require.Equal(t, "TRACE3", Trace3.String())
	require.Equal(t, "TRACE4", Trace4.String())
	require.Equal(t, "DEBUG", Debug.String())
	require.Equal(t, "DEBUG2", Debug2.String())
	require.Equal(t, "DEBUG3", Debug3.String())
	require.Equal(t, "DEBUG4", Debug4.String())
	require.Equal(t, "INFO", Info.String())
	require.Equal(t, "INFO2", Info2.String())
	require.Equal(t, "INFO3", Info3.String())
	require.Equal(t, "INFO4", Info4.String())
	require.Equal(t, "WARN", Warn.String())
	require.Equal(t, "WARN2", Warn2.String())
	require.Equal(t, "WARN3", Warn3.String())
	require.Equal(t, "WARN4", Warn4.String())
	require.Equal(t, "ERROR", Error.String())
	require.Equal(t, "ERROR2", Error2.String())
	require.Equal(t, "ERROR3", Error3.String())
	require.Equal(t, "ERROR4", Error4.String())
	require.Equal(t, "FATAL", Fatal.String())
	require.Equal(t, "FATAL2", Fatal2.String())
	require.Equal(t, "FATAL3", Fatal3.String())
	require.Equal(t, "FATAL4", Fatal4.String())
}
