// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package uri // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/parser/uri"

import (
	"context"
	"fmt"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/parseutils"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

// Parser is an operator that parses a uri.
type Parser struct {
	helper.ParserOperator
}

func (p *Parser) ProcessBatch(ctx context.Context, entries []*entry.Entry) error {
	return p.ProcessBatchWith(ctx, entries, p.parse)
}

// Process will parse an entry.
func (p *Parser) Process(ctx context.Context, entry *entry.Entry) error {
	return p.ProcessWith(ctx, entry, p.parse)
}

// parse will parse a uri from a field and attach it to an entry.
func (*Parser) parse(value any) (any, error) {
	switch m := value.(type) {
	case string:
		return parseutils.ParseURI(m, metadata.ParserURIEcscompliantFeatureGate.IsEnabled())
	default:
		return nil, fmt.Errorf("type '%T' cannot be parsed as URI", value)
	}
}
