// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package hotreloadprocessor

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestFileWatcher(t *testing.T) {
	dir := t.TempDir()

	done := make(chan bool)
	watchedFiles := []string{}
	watcher, err := NewFileWatcher(zap.NewNop(), dir, func(filePath string) error {
		watchedFiles = append(watchedFiles, filePath)
		done <- true
		return nil
	})
	require.NoError(t, err)

	err = watcher.Start(context.Background())
	require.NoError(t, err)

	filePath := filepath.Join(dir, "config.yaml")
	os.WriteFile(filePath, []byte("test"), 0644)

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("timeout: file not watched")
	}

	watcher.Stop(context.Background())

	require.Equal(t, []string{filePath}, watchedFiles)
}
