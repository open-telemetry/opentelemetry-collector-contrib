// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package translation // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor/internal/translation"

import (
	"errors"
	"fmt"

	ast10 "go.opentelemetry.io/otel/schema/v1.0/ast"
	ast11 "go.opentelemetry.io/otel/schema/v1.1/ast"

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
func NewRevision(ver *Version, def ast11.VersionDef, copyAttributes bool) (*RevisionV1, error) {
	spanEvents, err := newSpanEventChangeList(def.SpanEvents, copyAttributes)
	if err != nil {
		return nil, fmt.Errorf("version %s span_events: %w", ver, err)
	}
	return &RevisionV1{
		ver:        ver,
		all:        newAllChangeList(def.All, copyAttributes),
		resources:  newResourceChangeList(def.Resources, copyAttributes),
		spans:      newSpanChangeList(def.Spans, copyAttributes),
		spanEvents: spanEvents,
		metrics:    newMetricChangeList(def.Metrics, copyAttributes),
		logs:       newLogsChangelist(def.Logs, copyAttributes),
	}, nil
}

func (r RevisionV1) Version() *Version {
	return r.ver
}

func newAllChangeList(all ast10.Attributes, copyAttributes bool) *changelist.ChangeList {
	values := make([]migrate.Migrator, 0)
	for _, at := range all.Changes {
		if renamed := at.RenameAttributes; renamed != nil {
			attributeChangeSet := migrate.NewAttributeChangeSet(renamed.AttributeMap, copyAttributes)
			allTransformer := transformer.NewAllAttributesTransformer(attributeChangeSet)
			values = append(values, allTransformer)
		}
	}
	return &changelist.ChangeList{Migrators: values}
}

func newResourceChangeList(resource ast10.Attributes, copyAttributes bool) *changelist.ChangeList {
	values := make([]migrate.Migrator, 0)
	for _, at := range resource.Changes {
		if renamed := at.RenameAttributes; renamed != nil {
			attributeChangeSet := migrate.NewAttributeChangeSet(renamed.AttributeMap, copyAttributes)
			resourceTransformer := transformer.ResourceAttributes{AttributeChange: attributeChangeSet}
			values = append(values, resourceTransformer)
		}
	}
	return &changelist.ChangeList{Migrators: values}
}

func newSpanChangeList(spans ast10.Spans, copyAttributes bool) *changelist.ChangeList {
	values := make([]migrate.Migrator, 0)
	for _, at := range spans.Changes {
		if renamed := at.RenameAttributes; renamed != nil {
			conditionalAttributesChangeSet := transformer.SpanConditionalAttributes{Migrator: migrate.NewConditionalAttributeSet(renamed.AttributeMap, copyAttributes, renamed.ApplyToSpans...)}
			values = append(values, conditionalAttributesChangeSet)
		}
	}
	return &changelist.ChangeList{Migrators: values}
}

func newMetricChangeList(metrics ast11.Metrics, copyAttributes bool) *changelist.ChangeList {
	values := make([]migrate.Migrator, 0)
	for _, at := range metrics.Changes {
		if renameAttributes := at.RenameAttributes; renameAttributes != nil {
			attributeChangeSet := transformer.MetricDataPointAttributes{
				ConditionalAttributeChange: migrate.NewConditionalAttributeSet(renameAttributes.AttributeMap, copyAttributes, renameAttributes.ApplyToMetrics...),
			}
			values = append(values, attributeChangeSet)
		}
		if renamedMetrics := at.RenameMetrics; renamedMetrics != nil {
			signalNameChange := transformer.MetricSignalNameChange{SignalNameChange: migrate.NewSignalNameChange(renamedMetrics)}
			values = append(values, signalNameChange)
		}
		if splitMetrics := at.Split; splitMetrics != nil {
			// TODO: Implement split
			continue
		}
	}
	return &changelist.ChangeList{Migrators: values}
}

func newSpanEventChangeList(spanEvents ast10.SpanEvents, copyAttributes bool) (*changelist.ChangeList, error) {
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

			multiConditionalAttributeSet := migrate.NewMultiConditionalAttributeSet(renamedAttribute.AttributeMap, copyAttributes, map[string][]string{
				"span.name":  acceptableSpanNames,
				"event.name": acceptableEventNames,
			})
			spanEventAttributeChangeSet := transformer.SpanEventConditionalAttributes{MultiConditionalAttributeSet: multiConditionalAttributeSet}
			values = append(values, spanEventAttributeChangeSet)
		} else {
			return nil, errors.New("span_events change must have either rename_events or rename_attributes")
		}
	}
	return &changelist.ChangeList{Migrators: values}, nil
}

func newLogsChangelist(logs ast10.Logs, copyAttributes bool) *changelist.ChangeList {
	values := make([]migrate.Migrator, 0)
	for _, at := range logs.Changes {
		if renamed := at.RenameAttributes; renamed != nil {
			attributeChangeSet := migrate.NewAttributeChangeSet(renamed.AttributeMap, copyAttributes)
			logTransformer := transformer.LogAttributes{AttributeChange: attributeChangeSet}
			values = append(values, logTransformer)
		}
	}
	return &changelist.ChangeList{Migrators: values}
}
