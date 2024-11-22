// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package severity // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/parser/severity"

import (
	"context"
	"errors"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

// Parser is an operator that parses severity from a field to an entry.
type Parser struct {
	helper.TransformerOperator
	helper.SeverityParser
}

func (p *Parser) ProcessBatch(ctx context.Context, entries []entry.Entry) error {
	var errs []error
	for i := range entries {
		errs = append(errs, p.Process(ctx, &entries[i]))
	}
	return errors.Join(errs...)
}

// Process will parse severity from an entry.
func (p *Parser) Process(ctx context.Context, entry *entry.Entry) error {
	return p.ProcessWith(ctx, entry, p.Parse)
}
