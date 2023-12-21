// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tracker

import (
	"fmt"
	"os"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/filetest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/reader"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/testutil"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/encoding"
)

func TestTracker(t *testing.T) {
	var readerFactory reader.Factory = reader.Factory{
		SugaredLogger:   testutil.Logger(t),
		FingerprintSize: 1024,
		Encoding:        encoding.Nop,
	}

	tempDir := t.TempDir()
	tracker := New(testutil.Logger(t), 1024, readerFactory)
	temps := make([]*os.File, 0, 10)
	for i := 0; i < 10; i++ {
		temps = append(temps, filetest.OpenTemp(t, tempDir))
	}
	// Write one log to each file
	for i, temp := range temps {
		message := fmt.Sprintf("file %d: %s", i, "log")
		_, err := temp.WriteString(message + "\n")
		require.NoError(t, err)
	}

	// read files
	for _, temp := range temps {
		tracker.ReadFile(temp.Name())
	}

	require.Equal(t, tracker.activeFiles.Len(), 10)

	tracker.PostConsume()

	require.Equal(t, tracker.activeFiles.Len(), 0)
	require.Equal(t, tracker.openFiles.Len(), 10)
	require.Equal(t, tracker.closedFiles.Len(), 0)

	tracker.PostConsume()

	require.Equal(t, tracker.activeFiles.Len(), 0)
	require.Equal(t, tracker.openFiles.Len(), 0)
	require.Equal(t, tracker.closedFiles.Len(), 10)

	tracker.PostConsume()
	require.Equal(t, tracker.closedFiles.Len(), 10)

	tracker.MovingAverageMatches = 2

	tracker.PostConsume()
	require.Equal(t, tracker.closedFiles.Len(), 8)
}
