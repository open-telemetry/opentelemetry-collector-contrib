// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package severity // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/parser/severity"

import (
	"context"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

// Parser is an operator that parses severity from a field to an entry.
type Parser struct {
	helper.TransformerOperator
	helper.SeverityParser
}

func (p *Parser) ProcessBatch(ctx context.Context, entries []*entry.Entry) error {
	return p.ProcessBatchWith(ctx, entries, p.Process)
}

// Process will parse severity from an entry.
func (p *Parser) Process(ctx context.Context, entry *entry.Entry) error {
	return p.ProcessWith(ctx, entry, p.Parse)
}
