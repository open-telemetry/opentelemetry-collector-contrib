// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package archive // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/archive"

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/fingerprint"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/reader"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
)

type nopArchive struct{}

func NewNopArchive() Archive {
	return &nopArchive{}
}

func (a *nopArchive) SetStorageClient(_ operator.Persister) {
}

func (a *nopArchive) Match([]*fingerprint.Fingerprint) ([]*reader.Metadata, []*fingerprint.Fingerprint, error) {
	return nil, nil, nil
}

func (a *nopArchive) Write(_ []*reader.Metadata) error {
	return nil
}
