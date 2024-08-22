// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package archive // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/archive"

import (
	"os"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/fingerprint"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/reader"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
)

const _ = "knownFiles"

type Archive interface {
	SetStorageClient(operator.Persister)

	Match([]*FileRecord) ([]*FileRecord, error)
	Write([]*reader.Metadata) error
}

type FileRecord struct {
	File        *os.File
	Fingerprint *fingerprint.Fingerprint

	// Metadata is populated if a match is found in storage
	// For new files, Metadata would remain empty
	Metadata *reader.Metadata
}

func NewFileRecord(file *os.File, fp *fingerprint.Fingerprint) *FileRecord {
	return &FileRecord{
		File:        file,
		Fingerprint: fp,
	}
}

type archive struct {
	persister operator.Persister
}

func NewArchive() Archive {
	return &archive{}
}

func (a *archive) SetStorageClient(persister operator.Persister) {
	a.persister = persister
}

// The Match function processes unmatched files by performing the following steps
//  1. Access the storage key and retrieve the existing metadata.
//  2. Iterate through the unmatched files:
//     a. If a corresponding record is found in storage, update the record's Metadata with the retrieved metadata
//     b. If no matching record is found, skip to the next file.
//     c. Update the storage key with updated metadata
//  3. Unmatched files will be updated with the following details:
//     a. Files with a matching record will have their Metadata updated to the known metadata.
//     b. New files will have an empty Metadata record.
func (a *archive) Match(_ []*FileRecord) ([]*FileRecord, error) {
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
