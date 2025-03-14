// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package translation // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor/internal/translation"

import (
	"sort"
	"strings"

	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	encoder "go.opentelemetry.io/otel/schema/v1.1"
	ast11 "go.opentelemetry.io/otel/schema/v1.1/ast"
	"go.uber.org/multierr"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor/internal/alias"
)

// Translation defines the complete abstraction of schema translation file
// that is defined as part of the https://opentelemetry.io/docs/reference/specification/schemas/file_format_v1.0.0/
// Each instance of Translation is "Target Aware", meaning that given a schemaURL as an input
// it will convert from the given input, to the configured target.
//
// Note: as an optimisation, once a Translation is returned from the manager,
//
//	there is no checking the incoming signals if the schema family is a match.
type Translation interface {
	// SupportedVersion checks to see if the provided version is defined as part
	// of this translation since it is useful to know if the translation is missing
	// updates.
	SupportedVersion(v *Version) bool

	// ApplyAllResourceChanges will modify the resource part of the incoming signals
	// This applies to all telemetry types and should be applied there
	ApplyAllResourceChanges(in alias.Resource, inSchemaURL string) error

	// ApplyScopeSpanChanges will modify all spans and span events within the incoming signals
	ApplyScopeSpanChanges(in ptrace.ScopeSpans, inSchemaURL string) error

	// ApplyScopeLogChanges will modify all logs within the incoming signal
	ApplyScopeLogChanges(in plog.ScopeLogs, inSchemaURL string) error

	// ApplyScopeMetricChanges will update all metrics including
	// histograms, exponential histograms, summaries, sum and gauges
	ApplyScopeMetricChanges(in pmetric.ScopeMetrics, inSchemaURL string) error
}

type translator struct {
	targetSchemaURL string
	target          *Version
	indexes         map[Version]int // map from version to index in revisions containing the pertinent Version
	revisions       []RevisionV1

	log *zap.Logger
}

type iterator func() (r RevisionV1, more bool)

var (
	_ sort.Interface = (*translator)(nil)
	_ Translation    = (*translator)(nil)
)

func (t *translator) loadTranslation(content *ast11.Schema) error {
	var errs error
	t.log.Debug("Updating translation")
	for v, def := range content.Versions {
		version, err := NewVersion(string(v))
		if err != nil {
			errs = multierr.Append(errs, err)
			continue
		}
		_, exist := t.indexes[*version]
		if exist {
			continue
		}
		t.log.Debug("Creating new entry",
			zap.Stringer("version", version),
		)
		t.indexes[*version], t.revisions = len(t.revisions), append(t.revisions,
			*NewRevision(version, def),
		)
	}
	sort.Sort(t)

	t.log.Debug("Finished update")
	return errs
}

func newTranslatorFromSchema(log *zap.Logger, targetSchemaURL string, schemaFileSchema *ast11.Schema) (*translator, error) {
	_, target, err := GetFamilyAndVersion(targetSchemaURL)
	if err != nil {
		return nil, err
	}
	t := &translator{
		targetSchemaURL: targetSchemaURL,
		target:          target,
		log:             log,
		indexes:         map[Version]int{},
	}

	if err := t.loadTranslation(schemaFileSchema); err != nil {
		return nil, err
	}
	return t, nil
}

func newTranslator(log *zap.Logger, targetSchemaURL string, schema string) (*translator, error) {
	schemaFileSchema, err := encoder.Parse(strings.NewReader(schema))
	if err != nil {
		return nil, err
	}
	var t *translator
	if t, err = newTranslatorFromSchema(log, targetSchemaURL, schemaFileSchema); err != nil {
		return nil, err
	}
	return t, nil
}

func (t *translator) Len() int {
	return len(t.revisions)
}

func (t *translator) Less(i, j int) bool {
	return t.revisions[i].Version().LessThan(t.revisions[j].Version())
}

func (t *translator) Swap(i, j int) {
	a, b := t.revisions[i].Version(), t.revisions[j].Version()
	t.indexes[*a], t.indexes[*b] = j, i
	t.revisions[i], t.revisions[j] = t.revisions[j], t.revisions[i]
}

func (t *translator) SupportedVersion(v *Version) bool {
	_, ok := t.indexes[*v]
	return ok
}

func (t *translator) ApplyAllResourceChanges(resource alias.Resource, inSchemaURL string) error {
	t.log.Debug("Applying all resource changes")
	_, ver, err := GetFamilyAndVersion(inSchemaURL)
	if err != nil {
		return err
	}
	it, status := t.iterator(ver)
	for rev, more := it(); more; rev, more = it() {
		switch status {
		case Update:
			err = rev.all.Apply(resource.Resource())
			if err != nil {
				return err
			}
			err = rev.resources.Apply(resource.Resource())
			if err != nil {
				return err
			}
		case Revert:
			err = rev.resources.Rollback(resource.Resource())
			if err != nil {
				return err
			}
			err = rev.all.Rollback(resource.Resource())
			if err != nil {
				return err
			}
		}
	}
	resource.SetSchemaUrl(t.targetSchemaURL)
	return nil
}

func (t *translator) ApplyScopeLogChanges(scopeLogs plog.ScopeLogs, inSchemaURL string) error {
	_, ver, err := GetFamilyAndVersion(inSchemaURL)
	if err != nil {
		return err
	}
	it, status := t.iterator(ver)
	if status == NoChange {
		return nil
	}
	for rev, more := it(); more; rev, more = it() {
		for l := 0; l < scopeLogs.LogRecords().Len(); l++ {
			log := scopeLogs.LogRecords().At(l)
			switch status {
			case Update:
				err = rev.all.Apply(log)
				if err != nil {
					return err
				}
				err = rev.logs.Apply(log)
				if err != nil {
					return err
				}
			case Revert:
				err = rev.logs.Rollback(log)
				if err != nil {
					return err
				}
				err = rev.all.Rollback(log)
				if err != nil {
					return err
				}
			}
		}
	}
	scopeLogs.SetSchemaUrl(t.targetSchemaURL)
	return nil
}

func (t *translator) ApplyScopeSpanChanges(scopeSpans ptrace.ScopeSpans, inSchemaURL string) error {
	_, ver, err := GetFamilyAndVersion(inSchemaURL)
	if err != nil {
		return err
	}
	it, status := t.iterator(ver)
	for rev, more := it(); more; rev, more = it() {
		for i := 0; i < scopeSpans.Spans().Len(); i++ {
			span := scopeSpans.Spans().At(i)
			switch status {
			case Update:
				err = rev.all.Apply(span)
				if err != nil {
					return err
				}
				err = rev.spans.Apply(span)
				if err != nil {
					return err
				}
				for e := 0; e < span.Events().Len(); e++ {
					event := span.Events().At(e)
					err = rev.all.Apply(event)
					if err != nil {
						return err
					}
				}
				if err = rev.spanEvents.Apply(span); err != nil {
					return err
				}
			case Revert:
				if err = rev.spanEvents.Rollback(span); err != nil {
					return err
				}
				for e := 0; e < span.Events().Len(); e++ {
					event := span.Events().At(e)
					err = rev.all.Rollback(event)
					if err != nil {
						return err
					}
				}
				err = rev.spans.Rollback(span)
				if err != nil {
					return err
				}
				err = rev.all.Rollback(span)
				if err != nil {
					return err
				}
			}
		}
		scopeSpans.SetSchemaUrl(t.targetSchemaURL)
	}
	return nil
}

func (t *translator) ApplyScopeMetricChanges(scopeMetrics pmetric.ScopeMetrics, inSchemaURL string) error {
	_, ver, err := GetFamilyAndVersion(inSchemaURL)
	if err != nil {
		return err
	}
	it, status := t.iterator(ver)
	for rev, more := it(); more; rev, more = it() {
		for i := 0; i < scopeMetrics.Metrics().Len(); i++ {
			metric := scopeMetrics.Metrics().At(i)
			switch status {
			case Update:
				if err = rev.all.Apply(metric); err != nil {
					return err
				}
				if err = rev.metrics.Apply(metric); err != nil {
					return err
				}
			case Revert:
				if err = rev.metrics.Rollback(metric); err != nil {
					return err
				}
				if err = rev.all.Rollback(metric); err != nil {
					return err
				}
			}
		}
	}
	scopeMetrics.SetSchemaUrl(t.targetSchemaURL)
	return nil
}

// iterator abstracts the logic to perform the migrations of "From Version to Version".
// The return values an iterator type and translation status that
// should be compared against Revert, Update, NoChange
// to determine what should be applied.
// In the event that the ChangeSet has not yet been created (it is possible on a cold start)
// then the iterator will wait til the update has been made
//
// Note: Once an iterator has been made, the passed context MUST cancel or run to completion
//
//	in order for the read lock to be released if either Revert or Upgrade has been returned.
func (t *translator) iterator(from *Version) (iterator, int) {
	status := from.Compare(t.target)
	if status == NoChange || !t.SupportedVersion(from) {
		return func() (r RevisionV1, more bool) { return RevisionV1{}, false }, NoChange
	}
	it, stop := t.indexes[*from], t.indexes[*t.target]
	if status == Update {
		// In the event of an update, the iterator needs to also run that version
		// for the signal to be the correct version.
		stop++

		// we need to not run the starting version to start with, that's already been done!
		it++
	}
	return func() (RevisionV1, bool) {
		// Performs a bounds check and if it has reached stop
		if it < 0 || it == len(t.revisions) || it == stop {
			return RevisionV1{}, false
		}

		r := t.revisions[it]
		// The iterator value needs to move the opposite direction of what
		// status is defined as so subtracting it to progress the iterator.
		it -= status
		return r, true
	}, status
}
