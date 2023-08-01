// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fileconsumer // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer"

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/util"
)

// Deprecated: [v0.82.0] This will be removed in a future release, tentatively v0.84.0.
type FileAttributes struct {
	Name             string `json:"-"`
	Path             string `json:"-"`
	NameResolved     string `json:"-"`
	PathResolved     string `json:"-"`
	HeaderAttributes map[string]any
}

// HeaderAttributesCopy gives a copy of the HeaderAttributes, in order to restrict mutation of the HeaderAttributes.
//
// Deprecated: [v0.82.0] This will be removed in a future release, tentatively v0.84.0.
func (f *FileAttributes) HeaderAttributesCopy() map[string]any {
	return util.MapCopy(f.HeaderAttributes)
}
