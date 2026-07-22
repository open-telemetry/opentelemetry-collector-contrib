// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package translation // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor/internal/translation"

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor/internal/changelist"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor/internal/migrate"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor/internal/transformer"
)

// NewRevisionFromV2Resolved builds a RevisionV1 from a v2 resolved registry.
//
// v2 has no per-section rename scoping (the catalog is flat): every
// `deprecated.reason: renamed` entry in attribute_catalog produces a single
// rename mapping (old key -> renamed_to) that is applied to all signals via
// the `all` ChangeList. Per-signal renames for metric/span/event names are
// extracted from registry.{metrics,spans,events}.
func NewRevisionFromV2Resolved(ver *Version, resolved *V2Resolved, copyAttributes bool) *RevisionV1 {
	attrRenames := make(map[string]string)
	for _, attr := range resolved.AttributeCatalog {
		if attr.Deprecated == nil || attr.Deprecated.Reason != DeprecatedReasonRenamed {
			continue
		}
		if attr.Deprecated.RenamedTo == "" || attr.Key == "" {
			continue
		}
		attrRenames[attr.Key] = attr.Deprecated.RenamedTo
	}

	metricRenames := extractSignalRenames(resolved.Registry.Metrics)
	spanRenames := extractSignalRenames(resolved.Registry.Spans)
	eventRenames := extractSignalRenames(resolved.Registry.Events)

	return &RevisionV1{
		ver:        ver,
		all:        newV2AllChangeList(attrRenames, copyAttributes),
		resources:  emptyChangeList(),
		spans:      newV2SpanChangeList(spanRenames),
		spanEvents: newV2SpanEventChangeList(eventRenames),
		metrics:    newV2MetricChangeList(metricRenames),
		logs:       emptyChangeList(),
	}
}

func extractSignalRenames(signals []V2Signal) map[string]string {
	out := make(map[string]string)
	for _, s := range signals {
		if s.Deprecated == nil || s.Deprecated.Reason != DeprecatedReasonRenamed {
			continue
		}
		if s.Deprecated.RenamedTo == "" || s.Name == "" {
			continue
		}
		out[s.Name] = s.Deprecated.RenamedTo
	}
	return out
}

func newV2AllChangeList(renames map[string]string, copyAttributes bool) *changelist.ChangeList {
	if len(renames) == 0 {
		return emptyChangeList()
	}
	attributeChangeSet := migrate.NewAttributeChangeSet(renames, copyAttributes)
	allTransformer := transformer.NewAllAttributesTransformer(attributeChangeSet)
	return &changelist.ChangeList{Migrators: []migrate.Migrator{allTransformer}}
}

func newV2MetricChangeList(renames map[string]string) *changelist.ChangeList {
	if len(renames) == 0 {
		return emptyChangeList()
	}
	signalNameChange := transformer.MetricSignalNameChange{SignalNameChange: migrate.NewSignalNameChange(renames)}
	return &changelist.ChangeList{Migrators: []migrate.Migrator{signalNameChange}}
}

func newV2SpanChangeList(renames map[string]string) *changelist.ChangeList {
	if len(renames) == 0 {
		return emptyChangeList()
	}
	signalNameChange := transformer.SpanSignalNameChange{SignalNameChange: migrate.NewSignalNameChange(renames)}
	return &changelist.ChangeList{Migrators: []migrate.Migrator{signalNameChange}}
}

func newV2SpanEventChangeList(renames map[string]string) *changelist.ChangeList {
	if len(renames) == 0 {
		return emptyChangeList()
	}
	signalNameChange := transformer.SpanEventSignalNameChange{SignalNameChange: migrate.NewSignalNameChange(renames)}
	return &changelist.ChangeList{Migrators: []migrate.Migrator{signalNameChange}}
}

func emptyChangeList() *changelist.ChangeList {
	return &changelist.ChangeList{Migrators: nil}
}
