// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlprofile // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlprofile"

import (
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/pdata/pcommon"
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
	err = encoder.AddArray("sample_type", vts)

	ss, err := newSamples(p, pp.Sample())
	if err != nil {
		return err
	}
	err = encoder.AddArray("sample", ss)

	return err
}

func (p profile) getString(idx int32) (string, error) {
	pp := pprofile.Profile(p)
	strTable := pp.StringTable()
	if idx >= int32(strTable.Len()) {
		return "", fmt.Errorf("string index out of bounds: %d", idx)
	}
	return strTable.At(int(idx)), nil
}

func (p profile) getFunction(idx int32) (function, error) {
	pp := pprofile.Profile(p)
	fnTable := pp.FunctionTable()
	if idx >= int32(fnTable.Len()) {
		return function{}, fmt.Errorf("function index out of bounds: %d", idx)
	}
	return newFunction(p, fnTable.At(int(idx)))
}

func (p profile) getMapping(idx int32) (mapping, error) {
	pp := pprofile.Profile(p)
	mTable := pp.MappingTable()
	if idx >= int32(mTable.Len()) {
		return mapping{}, fmt.Errorf("mapping index out of bounds: %d", idx)
	}
	return newMapping(p, mTable.At(int(idx)))
}

func (p profile) getLocations(start, length int32) (locations, error) {
	pp := pprofile.Profile(p)
	locTable := pp.LocationTable()
	if start >= int32(locTable.Len()) {
		return locations{}, fmt.Errorf("location start index out of bounds: %d", start)
	}
	if start+length > int32(locTable.Len()) {
		return locations{}, fmt.Errorf("location end index out of bounds: %d", start+length)
	}

	ls := make(locations, 0, length)
	for i := range length {
		l, err := newLocation(p, locTable.At(int(start+i)))
		if err != nil {
			return ls, err
		}
		ls = append(ls, l)
	}

	return ls, nil
}

func (p profile) getAttribute(idx int32) (attribut, error) {
	pp := pprofile.Profile(p)
	attrTable := pp.AttributeTable()
	if idx >= int32(attrTable.Len()) {
		return attribut{}, fmt.Errorf("attribute index out of bounds: %d", idx)
	}
	attr := attrTable.At(int(idx))
	// Is there a better way to marshal the value?
	return attribut{attr.Key(), attr.Value().AsString()}, nil
}

type samples []sample

func (ss samples) MarshalLogArray(encoder zapcore.ArrayEncoder) error {
	for _, s := range ss {
		if err := encoder.AppendObject(s); err != nil {
			return err
		}
	}
	return nil
}

func newSamples(p profile, sampleSlice pprofile.SampleSlice) (samples, error) {
	ss := make(samples, 0, sampleSlice.Len())
	for i := range sampleSlice.Len() {
		s, err := newSample(p, sampleSlice.At(i))
		if err != nil {
			return nil, err
		}
		ss = append(ss, s)
	}
	return ss, nil
}

type sample struct {
	timestamps timestamps
	attributes attributes
	locations  locations
	values     values
}

func newSample(p profile, ps pprofile.Sample) (sample, error) {
	var s sample
	var err error

	if s.timestamps, err = newTimestamps(p, ps.TimestampsUnixNano()); err != nil {
		return s, err
	}
	if s.attributes, err = newAttributes(p, ps.AttributeIndices()); err != nil {
		return s, err
	}
	if s.locations, err = p.getLocations(ps.LocationsStartIndex(), ps.LocationsLength()); err != nil {
		return s, err
	}
	if s.values, err = newValues(p, ps.Value()); err != nil {
		return s, err
	}

	return s, nil
}

func (s sample) MarshalLogObject(encoder zapcore.ObjectEncoder) error {
	err := encoder.AddArray("timestamps_unix_nano", s.timestamps)
	errors.Join(err, encoder.AddArray("attributes", s.attributes))
	errors.Join(err, encoder.AddArray("locations", s.locations))
	errors.Join(err, encoder.AddArray("values", s.values))
	return err
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

func newValueTypes(p profile, sampleTypes pprofile.ValueTypeSlice) (valueTypes, error) {
	vts := make(valueTypes, 0, sampleTypes.Len())
	for i := range sampleTypes.Len() {
		vt, err := newValueType(p, sampleTypes.At(i))
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

type locations []location

func (s locations) MarshalLogArray(encoder zapcore.ArrayEncoder) error {
	for _, l := range s {
		if err := encoder.AppendObject(l); err != nil {
			return err
		}
	}
	return nil
}

type location struct {
	mapping    mapping
	address    uint64
	lines      lines
	isFolded   bool
	attributes attributes
}

func newLocation(p profile, pl pprofile.Location) (location, error) {
	var l location
	var err error

	if pl.MappingIndex() != 0 { // optional
		if l.mapping, err = p.getMapping(pl.MappingIndex()); err != nil {
			return l, err
		}
	}
	if l.attributes, err = newAttributes(p, pl.AttributeIndices()); err != nil {
		return l, err
	}
	if l.lines, err = newLines(p, pl.Line()); err != nil {
		return l, err
	}
	l.address = pl.Address()
	l.isFolded = pl.IsFolded()

	return l, nil
}

func (l location) MarshalLogObject(encoder zapcore.ObjectEncoder) error {
	encoder.AddUint64("address", l.address)
	encoder.AddBool("is_folded", l.isFolded)
	err := encoder.AddObject("mapping", l.mapping)
	errors.Join(err, encoder.AddArray("lines", l.lines))
	errors.Join(err, encoder.AddArray("attributes", l.attributes))
	return err
}

type mappings []mapping

func (s mappings) MarshalLogArray(encoder zapcore.ArrayEncoder) error {
	for _, l := range s {
		if err := encoder.AppendObject(l); err != nil {
			return err
		}
	}
	return nil
}

func newMappings(p profile, pmappings pprofile.MappingSlice) (mappings, error) {
	ms := make(mappings, 0, pmappings.Len())
	for i := range pmappings.Len() {
		m, err := newMapping(p, pmappings.At(i))
		if err != nil {
			return nil, err
		}
		ms = append(ms, m)
	}
	return ms, nil
}

type mapping struct {
	filename        string
	memoryStart     uint64
	memoryLimit     uint64
	fileOffset      uint64
	hasFunctions    bool
	hasFilenames    bool
	hasLineNumbers  bool
	hasInlineFrames bool
}

func newMapping(p profile, pm pprofile.Mapping) (mapping, error) {
	var m mapping
	var err error

	if m.filename, err = p.getString(pm.FilenameStrindex()); err != nil {
		return m, err
	}
	m.memoryStart = pm.MemoryStart()
	m.memoryLimit = pm.MemoryLimit()
	m.fileOffset = pm.FileOffset()
	m.hasFunctions = pm.HasFunctions()
	m.hasFilenames = pm.HasFilenames()
	m.hasLineNumbers = pm.HasLineNumbers()
	m.hasInlineFrames = pm.HasInlineFrames()

	return m, nil
}

func (m mapping) MarshalLogObject(encoder zapcore.ObjectEncoder) error {
	encoder.AddString("filename", m.filename)
	encoder.AddUint64("memory_start", m.memoryStart)
	encoder.AddUint64("memory_limit", m.memoryLimit)
	encoder.AddUint64("file_offset", m.fileOffset)
	encoder.AddBool("has_functions", m.hasFunctions)
	encoder.AddBool("has_filenames", m.hasFilenames)
	encoder.AddBool("has_line_numbers", m.hasLineNumbers)
	encoder.AddBool("has_inline_frames", m.hasInlineFrames)
	return nil
}

type attributes []attribut

func (s attributes) MarshalLogArray(encoder zapcore.ArrayEncoder) error {
	for _, a := range s {
		if err := encoder.AppendObject(a); err != nil {
			return err
		}
	}
	return nil
}

func newAttributes(p profile, pattrs pcommon.Int32Slice) (attributes, error) {
	as := make(attributes, 0, pattrs.Len())
	for i := range pattrs.Len() {
		a, err := p.getAttribute(pattrs.At(i))
		if err != nil {
			return nil, err
		}
		as = append(as, a)
	}
	return as, nil
}

type attribut struct {
	key   string
	value string
}

func (a attribut) MarshalLogObject(encoder zapcore.ObjectEncoder) error {
	encoder.AddString("key", a.key)
	encoder.AddString("value", a.value)
	return nil
}

type lines []line

func (s lines) MarshalLogArray(encoder zapcore.ArrayEncoder) error {
	for _, l := range s {
		if err := encoder.AppendObject(l); err != nil {
			return err
		}
	}
	return nil
}

func newLines(p profile, plines pprofile.LineSlice) (lines, error) {
	ls := make(lines, 0, plines.Len())
	for i := range plines.Len() {
		l, err := newLine(p, plines.At(i))
		if err != nil {
			return nil, err
		}
		ls = append(ls, l)
	}
	return ls, nil
}

type line struct {
	function function
	line     int64
	column   int64
}

func newLine(p profile, pl pprofile.Line) (line, error) {
	var l line
	var err error

	if l.function, err = p.getFunction(pl.FunctionIndex()); err != nil {
		return l, err
	}
	l.line = pl.Line()
	l.column = pl.Column()

	return l, nil
}

func (l line) MarshalLogObject(encoder zapcore.ObjectEncoder) error {
	if err := encoder.AddObject("function", l.function); err != nil {
		return err
	}
	encoder.AddInt64("line", l.line)
	encoder.AddInt64("column", l.column)
	return nil
}

type function struct {
	name       string
	systemName string
	filename   string
	startLine  int64
}

func newFunction(p profile, pf pprofile.Function) (function, error) {
	var f function
	var err error

	if f.name, err = p.getString(pf.NameStrindex()); err != nil {
		return f, err
	}
	if f.systemName, err = p.getString(pf.SystemNameStrindex()); err != nil {
		return f, err
	}
	if f.filename, err = p.getString(pf.FilenameStrindex()); err != nil {
		return f, err
	}
	f.startLine = pf.StartLine()

	return f, nil
}

func (f function) MarshalLogObject(encoder zapcore.ObjectEncoder) error {
	encoder.AddString("name", f.name)
	encoder.AddString("system_name", f.systemName)
	encoder.AddString("filename", f.filename)
	encoder.AddInt64("start_line", f.startLine)
	return nil
}

type timestamps []timestamp

func (s timestamps) MarshalLogArray(encoder zapcore.ArrayEncoder) error {
	for _, t := range s {
		if err := encoder.AppendObject(t); err != nil {
			return err
		}
	}
	return nil
}

func newTimestamps(p profile, ptimestamps pcommon.UInt64Slice) (timestamps, error) {
	ts := make(timestamps, 0, ptimestamps.Len())
	for i := range ptimestamps.Len() {
		ts = append(ts, timestamp(ptimestamps.At(i)))
	}
	return ts, nil
}

type timestamp uint64

func (l timestamp) MarshalLogObject(encoder zapcore.ObjectEncoder) error {
	encoder.AddUint64("timestamp_unix_nano", uint64(l))
	return nil
}

type values []value

func (s values) MarshalLogArray(encoder zapcore.ArrayEncoder) error {
	for _, v := range s {
		if err := encoder.AppendObject(v); err != nil {
			return err
		}
	}
	return nil
}

func newValues(p profile, pvalues pcommon.Int64Slice) (values, error) {
	vs := make(values, 0, pvalues.Len())
	for i := range pvalues.Len() {
		vs = append(vs, value(pvalues.At(i)))
	}
	return vs, nil
}

type value int64

func (v value) MarshalLogObject(encoder zapcore.ObjectEncoder) error {
	encoder.AddInt64("value", int64(v))
	return nil
}
