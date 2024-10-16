// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package transformer // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor/internal/transformer"

import (
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor/internal/migrate"
)

// SpanEventConditionalAttributes is an transformer that powers the [Span Event's rename_attributes] change.
// [Span Event's rename_attributes]: https://opentelemetry.io/docs/specs/otel/schemas/file_format_v1.1.0/#rename_attributes-transformation-1
type SpanEventConditionalAttributes struct {
	MultiConditionalAttributeSet migrate.MultiConditionalAttributeSet
}

func (o SpanEventConditionalAttributes) IsMigrator() {}

func (o SpanEventConditionalAttributes) Do(ss migrate.StateSelector, span ptrace.Span) error {
	for e := 0; e < span.Events().Len(); e++ {
		event := span.Events().At(e)
		if err := o.MultiConditionalAttributeSet.Do(ss, event.Attributes(),
			map[string]string{
				"event.name": event.Name(),
				"span.name":  span.Name(),
			}); err != nil {
			return err
		}
	}
	return nil
}
