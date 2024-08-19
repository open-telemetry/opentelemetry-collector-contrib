// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package archive // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/archive"

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/fileset"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/fingerprint"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/reader"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
)

const _ = "knownFiles"

type Archive interface {
	SetStorageClient(operator.Persister)
	Match(*fingerprint.Fingerprint) (*reader.Metadata, error)
	Write([]*reader.Metadata) error
}

type archive struct {
	persister      operator.Persister
	pollsToArchive int
	index          int
	_              *fileset.Fileset[*reader.Metadata]
}

func NewArchive(pollsToArchive int) Archive {
	return &archive{pollsToArchive: pollsToArchive}
}

func (a *archive) SetStorageClient(persister operator.Persister) {
	a.persister = persister
}

func (a *archive) Match(_ *fingerprint.Fingerprint) (*reader.Metadata, error) {
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
