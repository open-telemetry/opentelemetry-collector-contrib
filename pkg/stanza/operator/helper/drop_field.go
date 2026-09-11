// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package helper // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"

import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"

// DropFieldConfig provides configuration for dropping parsed fields from an entry.
type DropFieldConfig struct {
	DropField bool `mapstructure:"drop_field,omitempty"`
}

// Drop deletes the field from the entry if DropField is set to true.
func (c DropFieldConfig) Drop(ent *entry.Entry, field entry.Field) {
	if c.DropField {
		ent.Delete(field)
	}
}
