// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package testbed // import "github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAgentExeOption(t *testing.T) {
	path := "bin/otelcontribcol"
	col := NewChildProcessCollector(WithAgentExePath(path))

	cpc, ok := col.(*childProcessCollector)

	require.True(t, ok)
	require.Equal(t, path, cpc.agentExePath)
}
