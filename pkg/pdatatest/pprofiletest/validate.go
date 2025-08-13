// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pprofiletest // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pprofiletest"
import (
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pprofile"
)

func ValidateProfile(dic pprofile.ProfilesDictionary, pp pprofile.Profile) error {
	var errs error

	stLen := dic.StringTable().Len()
	if stLen < 1 {
		// Return here to avoid panicking when accessing the string table.
		return errors.New("empty string table, must at least contain the empty string")
	}

	if dic.StringTable().At(0) != "" {
		errs = errors.Join(errs, errors.New("string table must start with the empty string"))
	}

	if pp.SampleType().Len() < 1 {
		// Since the proto field 'default_sample_type_index' is always valid, there must be at least
		// one sample type in the profile.
		errs = errors.Join(errs, errors.New("missing sample type, need at least a default"))
	}

	errs = errors.Join(errs, validateSampleType(dic, pp))

	errs = errors.Join(errs, validateSamples(dic, pp))

	if err := validateValueType(stLen, pp.PeriodType()); err != nil {
		errs = errors.Join(errs, fmt.Errorf("period_type: %w", err))
	}

	if err := validateIndex(stLen, pp.DefaultSampleTypeIndex()); err != nil {
		errs = errors.Join(errs, fmt.Errorf("default_sample_type_strindex: %w", err))
	}

	if err := validateIndices(stLen, pp.CommentStrindices()); err != nil {
		errs = errors.Join(errs, fmt.Errorf("comment_strindices: %w", err))
	}

	if err := validateIndices(dic.AttributeTable().Len(), pp.AttributeIndices()); err != nil {
		errs = errors.Join(errs, fmt.Errorf("attribute_indices: %w", err))
	}

	errs = errors.Join(errs, validateAttributeUnits(dic))

	return errs
}

func validateIndices(length int, indices pcommon.Int32Slice) error {
	var errs error

	for i := range indices.Len() {
		if err := validateIndex(length, indices.At(i)); err != nil {
			errs = errors.Join(errs, fmt.Errorf("[%d]: %w", i, err))
		}
	}

	return errs
}

func validateIndex(length int, idx int32) error {
	if idx < 0 || int(idx) >= length {
		return fmt.Errorf("index %d is out of range [0..%d)",
			idx, length)
	}
	return nil
}

func validateSampleType(dic pprofile.ProfilesDictionary, pp pprofile.Profile) error {
	var errs error

	stLen := dic.StringTable().Len()
	for i := range pp.SampleType().Len() {
		if err := validateValueType(stLen, pp.SampleType().At(i)); err != nil {
			errs = errors.Join(errs, fmt.Errorf("sample_type[%d]: %w", i, err))
		}
	}

	return errs
}

func validateValueType(stLen int, pvt pprofile.ValueType) error {
	var errs error

	if err := validateIndex(stLen, pvt.TypeStrindex()); err != nil {
		errs = errors.Join(errs, fmt.Errorf("type_strindex: %w", err))
	}

	if err := validateIndex(stLen, pvt.UnitStrindex()); err != nil {
		errs = errors.Join(errs, fmt.Errorf("unit_strindex: %w", err))
	}

	if pvt.AggregationTemporality() != pprofile.AggregationTemporalityDelta &&
		pvt.AggregationTemporality() != pprofile.AggregationTemporalityCumulative {
		errs = errors.Join(errs, fmt.Errorf("aggregation_temporality %d is invalid",
			pvt.AggregationTemporality()))
	}

	return errs
}

func validateSamples(dic pprofile.ProfilesDictionary, pp pprofile.Profile) error {
	var errs error

	for i := range pp.Sample().Len() {
		if err := validateSample(dic, pp, pp.Sample().At(i)); err != nil {
			errs = errors.Join(errs, fmt.Errorf("sample[%d]: %w", i, err))
		}
	}

	return errs
}

func validateSample(dic pprofile.ProfilesDictionary, pp pprofile.Profile, sample pprofile.Sample) error {
	var errs error

	length := sample.LocationsLength()
	if length < 0 {
		errs = errors.Join(errs, fmt.Errorf("locations_length %d is negative", length))
	}

	if length > 0 {
		start := sample.LocationsStartIndex()
		if err := validateIndex(pp.LocationIndices().Len(), start); err != nil {
			errs = errors.Join(errs, fmt.Errorf("locations_start_index: %w", err))
		}

		end := start + length
		if err := validateIndex(pp.LocationIndices().Len(), end-1); err != nil {
			errs = errors.Join(errs, fmt.Errorf("locations end (%d+%d): %w", start, length, err))
		}

		if errs != nil {
			// Return here to avoid panicking when accessing the location indices.
			return errs
		}

		for i := start; i < end; i++ {
			locIdx := pp.LocationIndices().At(int(i))
			if err := validateIndex(dic.LocationTable().Len(), locIdx); err != nil {
				errs = errors.Join(errs, fmt.Errorf("location_indices[%d]: %w", i, err))
				continue
			}
			if err := validateLocation(dic, dic.LocationTable().At(int(locIdx))); err != nil {
				errs = errors.Join(errs, fmt.Errorf("locations[%d]: %w", i, err))
			}
		}
	}

	numValues := pp.SampleType().Len()
	if sample.Value().Len() != numValues {
		errs = errors.Join(errs, fmt.Errorf("value length %d does not match sample_type length=%d",
			sample.Value().Len(), numValues))
	}

	if err := validateIndices(dic.AttributeTable().Len(), sample.AttributeIndices()); err != nil {
		errs = errors.Join(errs, fmt.Errorf("attribute_indices: %w", err))
	}

	startTime := uint64(pp.Time().AsTime().UnixNano())
	endTime := startTime + uint64(pp.Duration().AsTime().UnixNano())
	for i := range sample.TimestampsUnixNano().Len() {
		ts := sample.TimestampsUnixNano().At(i)
		if ts < startTime || ts >= endTime {
			errs = errors.Join(errs, fmt.Errorf("timestamps_unix_nano[%d] %d is out of range [%d..%d)",
				i, ts, startTime, endTime))
		}
	}

	if sample.HasLinkIndex() {
		if err := validateIndex(dic.LinkTable().Len(), sample.LinkIndex()); err != nil {
			errs = errors.Join(errs, fmt.Errorf("link_index: %w", err))
		}
	}

	return errs
}

func validateLocation(dic pprofile.ProfilesDictionary, loc pprofile.Location) error {
	var errs error

	if loc.HasMappingIndex() {
		if err := validateIndex(dic.MappingTable().Len(), loc.MappingIndex()); err != nil {
			// Continuing would run into a panic.
			return fmt.Errorf("mapping_index: %w", err)
		}

		if err := validateMapping(dic, dic.MappingTable().At(int(loc.MappingIndex()))); err != nil {
			errs = errors.Join(errs, fmt.Errorf("mapping: %w", err))
		}
	}

	for i := range loc.Line().Len() {
		if err := validateLine(dic, loc.Line().At(i)); err != nil {
			errs = errors.Join(errs, fmt.Errorf("line[%d]: %w", i, err))
		}
	}

	if err := validateIndices(dic.AttributeTable().Len(), loc.AttributeIndices()); err != nil {
		errs = errors.Join(errs, fmt.Errorf("attribute_indices: %w", err))
	}

	return errs
}

func validateLine(dic pprofile.ProfilesDictionary, line pprofile.Line) error {
	if err := validateIndex(dic.FunctionTable().Len(), line.FunctionIndex()); err != nil {
		return fmt.Errorf("function_index: %w", err)
	}

	return nil
}

func validateMapping(dic pprofile.ProfilesDictionary, mapping pprofile.Mapping) error {
	var errs error

	if err := validateIndex(dic.StringTable().Len(), mapping.FilenameStrindex()); err != nil {
		errs = errors.Join(errs, fmt.Errorf("filename_strindex: %w", err))
	}

	if err := validateIndices(dic.AttributeTable().Len(), mapping.AttributeIndices()); err != nil {
		errs = errors.Join(errs, fmt.Errorf("attribute_indices: %w", err))
	}

	return errs
}

func validateAttributeUnits(dic pprofile.ProfilesDictionary) error {
	var errs error

	for i := range dic.AttributeUnits().Len() {
		if err := validateAttributeUnit(dic, dic.AttributeUnits().At(i)); err != nil {
			errs = errors.Join(errs, fmt.Errorf("attribute_units[%d]: %w", i, err))
		}
	}

	return errs
}

func validateAttributeUnit(dic pprofile.ProfilesDictionary, au pprofile.AttributeUnit) error {
	var errs error

	if err := validateIndex(dic.StringTable().Len(), au.AttributeKeyStrindex()); err != nil {
		errs = errors.Join(errs, fmt.Errorf("attribute_key: %w", err))
	}

	if err := validateIndex(dic.StringTable().Len(), au.UnitStrindex()); err != nil {
		errs = errors.Join(errs, fmt.Errorf("unit: %w", err))
	}

	return errs
}
