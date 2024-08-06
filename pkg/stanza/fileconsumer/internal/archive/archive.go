// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package archive

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/fileset"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/fingerprint"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/reader"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
)

const knownFilesKeyPrefix = "knownFiles"

type Archive interface {
	SetStorageClient(persister operator.Persister)
	Match(fp *fingerprint.Fingerprint) *reader.Metadata
}

type archive struct {
	persister      operator.Persister
	pollsToArchive int
	fileset        *fileset.Fileset[*reader.Metadata]
}

func NewArchive(pollsToArchive int) *archive {
	return &archive{pollsToArchive: pollsToArchive}
}

func (a *archive) SetStorageClient(persister operator.Persister) {
	a.persister = persister
}

func (a *archive) Match(fp *fingerprint.Fingerprint) *reader.Metadata {
	// TODO:
	// 		Add logic to go through the storage and return a match.
	//		Also update the storage if match found.
	return nil
}
