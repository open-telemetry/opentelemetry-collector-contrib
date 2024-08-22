// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package archive // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/archive"

import (
	"os"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/fileset"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/fingerprint"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/reader"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
)

const _ = "knownFiles"

type Archive interface {
	SetStorageClient(operator.Persister)
	Match([]*ArchiveFileRecord) ([]*reader.Reader, error)
	Write([]*reader.Metadata) error
}

type ArchiveFileRecord struct {
	file *os.File
	fp   *fingerprint.Fingerprint
}

func NewArchiveRecord(file *os.File, fp *fingerprint.Fingerprint) *ArchiveFileRecord {
	return &ArchiveFileRecord{
		file: file,
		fp:   fp,
	}
}

type archive struct {
	persister      operator.Persister
	pollsToArchive int
	_              int
	_              *fileset.Fileset[*reader.Metadata]
	readerFactory  reader.Factory
}

func NewArchive(pollsToArchive int, readerFactory reader.Factory) Archive {
	return &archive{pollsToArchive: pollsToArchive, readerFactory: readerFactory}
}

func (a *archive) SetStorageClient(persister operator.Persister) {
	a.persister = persister
}

func (a *archive) Match(_ []*ArchiveFileRecord) ([]*reader.Reader, error) {
	// Arguments:
	//		unmatched files
	// Returns:
	//		readers created from old/new metadata
	// TODO:
	// 		Add logic to go through the storage and return a match.
	//		Also update the storage if match found.

	return nil, nil
}

func (a *archive) Write(_ []*reader.Metadata) error {
	// TODO:
	// 		Add logic to update the index.
	//	 	Handle rollover logic
	return nil
}
