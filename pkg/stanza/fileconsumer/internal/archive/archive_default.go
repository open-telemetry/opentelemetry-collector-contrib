// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package archive // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/archive"

import (
	"errors"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/reader"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
)

type defaultArchive struct {
	readerFactory *reader.Factory
}

func NewDefaultArchive(readerFactory *reader.Factory) Archive {
	return &defaultArchive{readerFactory: readerFactory}
}

func (a *defaultArchive) SetStorageClient(_ operator.Persister) {
}

func (a *defaultArchive) Match(unmatchedFiles []*ArchiveFileRecord) ([]*reader.Reader, error) {
	// Default archiving mode creates new readers for unmatched files
	readers := make([]*reader.Reader, 0)
	var combinedError error
	for _, record := range unmatchedFiles {
		r, err := a.readerFactory.NewReader(record.file, record.fp)
		if err != nil {
			combinedError = errors.Join(combinedError, err)
		} else {
			readers = append(readers, r)
		}
	}
	return readers, combinedError
}

func (a *defaultArchive) Write(_ []*reader.Metadata) error {
	// discard the old offsets by default
	return nil
}
