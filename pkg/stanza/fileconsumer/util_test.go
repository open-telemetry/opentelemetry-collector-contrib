// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fileconsumer

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/emittest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/testutil"
)

func testManager(t *testing.T, cfg *Config, opts ...Option) (*Manager, *emittest.Sink) {
	sink := emittest.NewSink()
	return testManagerWithSink(t, cfg, sink, opts...), sink
}

func testManagerWithSink(t *testing.T, cfg *Config, sink *emittest.Sink, opts ...Option) *Manager {
	input, err := cfg.Build(testutil.Logger(t), sink.Callback, opts...)
	require.NoError(t, err)
	t.Cleanup(func() { input.tracker.ClosePreviousFiles() })
	return input
}
