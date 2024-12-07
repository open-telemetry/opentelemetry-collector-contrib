// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package noop // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/transformer/noop"

import (
	"context"
	"errors"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

// Transformer is an operator that performs no operations on an entry.
type Transformer struct {
	helper.TransformerOperator
}

func (t *Transformer) ProcessBatch(ctx context.Context, entries []*entry.Entry) error {
	var errs []error
	for i := range entries {
		errs = append(errs, t.Process(ctx, entries[i]))
	}
	return errors.Join(errs...)
}

// Process will forward the entry to the next output without any alterations.
func (t *Transformer) Process(ctx context.Context, entry *entry.Entry) error {
	return t.Write(ctx, entry)
}
