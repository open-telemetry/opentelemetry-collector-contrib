// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
package assignkeys // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/transformer/assignkeys"

import (
	"context"
	"errors"
	"fmt"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

// Transformer transforms a list in the entry field into a map. Each value is assigned a key from configuration keys
type Transformer struct {
	helper.TransformerOperator
	Field entry.Field
	Keys  []string
}

func (t *Transformer) ProcessBatch(ctx context.Context, entries []entry.Entry) error {
	var errs []error
	for i := range entries {
		errs = append(errs, t.Process(ctx, &entries[i]))
	}
	return errors.Join(errs...)
}

// Process will process an entry with AssignKeys transformation.
func (t *Transformer) Process(ctx context.Context, entry *entry.Entry) error {
	return t.ProcessWith(ctx, entry, t.Transform)
}

// Transform will apply AssignKeys to an entry
func (t *Transformer) Transform(entry *entry.Entry) error {
	inputListInterface, ok := entry.Get(t.Field)
	if !ok {
		// The field doesn't exist, so ignore it
		return fmt.Errorf("apply assign_keys: field %s does not exist on entry", t.Field)
	}

	inputList, ok := inputListInterface.([]any)
	if !ok {
		return fmt.Errorf("apply assign_keys: couldn't convert field %s to []any", t.Field)
	}
	if len(inputList) != len(t.Keys) {
		return fmt.Errorf("apply assign_keys: field %s contains %d values while expected keys are %s contain %d keys", t.Field, len(inputList), t.Keys, len(t.Keys))
	}

	assignedMap := t.AssignKeys(t.Keys, inputList)

	err := entry.Set(t.Field, assignedMap)
	if err != nil {
		return err
	}
	return nil
}

func (t *Transformer) AssignKeys(keys []string, values []any) map[string]any {
	outputMap := make(map[string]any, len(keys))
	for i, key := range keys {
		outputMap[key] = values[i]
	}

	return outputMap
}
