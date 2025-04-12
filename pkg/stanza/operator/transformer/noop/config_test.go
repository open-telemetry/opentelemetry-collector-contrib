// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package noop

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
)

func TestBuildValid(t *testing.T) {
	cfg := NewConfigWithID("test")
	set := componenttest.NewNopTelemetrySettings()
	op, err := cfg.Build(set)
	require.NoError(t, err)
	require.IsType(t, &Transformer{}, op)
}

func TestBuildInvalid(t *testing.T) {
	cfg := NewConfigWithID("test")
	set := componenttest.NewNopTelemetrySettings()
	set.Logger = nil
	_, err := cfg.Build(set)
	require.ErrorContains(t, err, "build context is missing a logger")
}
