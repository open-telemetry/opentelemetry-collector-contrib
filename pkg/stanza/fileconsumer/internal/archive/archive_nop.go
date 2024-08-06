// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package archive

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/fingerprint"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/reader"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
)

type nop_archive struct{}

func NewNopArchive() *nop_archive {
	return &nop_archive{}
}

func (a *nop_archive) SetStorageClient(_ operator.Persister) {
}

func (a *nop_archive) Match(_ *fingerprint.Fingerprint) *reader.Metadata {
	return nil
}
