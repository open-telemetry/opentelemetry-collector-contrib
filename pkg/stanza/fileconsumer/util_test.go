// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fileconsumer

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/emittest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/tracker"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/testutil"
)

func testManager(t *testing.T, cfg *Config, opts ...Option) (*Manager, *emittest.Sink) {
	sink := emittest.NewSink()
	return testManagerWithSink(t, cfg, sink, opts...), sink
}

func testManagerWithSink(t *testing.T, cfg *Config, sink *emittest.Sink, opts ...Option) *Manager {
	set := componenttest.NewNopTelemetrySettings()
	input, err := cfg.Build(set, sink.Callback, opts...)
	input.tracker = tracker.NewFileTracker(context.Background(), set, cfg.MaxBatches, cfg.PollsToArchive, testutil.NewUnscopedMockPersister())
	require.NoError(t, err)
	t.Cleanup(func() { input.tracker.ClosePreviousFiles() })
	return input
}
