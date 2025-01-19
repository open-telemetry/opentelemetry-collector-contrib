// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package flatten // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/transformer/flatten"

import (
	"context"
	"fmt"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/errors"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

// Transformer flattens an object in the entry field
type Transformer[T interface {
	entry.BodyField | entry.ResourceField | entry.AttributeField
	entry.FieldInterface
	Parent() T
	Child(string) T
}] struct {
	helper.TransformerOperator
	Field T
}

// Process will process an entry with a flatten transformation.
func (t *Transformer[T]) Process(ctx context.Context, entry *entry.Entry) error {
	return t.ProcessWith(ctx, entry, t.Transform)
}

// Transform will apply the flatten operation to an entry
func (t *Transformer[T]) Transform(entry *entry.Entry) error {
	parent := t.Field.Parent()
	val, ok := entry.Delete(t.Field)
	if !ok {
		// The field doesn't exist, so ignore it
		return fmt.Errorf("apply flatten: field %s does not exist on entry", t.Field)
	}

	valMap, ok := val.(map[string]any)
	if !ok {
		// The field we were asked to flatten was not a map, so put it back
		err := entry.Set(t.Field, val)
		if err != nil {
			return errors.Wrap(err, "reset non-map field")
		}
		return fmt.Errorf("apply flatten: field %s is not a map", t.Field)
	}

	for k, v := range valMap {
		err := entry.Set(parent.Child(k), v)
		if err != nil {
			return err
		}
	}
	return nil
}
