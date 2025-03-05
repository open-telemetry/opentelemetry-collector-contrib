// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package stripansiescapecodes // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/transformer/stripansiescapecodes"

import (
	"context"
	"fmt"
	"regexp"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

var ansiCsiEscapeRegex = regexp.MustCompile(`\x1B\[[\x30-\x3F]*[\x20-\x2F]*[\x40-\x7E]`)

// Transformer is an operator that will remove ANSI Escape Codes from a string.
type Transformer struct {
	helper.TransformerOperator
	field entry.Field
}

// Process will remove ANSI Escape Codes from a string
func (t *Transformer) Process(ctx context.Context, entry *entry.Entry) error {
	return t.ProcessWith(ctx, entry, t.strip)
}

// strip will remove ANSI Escape Codes from a string
func (t *Transformer) strip(e *entry.Entry) error {
	value, ok := t.field.Get(e)
	if !ok {
		return nil
	}

	switch v := value.(type) {
	case string:
		s := ansiCsiEscapeRegex.ReplaceAllString(v, "")
		return t.field.Set(e, s)
	default:
		return fmt.Errorf("type %T cannot be stripped", value)
	}
}
