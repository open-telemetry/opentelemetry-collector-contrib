// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package unquote // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/transformer/unquote"

import (
	"context"
	"fmt"
	"strconv"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

// Transformer is an operator that unquotes a string.
type Transformer struct {
	helper.TransformerOperator
	field entry.Field
}

// Process will unquote a string
func (t *Transformer) Process(ctx context.Context, entry *entry.Entry) error {
	return t.ProcessWith(ctx, entry, t.unquote)
}

// unquote will unquote a string
func (t *Transformer) unquote(e *entry.Entry) error {
	value, ok := t.field.Get(e)
	if !ok {
		return nil
	}

	switch v := value.(type) {
	case string:
		s, err := strconv.Unquote(v)
		if err != nil {
			return err
		}
		return t.field.Set(e, s)
	default:
		return fmt.Errorf("type %T cannot be unquoted", value)
	}
}
