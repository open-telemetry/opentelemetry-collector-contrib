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
	write := func(_ context.Context, ent *entry.Entry) error {
		filteredEntries = append(filteredEntries, ent)
		return nil
	}
	var errs []error
	for _, ent := range entries {
		skip, err := t.Skip(ctx, ent)
		if err != nil {
			errs = append(errs, t.HandleEntryErrorWithWrite(ctx, ent, err, write))
			continue
		}
		if skip {
			// Write the entry without filtering
			_ = write(ctx, ent)
			continue
		}

		env := helper.GetExprEnv(ent)
		matches, err := vm.Run(t.expression, env)
		helper.PutExprEnv(env)

		if err != nil {
			t.Logger().Error("Running expressing returned an error", zap.Error(err))
			_ = write(ctx, ent)
			continue
		}

		filtered, ok := matches.(bool)
		if !ok {
			t.Logger().Error("Expression did not compile as a boolean")
			_ = write(ctx, ent)
			continue
		}

		if !filtered {
			_ = write(ctx, ent)
			continue
		}

		i, err := randInt(rand.Reader, upperBound)
		if err != nil {
			errs = append(errs, err)
			continue
		}

		if i.Cmp(t.dropCutoff) >= 0 {
			_ = write(ctx, ent)
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
