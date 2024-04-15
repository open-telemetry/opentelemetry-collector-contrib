// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package scope // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/parser/scope"

import (
	"context"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

// Parser is an operator that parses logger name from a field to an entry.
type Parser struct {
	helper.TransformerOperator
	helper.ScopeNameParser
}

// Process will parse logger name from an entry.
func (p *Parser) Process(ctx context.Context, entry *entry.Entry) error {
	return p.ProcessWith(ctx, entry, p.Parse)
}
