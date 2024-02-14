// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package testbed // import "github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"

import (
	"os"
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

func TestAgentEnvVarOption(t *testing.T) {
	col := NewChildProcessCollector(
		WithAgentExePath("not-real"),
		WithEnvVar("var-one", "var-one-value"),
		WithEnvVar("var-two", "var-two-value"),
		WithEnvVar("var-two", "actual-var-two-value"),
	)

	cpc, ok := col.(*childProcessCollector)
	require.True(t, ok)

	expeectedEnvVarMap := map[string]string{
		"var-one": "var-one-value",
		"var-two": "actual-var-two-value",
	}
	require.Equal(t, expeectedEnvVarMap, cpc.additionalEnv)

	// results from `not-real` not being found but contents unrelated to this test
	require.Error(t, col.Start(
		StartParams{
			CmdArgs:     []string{"--config", "to-prevent-builder"},
			LogFilePath: "/dev/null",
		},
	))
	expectedEnvVars := append(os.Environ(), "var-one=var-one-value", "var-two=actual-var-two-value")
	require.Equal(t, expectedEnvVars, cpc.cmd.Env)
}
