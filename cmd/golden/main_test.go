// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package main // import "github.com/open-telemetry/opentelemetry-collector-contrib/cmd/golden"

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTimeout(t *testing.T) {
	err := run([]string{"--timeout", "1s", "--write-expected", "--expected", "foo.yaml"})
	require.EqualError(t, err, "timeout waiting for data")
}

func TestMissingArgument(t *testing.T) {
	err := run([]string{"--timeout", "--write-expected", "--expected", "foo.yaml"})
	require.EqualError(t, err, `time: invalid duration "--write-expected"`)
}

func TestMissingArgumentLast(t *testing.T) {
	err := run([]string{"--write-expected", "--expected", "foo.yaml", "--timeout"})
	require.EqualError(t, err, "--timeout requires an argument")
}
