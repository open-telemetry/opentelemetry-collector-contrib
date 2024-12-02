// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package translation // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor/internal/translation"

import (
	"go.opentelemetry.io/otel/schema/v1.0/ast"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor/internal/changelist"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor/internal/migrate"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor/internal/transformer"
)

// RevisionV1 represents all changes that are to be applied to a signal at a given version.  V1 represents the fact
// that this struct only support the Schema Files version 1.0 - not 1.1 which contains split.
// todo(ankit) implement split and rest of otel schema
type RevisionV1 struct {
	ver        *Version
	all        *changelist.ChangeList
	resources  *changelist.ChangeList
	spans      *changelist.ChangeList
	spanEvents *changelist.ChangeList
	metrics    *changelist.ChangeList
	logs       *changelist.ChangeList
}

// NewRevision processes the VersionDef and assigns the version to this revision
// to allow sorting within a slice.
// Since VersionDef uses custom types for various definitions, it isn't possible
// to cast those values into the primitives so each has to be processed together.
// Generics would be handy here.
// todo(ankit) investigate using generics
func NewRevision(ver *Version, def ast.VersionDef) *RevisionV1 {
	// todo(ankit) change logs to be an ast.Attributes type so I dont have to change this
	var logChanges ast.Attributes
	for _, change := range def.Logs.Changes {
		//nolint:gosimple
		logChanges.Changes = append(logChanges.Changes, ast.AttributeChange{RenameAttributes: change.RenameAttributes})
	}
	return &RevisionV1{
		ver:        ver,
		all:        newAllChangeList(def.All),
		resources:  newResourceChangeList(def.Resources),
		spans:      newSpanChangeList(def.Spans),
		spanEvents: newSpanEventChangeList(def.SpanEvents),
		metrics:    newMetricChangeList(def.Metrics),
		logs:       newLogsChangelist(def.Logs),
	}
}

func (r RevisionV1) Version() *Version {
	return r.ver
}

func newAllChangeList(all ast.Attributes) *changelist.ChangeList {
	values := make([]migrate.Migrator, 0)
	for _, at := range all.Changes {
		if renamed := at.RenameAttributes; renamed != nil {
			attributeChangeSet := migrate.NewAttributeChangeSet(renamed.AttributeMap)
			allTransformer := transformer.NewAllAttributesTransformer(attributeChangeSet)
			values = append(values, allTransformer)
		}
	}
	return &changelist.ChangeList{Migrators: values}
}

func newResourceChangeList(resource ast.Attributes) *changelist.ChangeList {
	values := make([]migrate.Migrator, 0)
	for _, at := range resource.Changes {
		if renamed := at.RenameAttributes; renamed != nil {
			attributeChangeSet := migrate.NewAttributeChangeSet(renamed.AttributeMap)
			resourceTransformer := transformer.ResourceAttributes{AttributeChange: attributeChangeSet}
			values = append(values, resourceTransformer)
		}
	}
	return &changelist.ChangeList{Migrators: values}
}

func newSpanChangeList(spans ast.Spans) *changelist.ChangeList {
	values := make([]migrate.Migrator, 0)
	for _, at := range spans.Changes {
		if renamed := at.RenameAttributes; renamed != nil {
			conditionalAttributesChangeSet := transformer.SpanConditionalAttributes{Migrator: migrate.NewConditionalAttributeSet(renamed.AttributeMap, renamed.ApplyToSpans...)}
			values = append(values, conditionalAttributesChangeSet)
		}
	}
	return &changelist.ChangeList{Migrators: values}
}

func newMetricChangeList(metrics ast.Metrics) *changelist.ChangeList {
	values := make([]migrate.Migrator, 0)
	for _, at := range metrics.Changes {
		if renameAttributes := at.RenameAttributes; renameAttributes != nil {
			attributeChangeSet := transformer.MetricDataPointAttributes{
				ConditionalAttributeChange: migrate.NewConditionalAttributeSet(renameAttributes.AttributeMap, renameAttributes.ApplyToMetrics...),
			}
			values = append(values, attributeChangeSet)
		} else if renamedMetrics := at.RenameMetrics; renamedMetrics != nil {
			signalNameChange := transformer.MetricSignalNameChange{SignalNameChange: migrate.NewSignalNameChange(renamedMetrics)}
			values = append(values, signalNameChange)
		}
	}
	return &changelist.ChangeList{Migrators: values}
}

func newSpanEventChangeList(spanEvents ast.SpanEvents) *changelist.ChangeList {
	values := make([]migrate.Migrator, 0)
	for _, at := range spanEvents.Changes {
		if renamedEvent := at.RenameEvents; renamedEvent != nil {
			signalNameChange := migrate.NewSignalNameChange(renamedEvent.EventNameMap)
			spanEventSignalNameChange := transformer.SpanEventSignalNameChange{SignalNameChange: signalNameChange}
			values = append(values, spanEventSignalNameChange)
		} else if renamedAttribute := at.RenameAttributes; renamedAttribute != nil {
			acceptableSpanNames := make([]string, 0)
			for _, spanName := range renamedAttribute.ApplyToSpans {
				acceptableSpanNames = append(acceptableSpanNames, string(spanName))
			}
			acceptableEventNames := make([]string, 0)
			for _, eventName := range renamedAttribute.ApplyToEvents {
				acceptableEventNames = append(acceptableEventNames, string(eventName))
			}

			multiConditionalAttributeSet := migrate.NewMultiConditionalAttributeSet(renamedAttribute.AttributeMap, map[string][]string{
				"span.name":  acceptableSpanNames,
				"event.name": acceptableEventNames,
			})
			spanEventAttributeChangeSet := transformer.SpanEventConditionalAttributes{MultiConditionalAttributeSet: multiConditionalAttributeSet}
			values = append(values, spanEventAttributeChangeSet)
		} else {
			panic("spanEvents change must have either RenameEvents or RenameAttributes")
		}
	}
	return &changelist.ChangeList{Migrators: values}
}

func newLogsChangelist(logs ast.Logs) *changelist.ChangeList {
	values := make([]migrate.Migrator, 0)
	for _, at := range logs.Changes {
		if renamed := at.RenameAttributes; renamed != nil {
			attributeChangeSet := migrate.NewAttributeChangeSet(renamed.AttributeMap)
			logTransformer := transformer.LogAttributes{AttributeChange: attributeChangeSet}
			values = append(values, logTransformer)
		}
	}
	return &changelist.ChangeList{Migrators: values}
}
