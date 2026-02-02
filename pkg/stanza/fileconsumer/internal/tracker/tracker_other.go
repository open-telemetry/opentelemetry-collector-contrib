// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package tracker // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/tracker"

import (
	"os"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/fileset"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/fingerprint"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/reader"
)

// On non-windows platforms, we keep files open between poll cycles so that we can detect
// and read "lost" files, which have been moved out of the matching pattern.
func (t *fileTracker) EndConsume() (filesClosed int) {
	filesClosed = t.ClosePreviousFiles()

	// t.currentPollFiles -> t.previousPollFiles
	t.previousPollFiles = t.currentPollFiles
	t.currentPollFiles = fileset.New[*reader.Reader](t.maxBatchFiles)

	t.unmatchedFiles = make([]*os.File, 0)
	t.unmatchedFps = make([]*fingerprint.Fingerprint, 0)
	return filesClosed
}
