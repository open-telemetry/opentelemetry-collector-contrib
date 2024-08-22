// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package archive // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/archive"

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/reader"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
)

type defaultArchive struct {
}

func NewDefaultArchive() Archive {
	return &defaultArchive{}
}

func (a *defaultArchive) SetStorageClient(_ operator.Persister) {
}

func (a *defaultArchive) Match(unmatchedFiles []*FileRecord) ([]*FileRecord, error) {
	// Default archiving returns the files as it is
	return unmatchedFiles, nil
}

func (a *defaultArchive) Write(_ []*reader.Metadata) error {
	// discard the old offsets by default
	return nil
}
