// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package changelist // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor/internal/changelist"

import (
	"fmt"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor/internal/migrate"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor/internal/operator"
)

// ChangeList represents a list of changes within a section of the schema processor.  It can take in a list of different migrators for a specific section and will apply them in order, based on whether Apply or Rollback is called
type ChangeList struct {
	Migrators []migrate.Migrator
}

func (c ChangeList) Do(ss migrate.StateSelector, signal any) error {
	for i := 0; i < len(c.Migrators); i++ {
		var migrator migrate.Migrator
		// todo(ankit) in go1.23 switch to reversed iterators for this
		if ss == migrate.StateSelectorApply {
			migrator = c.Migrators[i]
		} else {
			migrator = c.Migrators[len(c.Migrators)-1-i]
		}
		// switch between operator types - what do the operators act on?
		switch thisMigrator := migrator.(type) {
		// this one acts on both spans and span events!
		case operator.SpanOperator:
			if span, ok := signal.(ptrace.Span); ok {
				if err := thisMigrator.Do(ss, span); err != nil {
					return err
				}
			} else {
				return fmt.Errorf("SpanOperator %T can't act on %T", thisMigrator, signal)
			}
		case operator.MetricOperator:
			if metric, ok := signal.(pmetric.Metric); ok {
				if err := thisMigrator.Do(ss, metric); err != nil {
					return err
				}
			} else {
				return fmt.Errorf("MetricOperator %T can't act on %T", thisMigrator, signal)
			}
		case operator.LogOperator:
			if log, ok := signal.(plog.LogRecord); ok {
				if err := thisMigrator.Do(ss, log); err != nil {
					return err
				}
			} else {
				return fmt.Errorf("LogOperator %T can't act on %T", thisMigrator, signal)
			}
		case operator.ResourceOperator:
			if resource, ok := signal.(pcommon.Resource); ok {
				if err := thisMigrator.Do(ss, resource); err != nil {
					return err
				}
			} else {
				return fmt.Errorf("ResourceOperator %T can't act on %T", thisMigrator, signal)
			}
		case operator.AllOperator:
			if err := thisMigrator.Do(ss, signal); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unsupported migrator type %T", thisMigrator)
		}
	}
	return nil
}

func (c ChangeList) Apply(signal any) error {
	return c.Do(migrate.StateSelectorApply, signal)
}

func (c ChangeList) Rollback(signal any) error {
	return c.Do(migrate.StateSelectorRollback, signal)
}
