// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package timeparser // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/parser/timeparser"

import (
	"context"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

// Parser is an operator that parses time from a field to an entry.
type Parser struct {
	helper.TransformerOperator
	helper.TimeParser
}

func (p *Parser) ProcessBatch(ctx context.Context, entries []*entry.Entry) error {
	return p.ProcessBatchWithTransform(ctx, entries, p.Parse)
}

// Process will parse time from an entry.
func (p *Parser) Process(ctx context.Context, entry *entry.Entry) error {
	return p.ProcessWith(ctx, entry, p.Parse)
}
