// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package drop

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/testutil"
)

func TestBuildValid(t *testing.T) {
	cfg := NewConfig("test")
	op, err := cfg.Build(testutil.Logger(t))
	require.NoError(t, err)
	require.IsType(t, &Output{}, op)
}

func TestBuildIvalid(t *testing.T) {
	cfg := NewConfig("test")
	_, err := cfg.Build(nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "build context is missing a logger")
}

func TestProcess(t *testing.T) {
	cfg := NewConfig("test")
	op, err := cfg.Build(testutil.Logger(t))
	require.NoError(t, err)

	entry := entry.New()
	result := op.Process(context.Background(), entry)
	require.Nil(t, result)
}
