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

func (t *Transformer) ProcessBatch(ctx context.Context, entries []*entry.Entry) error {
	return t.ProcessBatchWith(ctx, entries, t.Process)
}

// Process will process an entry with a remove transformation.
func (t *Transformer) Process(ctx context.Context, entry *entry.Entry) error {
	return t.ProcessWith(ctx, entry, t.Transform)
}

// Transform will apply the remove operation to an entry
func (t *Transformer) Transform(entry *entry.Entry) error {
	if t.Field.allAttributes {
		entry.Attributes = nil
		return nil
	}

	if t.Field.allResource {
		entry.Resource = nil
		return nil
	}

	_, exist := entry.Delete(t.Field.Field)
	if !exist {
		return fmt.Errorf("remove: field does not exist: %s", t.Field.String())
	}
	return nil
}
