// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package remove // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/transformer/remove"

import (
	"context"
	"fmt"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

// Transformer is an operator that deletes a field
type Transformer struct {
	helper.TransformerOperator
	Field rootableField
}

// Process will process an entry with a remove transformation.
func (p *Transformer) Process(ctx context.Context, entry *entry.Entry) error {
	return p.ProcessWith(ctx, entry, p.Transform)
}

// Transform will apply the remove operation to an entry
func (p *Transformer) Transform(entry *entry.Entry) error {
	if p.Field.allAttributes {
		entry.Attributes = nil
		return nil
	}

	if p.Field.allResource {
		entry.Resource = nil
		return nil
	}

	_, exist := entry.Delete(p.Field.Field)
	if !exist {
		return fmt.Errorf("remove: field does not exist: %s", p.Field.Field.String())
	}
	return nil
}
