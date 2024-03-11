// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package retain // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/transformer/retain"

import (
	"context"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

// Transformer keeps the given fields and deletes the rest.
type Transformer struct {
	helper.TransformerOperator
	Fields             []entry.Field
	AllBodyFields      bool
	AllAttributeFields bool
	AllResourceFields  bool
}

// Process will process an entry with a retain transformation.
func (p *Transformer) Process(ctx context.Context, entry *entry.Entry) error {
	return p.ProcessWith(ctx, entry, p.Transform)
}

// Transform will apply the retain operation to an entry
func (p *Transformer) Transform(e *entry.Entry) error {
	newEntry := entry.New()
	newEntry.ObservedTimestamp = e.ObservedTimestamp
	newEntry.Timestamp = e.Timestamp

	if !p.AllResourceFields {
		newEntry.Resource = e.Resource
	}
	if !p.AllAttributeFields {
		newEntry.Attributes = e.Attributes
	}
	if !p.AllBodyFields {
		newEntry.Body = e.Body
	}

	for _, field := range p.Fields {
		val, ok := e.Get(field)
		if !ok {
			continue
		}
		err := newEntry.Set(field, val)
		if err != nil {
			return err
		}
	}

	*e = *newEntry
	return nil
}
