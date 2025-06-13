// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package add // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/transformer/add"

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/expr-lang/expr/vm"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

// Transformer is an operator that adds a string value or an expression value
type Transformer struct {
	helper.TransformerOperator

	Field   entry.Field
	Value   any
	program *vm.Program
}

func (t *Transformer) ProcessBatch(ctx context.Context, entries []*entry.Entry) error {
	return t.ProcessBatchWith(ctx, entries, t.Process)
}

// Process will process an entry with a add transformation.
func (t *Transformer) Process(ctx context.Context, entry *entry.Entry) error {
	return t.ProcessWith(ctx, entry, t.Transform)
}

// Transform will apply the add operations to an entry
func (t *Transformer) Transform(e *entry.Entry) error {
	if t.Value != nil {
		return e.Set(t.Field, t.Value)
	}
	if t.program != nil {
		env := helper.GetExprEnv(e)
		defer helper.PutExprEnv(env)

		result, err := vm.Run(t.program, env)
		if err != nil {
			return fmt.Errorf("evaluate value_expr: %w", err)
		}
		return e.Set(t.Field, result)
	}
	return errors.New("add: missing required field 'value'")
}

func isExpr(str string) bool {
	return strings.HasPrefix(str, "EXPR(") && strings.HasSuffix(str, ")")
}
