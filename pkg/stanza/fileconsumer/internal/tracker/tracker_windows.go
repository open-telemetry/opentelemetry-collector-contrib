// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package tracker // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/tracker"

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/fileset"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/reader"
)

// On windows, we close files immediately after reading because they cannot be moved while open.
func (t *fileTracker) EndConsume() (filesClosed int) {
	spare := t.previousPollFiles
	if spare != nil {
		spare.Reset()
	}
	t.previousPollFiles = t.currentPollFiles
	filesClosed = t.ClosePreviousFiles()
	if spare == nil {
		t.currentPollFiles = fileset.New[*reader.Reader](t.maxBatchFiles)
	} else {
		t.currentPollFiles = spare
	}

	t.unmatchedFiles = t.unmatchedFiles[:0]
	t.unmatchedFps = t.unmatchedFps[:0]
	return filesClosed
}
