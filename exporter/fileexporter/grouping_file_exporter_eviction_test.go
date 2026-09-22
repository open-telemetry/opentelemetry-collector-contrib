// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fileexporter

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.uber.org/zap"
)

func newEvictionTestConfig(tmpDir string) *Config {
	return &Config{
		FormatType: formatTypeJSON,
		Path:       tmpDir + "/*.log",
		// No rotation, so the buffered writer is used. This is the default
		// when rotation is not set.
		Rotation: nil,
		GroupBy: &GroupBy{
			Enabled:           true,
			ResourceAttribute: defaultResourceAttribute,
			MaxOpenFiles:      1,
		},
	}
}

// A writer evicted while an export is running must not lose that export's data.
func TestGroupingFileExporterPreservesInFlightWrite(t *testing.T) {
	tmpDir := t.TempDir()
	conf := newEvictionTestConfig(tmpDir)

	gfe, ok := newFileExporter(conf, zap.NewNop()).(*groupingFileExporter)
	require.True(t, ok)
	require.NoError(t, gfe.Start(t.Context(), componenttest.NewNopHost()))

	inExport := make(chan struct{})
	proceed := make(chan struct{})
	var once sync.Once

	// Pause the export to "one" so its writer can be evicted while the export is running.
	orig := gfe.newFileWriter
	gfe.newFileWriter = func(path string) (*fileWriter, error) {
		w, err := orig(path)
		if err != nil || !strings.HasSuffix(path, "one.log") {
			return w, err
		}
		export := w.exporter
		w.exporter = func(fw *fileWriter, buf []byte) error {
			once.Do(func() {
				close(inExport)
				<-proceed
			})
			return export(fw, buf)
		}
		return w, nil
	}

	payload := []byte(`{"resourceLogs":"one"}`)

	var wg sync.WaitGroup
	var inFlightErr error
	wg.Go(func() {
		inFlightErr = gfe.write(t.Context(), "one", payload)
	})

	<-inExport
	// MaxOpenFiles is 1, so this evicts "one".
	require.NoError(t, gfe.write(t.Context(), "two", []byte(`{"resourceLogs":"two"}`)))
	close(proceed)
	wg.Wait()

	require.NoError(t, gfe.Shutdown(t.Context()))

	require.NoError(t, inFlightErr)

	content, err := os.ReadFile(tmpDir + "/one.log")
	require.NoError(t, err)
	require.Contains(t, string(content), string(payload), "payload was lost after eviction")
}

// Many writers share a small cache. Run with -race to catch a close during export.
func TestGroupingFileExporterConcurrentEviction(t *testing.T) {
	tmpDir := t.TempDir()
	conf := newEvictionTestConfig(tmpDir)

	gfe, ok := newFileExporter(conf, zap.NewNop()).(*groupingFileExporter)
	require.True(t, ok)
	require.NoError(t, gfe.Start(t.Context(), componenttest.NewNopHost()))

	const (
		writers  = 8
		writes   = 200
		segments = 4
	)

	var wg sync.WaitGroup
	for worker := range writers {
		wg.Go(func() {
			for j := range writes {
				segment := fmt.Sprintf("segment-%d", j%segments)
				payload := fmt.Appendf(nil, `{"worker":%d,"seq":%d}`, worker, j)
				assert.NoError(t, gfe.write(t.Context(), segment, payload))
			}
		})
	}
	wg.Wait()

	require.NoError(t, gfe.Shutdown(t.Context()))
}
