// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlprofile // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlprofile"

import (
	"fmt"

	"go.opentelemetry.io/collector/pdata/pprofile"
	"go.uber.org/zap/zapcore"
)

type profile pprofile.Profile

func (p profile) MarshalLogObject(encoder zapcore.ObjectEncoder) error {
	pp := pprofile.Profile(p)
	vts, err := newValueTypes(p, pp.SampleType())
	if err != nil {
		return err
	}
	return encoder.AddArray("sample_type", vts)
}

func (p profile) getString(idx int32) (string, error) {
	pp := pprofile.Profile(p)
	strTable := pp.StringTable()
	if idx >= int32(strTable.Len()) {
		return "", fmt.Errorf("string index out of bounds: %d", idx)
	}
	return strTable.At(int(idx)), nil
}

type valueTypes []valueType

func (s valueTypes) MarshalLogArray(encoder zapcore.ArrayEncoder) error {
	for _, vt := range s {
		if err := encoder.AppendObject(vt); err != nil {
			return err
		}
	}
	return nil
}

func newValueTypes(p profile, sampleType pprofile.ValueTypeSlice) (valueTypes, error) {
	vts := make(valueTypes, 0, sampleType.Len())
	for i := range sampleType.Len() {
		vt, err := newValueType(p, sampleType.At(i))
		if err != nil {
			return nil, err
		}
		vts = append(vts, vt)
	}
	return vts, nil
}

type valueType struct {
	typ                    string
	unit                   string
	aggregationTemporality int32
}

func newValueType(p profile, vt pprofile.ValueType) (valueType, error) {
	var result valueType
	var err error

	if result.typ, err = p.getString(vt.TypeStrindex()); err != nil {
		return result, err
	}
	if result.unit, err = p.getString(vt.UnitStrindex()); err != nil {
		return result, err
	}
	result.aggregationTemporality = int32(vt.AggregationTemporality())

	return result, nil
}

func (vt valueType) MarshalLogObject(encoder zapcore.ObjectEncoder) error {
	encoder.AddString("type", vt.typ)
	encoder.AddString("unit", vt.unit)
	encoder.AddInt32("aggregation_temporality", vt.aggregationTemporality)
	return nil
}
