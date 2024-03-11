// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
package assignkeys // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/transformer/assignkeys"

import (
	"context"
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

// Process will process an entry with AssignKeys transformation.
func (p *Transformer) Process(ctx context.Context, entry *entry.Entry) error {
	return p.ProcessWith(ctx, entry, p.Transform)
}

// Transform will apply AssignKeys to an entry
func (p *Transformer) Transform(entry *entry.Entry) error {
	inputListInterface, ok := entry.Get(p.Field)
	if !ok {
		// The field doesn't exist, so ignore it
		return fmt.Errorf("apply assign_keys: field %s does not exist on entry", p.Field)
	}

	inputList, ok := inputListInterface.([]any)
	if !ok {
		return fmt.Errorf("apply assign_keys: couldn't convert field %s to []any", p.Field)
	}
	if len(inputList) != len(p.Keys) {
		return fmt.Errorf("apply assign_keys: field %s contains %d values while expected keys are %s contain %d keys", p.Field, len(inputList), p.Keys, len(p.Keys))
	}

	assignedMap := p.AssignKeys(p.Keys, inputList)

	err := entry.Set(p.Field, assignedMap)
	if err != nil {
		return err
	}
	return nil
}

func (p *Transformer) AssignKeys(keys []string, values []any) map[string]any {
	outputMap := make(map[string]any, len(keys))
	for i, key := range keys {
		outputMap[key] = values[i]
	}

	return outputMap
}
