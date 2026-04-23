// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package filter // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/transformer/filter"

import (
	"context"
	"crypto/rand"
	"errors"
	"math/big"

	"github.com/expr-lang/expr/vm"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

// Transformer is an operator that filters entries based on matching expressions
type Transformer struct {
	helper.TransformerOperator
	expression *vm.Program
	dropCutoff *big.Int // [0..1000)
}

func (t *Transformer) ProcessBatch(ctx context.Context, entries []*entry.Entry) error {
	filteredEntries := make([]*entry.Entry, 0, len(entries))
	var errs []error
	for _, ent := range entries {
		env := helper.GetExprEnv(ent)
		matches, err := vm.Run(t.expression, env)
		helper.PutExprEnv(env)

		if err != nil {
			t.Logger().Error("Running expressing returned an error", zap.Error(err))
			continue
		}

		filtered, ok := matches.(bool)
		if !ok {
			t.Logger().Error("Expression did not compile as a boolean")
			continue
		}

		if !filtered {
			filteredEntries = append(filteredEntries, ent)
			continue
		}

		i, err := randInt(rand.Reader, upperBound)
		if err != nil {
			errs = append(errs, err)
			continue
		}

		if i.Cmp(t.dropCutoff) >= 0 {
			filteredEntries = append(filteredEntries, ent)
		}
	}

	errs = append(errs, t.WriteBatch(ctx, filteredEntries))
	return errors.Join(errs...)
}

// Process will drop incoming entries that match the filter expression
func (t *Transformer) Process(ctx context.Context, entry *entry.Entry) error {
	env := helper.GetExprEnv(entry)
	defer helper.PutExprEnv(env)

	matches, err := vm.Run(t.expression, env)
	if err != nil {
		t.Logger().Error("Running expressing returned an error", zap.Error(err))
		return nil
	}

	filtered, ok := matches.(bool)
	if !ok {
		t.Logger().Error("Expression did not compile as a boolean")
		return nil
	}

	if !filtered {
		return t.Write(ctx, entry)
	}

	i, err := randInt(rand.Reader, upperBound)
	if err != nil {
		return err
	}

	if i.Cmp(t.dropCutoff) >= 0 {
		err := t.Write(ctx, entry)
		if err != nil {
			return err
		}
	}

	return nil
}
