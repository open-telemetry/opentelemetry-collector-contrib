// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fileconsumer

import (
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
	return buildTestManager(t, cfg, sink, nil, opts...)
}

// testManagerKeepFilesOpen builds a manager whose tracker keeps files open between poll
// cycles regardless of platform or feature-gate state. This lets the keep-open behavior
// be exercised in a parallel-safe way, without mutating the global feature-gate registry.
func testManagerKeepFilesOpen(t *testing.T, cfg *Config, opts ...Option) (*Manager, *emittest.Sink) {
	sink := emittest.NewSink()
	keepOpen := true
	return buildTestManager(t, cfg, sink, &keepOpen, opts...), sink
}

// testManagerKeepFilesClosed builds a manager whose tracker closes files immediately
// after each poll (the legacy Windows behavior), regardless of platform or gate state.
func testManagerKeepFilesClosed(t *testing.T, cfg *Config, opts ...Option) (*Manager, *emittest.Sink) {
	sink := emittest.NewSink()
	keepOpen := false
	return buildTestManager(t, cfg, sink, &keepOpen, opts...), sink
}

// buildTestManager builds a Manager and wires up its tracker for direct poll() calls in
// tests. If keepFilesOpenOverride is non-nil, it fixes the tracker's keep-open behavior
// for this instance (and, for tests that call Start(), survives tracker re-instantiation);
// otherwise the build-time default from keepFilesOpenBetweenPolls is used.
func buildTestManager(t *testing.T, cfg *Config, sink *emittest.Sink, keepFilesOpenOverride *bool, opts ...Option) *Manager {
	set := componenttest.NewNopTelemetrySettings()
	input, err := cfg.Build(set, sink.Callback, opts...)
	require.NoError(t, err)
	if keepFilesOpenOverride != nil {
		input.keepFilesOpen = *keepFilesOpenOverride
	}
	input.tracker = tracker.NewFileTracker(t.Context(), set, cfg.MaxBatches, cfg.PollsToArchive, testutil.NewUnscopedMockPersister(), input.keepFilesOpen)
	t.Cleanup(func() { input.tracker.ClosePreviousFiles() })
	return input
}
