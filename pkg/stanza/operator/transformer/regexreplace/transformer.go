// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package regexreplace // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/transformer/regexreplace"

import (
	"context"
	"fmt"
	"regexp"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

// Transformer is an operator that performs a regex-replace on a string field.
type Transformer struct {
	helper.TransformerOperator
	field       entry.Field
	regexp      *regexp.Regexp
	replaceWith string
}

func (t *Transformer) ProcessBatch(ctx context.Context, entries []*entry.Entry) error {
	return t.ProcessBatchWith(ctx, entries, t.process)
}

func (t *Transformer) process(_ context.Context, e *entry.Entry) error {
	value, ok := t.field.Get(e)
	if !ok {
		return nil
	}

	switch v := value.(type) {
	case string:
		s := t.regexp.ReplaceAllString(v, t.replaceWith)
		return t.field.Set(e, s)
	default:
		return fmt.Errorf("type %T cannot be handled", value)
	}
}
