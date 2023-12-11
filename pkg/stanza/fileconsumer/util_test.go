// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fileconsumer

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/emittest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/testutil"
)

func testManager(t *testing.T, cfg *Config) (*Manager, *emittest.Sink) {
	sink := emittest.NewSink()
	return testManagerWithSink(t, cfg, sink), sink
}

func testManagerWithSink(t *testing.T, cfg *Config, sink *emittest.Sink) *Manager {
	input, err := cfg.Build(testutil.Logger(t), sink.Callback)
	require.NoError(t, err)
	t.Cleanup(func() { input.closePreviousFiles() })
	return input
}
