// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fileexporter

import (
	"errors"
	"path/filepath"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

type flushTestWriter struct {
	err   error
	calls int
}

func (*flushTestWriter) Write(p []byte) (int, error) {
	return len(p), nil
}

func (*flushTestWriter) Close() error {
	return nil
}

func (w *flushTestWriter) flush() error {
	w.calls++
	return w.err
}

func newFlushTestWriter(t *testing.T, grouped bool, logger *zap.Logger, flushErr error) (*fileWriter, *flushTestWriter, FileExporter) {
	t.Helper()

	dir := t.TempDir()
	cfg := &Config{
		Path:       filepath.Join(dir, "out.json"),
		FormatType: formatTypeJSON,
	}
	if grouped {
		cfg.Path = filepath.Join(dir, "*.json")
		cfg.GroupBy = &GroupBy{
			Enabled:           true,
			ResourceAttribute: "service.name",
			MaxOpenFiles:      10,
		}
	}

	e := newFileExporter(cfg, logger)
	require.NoError(t, e.Start(t.Context(), componenttest.NewNopHost()))
	t.Cleanup(func() {
		require.NoError(t, e.Shutdown(t.Context()))
	})

	var writer *fileWriter
	if grouped {
		var err error
		writer, err = e.(*groupingFileExporter).getWriter("test")
		require.NoError(t, err)
		require.NoError(t, writer.release())
	} else {
		writer = e.(*fileExporter).writer
	}

	require.NoError(t, writer.file.Close())
	sink := &flushTestWriter{err: flushErr}
	writer.file = sink
	writer.flushInterval = time.Second
	writer.start()

	return writer, sink, e
}

func TestPeriodicFlushLogsFirstErrorOnly(t *testing.T) {
	for _, grouped := range []bool{false, true} {
		name := "single"
		if grouped {
			name = "grouped"
		}

		t.Run(name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				core, logs := observer.New(zap.ErrorLevel)
				firstErr := errors.New("disk full")
				writer, sink, _ := newFlushTestWriter(t, grouped, zap.New(core), firstErr)

				time.Sleep(time.Second)
				synctest.Wait()

				require.Equal(t, 1, logs.Len())
				require.Equal(t, zap.ErrorLevel, logs.All()[0].Level)
				require.Equal(t, writer.path, logs.All()[0].ContextMap()["path"])
				require.Equal(t, firstErr.Error(), logs.All()[0].ContextMap()["error"])

				time.Sleep(3 * time.Second)
				synctest.Wait()

				require.Equal(t, 4, sink.calls)
				require.Equal(t, 1, logs.Len(), "later flush failures must not log again")
			})
		})
	}
}

func TestPeriodicFlushSuccessDoesNotLog(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		core, logs := observer.New(zap.ErrorLevel)
		_, sink, _ := newFlushTestWriter(t, false, zap.New(core), nil)

		time.Sleep(3 * time.Second)
		synctest.Wait()

		require.Equal(t, 3, sink.calls)
		require.Empty(t, logs.All())
	})
}

func TestPeriodicFlushErrorsAreIndependent(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		core, logs := observer.New(zap.ErrorLevel)
		first, firstSink, e := newFlushTestWriter(t, true, zap.New(core), errors.New("first file failed"))
		second, err := e.(*groupingFileExporter).getWriter("second")
		require.NoError(t, err)
		require.NoError(t, second.release())
		require.NoError(t, second.file.Close())

		secondSink := &flushTestWriter{err: errors.New("second file failed")}
		second.file = secondSink
		second.flushInterval = time.Second
		second.start()

		time.Sleep(3 * time.Second)
		synctest.Wait()

		require.Equal(t, 3, firstSink.calls)
		require.Equal(t, 3, secondSink.calls)
		require.Equal(t, 2, logs.Len())
		require.Equal(t, 1, logs.FilterField(zap.String("path", first.path)).Len())
		require.Equal(t, 1, logs.FilterField(zap.String("path", second.path)).Len())
	})
}
