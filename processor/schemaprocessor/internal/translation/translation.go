// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package translation // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor/internal/translation"

import (
	"context"
	"io"
	"sort"

	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	encoder "go.opentelemetry.io/otel/schema/v1.0"
	"go.opentelemetry.io/otel/schema/v1.0/ast"
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
	// SupportedVersions checks to see if the provided version is defined as part
	// of this translation since it is useful to know it the translation is missing
	// updates.
	SupportedVersion(v *Version) bool

	// ApplyAllResourceChanges will modify the resource part of the incoming signals
	// This applies to all telemetry types and should be applied there
	ApplyAllResourceChanges(ctx context.Context, in alias.Resource, inSchemaURL string) error

	// ApplyScopeSpanChanges will modify all spans and span events within the incoming signals
	ApplyScopeSpanChanges(ctx context.Context, in ptrace.ScopeSpans, inSchemaURL string) error

	// ApplyScopeLogChanges will modify all logs within the incoming signal
	ApplyScopeLogChanges(ctx context.Context, in plog.ScopeLogs, inSchemaURL string) error

	// ApplyScopeMetricChanges will update all metrics including
	// histograms, exponetial histograms, summarys, sum and gauges
	ApplyScopeMetricChanges(ctx context.Context, in pmetric.ScopeMetrics, inSchemaURL string) error
}

type translator struct {
	schemaURL string
	target    *Version
	indexes   map[Version]int
	revisions []RevisionV1

	log *zap.Logger
}

type iterator func() (r RevisionV1, more bool)

var (
	_ sort.Interface = (*translator)(nil)
	_ Translation    = (*translator)(nil)
)

func (t *translator) loadTranslation(content *ast.Schema) error {
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

func newTranslatorFromSchema(log *zap.Logger, schemaURL string, schemaFileSchema *ast.Schema) (*translator, error) {
	_, target, err := GetFamilyAndVersion(schemaURL)
	if err != nil {
		return nil, err
	}
	t := &translator{
		schemaURL: schemaURL,
		target:    target,
		log:       log,
		indexes:   map[Version]int{},
	}

	if err := t.loadTranslation(schemaFileSchema); err != nil {
		return nil, err
	}
	return t, nil
}

func newTranslatorFromReader(log *zap.Logger, schemaURL string, content io.Reader) (*translator, error) {
	schemaFileSchema, err := encoder.Parse(content)
	if err != nil {
		return nil, err
	}
	var t *translator
	if t, err = newTranslatorFromSchema(log, schemaURL, schemaFileSchema); err != nil {
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

func (t *translator) ApplyAllResourceChanges(ctx context.Context, resource alias.Resource, inSchemaURL string) error {
	_, ver, err := GetFamilyAndVersion(inSchemaURL)
	if err != nil {
		return err
	}
	it, status := t.iterator(ctx, ver)
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
	resource.SetSchemaUrl(t.schemaURL)
	return nil
}

func (t *translator) ApplyScopeLogChanges(ctx context.Context, in plog.ScopeLogs, inSchemaURL string) error {
	_, ver, err := GetFamilyAndVersion(inSchemaURL)
	if err != nil {
		return err
	}
	it, status := t.iterator(ctx, ver)
	if status == NoChange {
		return nil
	}
	for rev, more := it(); more; rev, more = it() {
		for l := 0; l < in.LogRecords().Len(); l++ {
			log := in.LogRecords().At(l)
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
				err = rev.all.Rollback(log)
				if err != nil {
					return err
				}
				err = rev.logs.Rollback(log)
				if err != nil {
					return err
				}
			}
		}
	}
	in.SetSchemaUrl(t.schemaURL)
	return nil
}

func (t *translator) ApplyScopeSpanChanges(ctx context.Context, scopeSpans ptrace.ScopeSpans, inSchemaURL string) error {
	_, ver, err := GetFamilyAndVersion(inSchemaURL)
	if err != nil {
		return err
	}
	it, status := t.iterator(ctx, ver)
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
		scopeSpans.SetSchemaUrl(t.schemaURL)
	}
	return nil
}

func (t *translator) ApplyScopeMetricChanges(ctx context.Context, in pmetric.ScopeMetrics, inSchemaURL string) error {
	_, ver, err := GetFamilyAndVersion(inSchemaURL)
	if err != nil {
		return err
	}
	it, status := t.iterator(ctx, ver)
	for rev, more := it(); more; rev, more = it() {
		for i := 0; i < in.Metrics().Len(); i++ {
			metric := in.Metrics().At(i)
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
	in.SetSchemaUrl(t.schemaURL)
	return nil
}

// iterator abstractions the logic to perform an the migrations of, "From Version to Version".
// The return values an iterator type and translation status that
// should be compared against Revert, Update, NoChange
// to determine what should be applied.
// In the event that the ChangeSet has not yet been created (it is possible on a cold start)
// then the iterator will wait til the update has been made
//
// Note: Once an iterator has been made, the passed context MUST cancel or run to completion
//
//	in order for the read lock to be released if either Revert or Upgrade has been returned.
func (t *translator) iterator(ctx context.Context, from *Version) (iterator, int) {
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
		select {
		case <-ctx.Done():
			return RevisionV1{}, false
		default:
			// No action required heree
		}

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
