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

	errs = errors.Join(errs, validateSampleType(dic, pp))

	errs = errors.Join(errs, validateSamples(dic, pp))

	if err := validateValueType(stLen, pp.PeriodType()); err != nil {
		errs = errors.Join(errs, fmt.Errorf("period_type: %w", err))
	}

	if err := validateIndices(dic.AttributeTable().Len(), pp.AttributeIndices()); err != nil {
		errs = errors.Join(errs, fmt.Errorf("attribute_indices: %w", err))
	}

	errs = errors.Join(errs, validateKeyValueAndUnits(dic))

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
	stLen := dic.StringTable().Len()
	if err := validateValueType(stLen, pp.SampleType()); err != nil {
		return fmt.Errorf("sample_type: %w", err)
	}

	return nil
}

func validateValueType(stLen int, pvt pprofile.ValueType) error {
	var errs error

	if err := validateIndex(stLen, pvt.TypeStrindex()); err != nil {
		errs = errors.Join(errs, fmt.Errorf("type_strindex: %w", err))
	}

	if err := validateIndex(stLen, pvt.UnitStrindex()); err != nil {
		errs = errors.Join(errs, fmt.Errorf("unit_strindex: %w", err))
	}

	return errs
}

func validateSamples(dic pprofile.ProfilesDictionary, pp pprofile.Profile) error {
	var errs error

	for i := range pp.Samples().Len() {
		if err := validateSample(dic, pp, pp.Samples().At(i)); err != nil {
			errs = errors.Join(errs, fmt.Errorf("sample[%d]: %w", i, err))
		}
	}

	return errs
}

func validateSample(dic pprofile.ProfilesDictionary, pp pprofile.Profile, sample pprofile.Sample) error {
	var errs error

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

	if sample.LinkIndex() > 0 {
		if err := validateIndex(dic.LinkTable().Len(), sample.LinkIndex()); err != nil {
			errs = errors.Join(errs, fmt.Errorf("link_index: %w", err))
		}
	}

	return errs
}

func validateLocation(dic pprofile.ProfilesDictionary, loc pprofile.Location) error {
	var errs error

	if loc.MappingIndex() > 0 {
		if err := validateIndex(dic.MappingTable().Len(), loc.MappingIndex()); err != nil {
			// Continuing would run into a panic.
			return fmt.Errorf("mapping_index: %w", err)
		}

		if err := validateMapping(dic, dic.MappingTable().At(int(loc.MappingIndex()))); err != nil {
			errs = errors.Join(errs, fmt.Errorf("mapping: %w", err))
		}
	}

	for i := range loc.Lines().Len() {
		if err := validateLine(dic, loc.Lines().At(i)); err != nil {
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

func validateKeyValueAndUnits(dic pprofile.ProfilesDictionary) error {
	var errs error

	for i := range dic.AttributeTable().Len() {
		if err := validateKeyValueAndUnit(dic, dic.AttributeTable().At(i)); err != nil {
			errs = errors.Join(errs, fmt.Errorf("attribute_units[%d]: %w", i, err))
		}
	}

	return errs
}

func validateKeyValueAndUnit(dic pprofile.ProfilesDictionary, au pprofile.KeyValueAndUnit) error {
	var errs error

	if err := validateIndex(dic.StringTable().Len(), au.KeyStrindex()); err != nil {
		errs = errors.Join(errs, fmt.Errorf("key: %w", err))
	}

	if err := validateIndex(dic.StringTable().Len(), au.UnitStrindex()); err != nil {
		errs = errors.Join(errs, fmt.Errorf("unit: %w", err))
	}

	return errs
}
