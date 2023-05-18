// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package entry

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStringer(t *testing.T) {
	require.Equal(t, "default", Default.String())
	require.Equal(t, "trace", Trace.String())
	require.Equal(t, "trace2", Trace2.String())
	require.Equal(t, "trace3", Trace3.String())
	require.Equal(t, "trace4", Trace4.String())
	require.Equal(t, "debug", Debug.String())
	require.Equal(t, "debug2", Debug2.String())
	require.Equal(t, "debug3", Debug3.String())
	require.Equal(t, "debug4", Debug4.String())
	require.Equal(t, "info", Info.String())
	require.Equal(t, "info2", Info2.String())
	require.Equal(t, "info3", Info3.String())
	require.Equal(t, "info4", Info4.String())
	require.Equal(t, "warn", Warn.String())
	require.Equal(t, "warn2", Warn2.String())
	require.Equal(t, "warn3", Warn3.String())
	require.Equal(t, "warn4", Warn4.String())
	require.Equal(t, "error", Error.String())
	require.Equal(t, "error2", Error2.String())
	require.Equal(t, "error3", Error3.String())
	require.Equal(t, "error4", Error4.String())
	require.Equal(t, "fatal", Fatal.String())
	require.Equal(t, "fatal2", Fatal2.String())
	require.Equal(t, "fatal3", Fatal3.String())
	require.Equal(t, "fatal4", Fatal4.String())
}
