// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package move // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/transformer/move"

import (
	"context"
	"fmt"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

// Transformer is an operator that moves a field's value to a new field
type Transformer struct {
	helper.TransformerOperator
	From entry.Field
	To   entry.Field
}

// Process will process an entry with a move transformation.
func (p *Transformer) Process(ctx context.Context, entry *entry.Entry) error {
	return p.ProcessWith(ctx, entry, p.Transform)
}

// Transform will apply the move operation to an entry
func (p *Transformer) Transform(e *entry.Entry) error {
	val, exist := p.From.Delete(e)
	if !exist {
		return fmt.Errorf("move: field does not exist: %s", p.From.String())
	}
	return p.To.Set(e, val)
}
