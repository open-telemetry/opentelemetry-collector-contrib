// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package noop

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/testutil"
)

func TestBuildValid(t *testing.T) {
	cfg := NewConfigWithID("test")
	op, err := cfg.Build(testutil.Logger(t))
	require.NoError(t, err)
	require.IsType(t, &Transformer{}, op)
}

func TestBuildInvalid(t *testing.T) {
	cfg := NewConfigWithID("test")
	_, err := cfg.Build(nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "build context is missing a logger")
}
