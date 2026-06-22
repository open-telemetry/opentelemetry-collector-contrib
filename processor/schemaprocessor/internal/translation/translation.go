// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package translation // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor/internal/translation"

import (
	"errors"
	"fmt"
	"sort"

	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
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

	// TargetSchemaURL returns the target schema URL for this translation.
	TargetSchemaURL() string

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
	copyFromVersion *Version
	// singleHop is true for v2 resolved registries: there is exactly one
	// revision (the registry head), and any incoming version != target should
	// trigger that single revision in Apply/Revert direction.
	singleHop bool

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
	for v := range content.Versions {
		def := content.Versions[v]
		version, err := NewVersion(string(v))
		if err != nil {
			errs = multierr.Append(errs, err)
			continue
		}
		_, exist := t.indexes[*version]
		if exist {
			continue
		}
		// When copyFromVersion is set, attribute renames preserve both old
		// and new names for revisions between copyFromVersion and the target
		// (in either direction).
		copyAttributes := false
		if t.copyFromVersion != nil {
			lower, upper := t.copyFromVersion, t.target
			if t.target.LessThan(t.copyFromVersion) {
				lower, upper = t.target, t.copyFromVersion
			}
			copyAttributes = lower.LessThan(version) && !upper.LessThan(version)
		}
		rev, err := NewRevision(version, def, copyAttributes)
		if err != nil {
			errs = multierr.Append(errs, err)
			continue
		}
		t.log.Debug(
			"Creating new entry",
			zap.Stringer("version", version),
		)
		t.indexes[*version], t.revisions = len(t.revisions), append(t.revisions, *rev)
	}
	sort.Sort(t)

	t.log.Debug("Finished update")
	return errs
}

func newTranslatorFromSchema(log *zap.Logger, targetSchemaURL string, schemaFileSchema *ast11.Schema, copyFromVersion *Version) (*translator, error) {
	_, target, err := GetFamilyAndVersion(targetSchemaURL)
	if err != nil {
		return nil, err
	}
	t := &translator{
		targetSchemaURL: targetSchemaURL,
		target:          target,
		log:             log,
		copyFromVersion: copyFromVersion,
		indexes:         map[Version]int{},
	}

	if err := t.loadTranslation(schemaFileSchema); err != nil {
		return nil, err
	}
	return t, nil
}

// newTranslatorFromV2Resolved builds a single-hop translator from a v2
// resolved registry. There is exactly one revision (the registry head); any
// incoming version != target triggers it in Update or Revert direction.
func newTranslatorFromV2Resolved(log *zap.Logger, targetSchemaURL string, resolved *V2Resolved, copyFromVersion *Version) (*translator, error) {
	_, target, err := GetFamilyAndVersion(targetSchemaURL)
	if err != nil {
		return nil, err
	}
	// copyAttributes applies to renames between copyFromVersion and target,
	// inclusive on the lower bound. With a single-hop registry there is no
	// per-version chain to consult, so the flag is effectively "enabled when
	// migration mode is configured".
	copyAttributes := copyFromVersion != nil
	rev := NewRevisionFromV2Resolved(target, resolved, copyAttributes)
	t := &translator{
		targetSchemaURL: targetSchemaURL,
		target:          target,
		log:             log,
		copyFromVersion: copyFromVersion,
		indexes:         map[Version]int{*target: 0},
		revisions:       []RevisionV1{*rev},
		singleHop:       true,
	}
	log.Debug(
		"loaded v2 resolved registry",
		zap.Int("attribute_catalog_size", len(resolved.AttributeCatalog)),
		zap.Int("metric_signals", len(resolved.Registry.Metrics)),
		zap.Int("span_signals", len(resolved.Registry.Spans)),
		zap.Int("event_signals", len(resolved.Registry.Events)),
	)
	return t, nil
}

// newTranslatorFromParsed dispatches on the parsed schema format. v2
// manifests cannot be turned into a translator directly; the caller (manager)
// must follow resolved_registry_uri and re-invoke this function with the
// resolved/2.0 result.
func newTranslatorFromParsed(log *zap.Logger, targetSchemaURL string, parsed *ParsedSchema, copyFromVersion *Version) (*translator, error) {
	switch parsed.Format {
	case SchemaFormatV1:
		return newTranslatorFromSchema(log, targetSchemaURL, parsed.V1, copyFromVersion)
	case SchemaFormatV2Resolved:
		return newTranslatorFromV2Resolved(log, targetSchemaURL, parsed.V2Resolved, copyFromVersion)
	case SchemaFormatV2Manifest:
		return nil, errors.New("manifest/2.0 requires resolved_registry_uri fetch by caller")
	default:
		return nil, fmt.Errorf("unsupported parsed schema format: %d", parsed.Format)
	}
}

// newTranslator parses a schema document and returns a translator. Manifest
// documents return an error since they require a follow-up fetch (manager
// handles that path via newTranslatorFromParsed).
func newTranslator(log *zap.Logger, targetSchemaURL, schema string, copyFromVersion *Version) (*translator, error) {
	parsed, err := Parse(schema)
	if err != nil {
		return nil, err
	}
	return newTranslatorFromParsed(log, targetSchemaURL, parsed, copyFromVersion)
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

func (t *translator) TargetSchemaURL() string {
	return t.targetSchemaURL
}

func (t *translator) SupportedVersion(v *Version) bool {
	if t.singleHop {
		// v2 resolved registries hold renames for all historic predecessors
		// of the head version. Any incoming version is "supported" in the
		// sense that we will attempt translation; unmapped names pass through.
		return v != nil
	}
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
				err = rev.spanEvents.Apply(span)
				if err != nil {
					return err
				}
			case Revert:
				err = rev.spanEvents.Rollback(span)
				if err != nil {
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
				if err := rev.all.Apply(metric); err != nil {
					return err
				}
				if err := rev.metrics.Apply(metric); err != nil {
					return err
				}
			case Revert:
				if err := rev.metrics.Rollback(metric); err != nil {
					return err
				}
				if err := rev.all.Rollback(metric); err != nil {
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
	if t.singleHop {
		// Single revision (the v2 head). Yield it once; direction is set by
		// status (Update for from<target, Revert for from>target).
		consumed := false
		return func() (RevisionV1, bool) {
			if consumed || len(t.revisions) == 0 {
				return RevisionV1{}, false
			}
			consumed = true
			return t.revisions[0], true
		}, status
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
