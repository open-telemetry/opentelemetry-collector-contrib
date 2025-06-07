// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pprofiletest // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pprofiletest"
import (
	"fmt"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pprofile"
)

type Profiles struct {
	ResourceProfiles []ResourceProfile
}

func (p Profiles) Transform() pprofile.Profiles {
	pp := pprofile.NewProfiles()
	for _, rp := range p.ResourceProfiles {
		rp.Transform(pp)
	}
	return pp
}

type ResourceProfile struct {
	ScopeProfiles []ScopeProfile
	Resource      Resource
}

func (rp ResourceProfile) Transform(pp pprofile.Profiles) pprofile.ResourceProfiles {
	prp := pp.ResourceProfiles().AppendEmpty()
	for _, sp := range rp.ScopeProfiles {
		sp.Transform(pp.ProfilesDictionary(), prp)
	}
	for _, a := range rp.Resource.Attributes {
		if prp.Resource().Attributes().PutEmpty(a.Key).FromRaw(a.Value) != nil {
			panic(fmt.Sprintf("unsupported resource attribute value: {%s: %v (type %T)}",
				a.Key, a.Value, a.Value))
		}
	}
	return prp
}

type Resource struct {
	Attributes []Attribute
}

type ScopeProfile struct {
	Profile   []Profile
	Scope     Scope
	SchemaURL string
}

func (sp ScopeProfile) Transform(dic pprofile.ProfilesDictionary, prp pprofile.ResourceProfiles) pprofile.ScopeProfiles {
	psp := prp.ScopeProfiles().AppendEmpty()
	for _, p := range sp.Profile {
		p.Transform(dic, psp)
	}
	sp.Scope.Transform(psp)
	psp.SetSchemaUrl(sp.SchemaURL)

	return psp
}

type Scope struct {
	Attributes             []Attribute
	Name                   string
	Version                string
	DroppedAttributesCount uint32
}

func (sc Scope) Transform(psp pprofile.ScopeProfiles) pcommon.InstrumentationScope {
	psc := psp.Scope()
	for _, a := range sc.Attributes {
		if psc.Attributes().PutEmpty(a.Key).FromRaw(a.Value) != nil {
			panic(fmt.Sprintf("unsupported scope attribute value: {%s: %v (type %T)}",
				a.Key, a.Value, a.Value))
		}
	}
	psc.SetName(sc.Name)
	psc.SetVersion(sc.Version)
	psc.SetDroppedAttributesCount(sc.DroppedAttributesCount)

	return psc
}

type Profile struct {
	SampleType             ValueTypes
	Sample                 []Sample
	TimeNanos              pcommon.Timestamp
	DurationNanos          pcommon.Timestamp
	PeriodType             ValueType
	Period                 int64
	Comment                []string
	DefaultSampleType      ValueType
	ProfileID              pprofile.ProfileID
	DroppedAttributesCount uint32
	OriginalPayloadFormat  string
	OriginalPayload        []byte
	Attributes             []Attribute
	AttributeUnits         []AttributeUnit
}

func (p *Profile) Transform(dic pprofile.ProfilesDictionary, psp pprofile.ScopeProfiles) pprofile.Profile {
	pp := psp.Profiles().AppendEmpty()

	// Avoids that 0 (default) string indices point to nowhere.
	addString(dic, "")

	// If valueTypes are not set, set them to the default value.
	defaultValueType := ValueType{Typ: "samples", Unit: "count", AggregationTemporality: pprofile.AggregationTemporalityDelta}
	if p.PeriodType.Typ == "" && p.PeriodType.Unit == "" && p.PeriodType.AggregationTemporality == 0 {
		p.PeriodType = defaultValueType
	}
	if p.DefaultSampleType.Typ == "" && p.DefaultSampleType.Unit == "" && p.DefaultSampleType.AggregationTemporality == 0 {
		p.DefaultSampleType = defaultValueType
	}

	p.SampleType.Transform(dic, pp)
	for _, sa := range p.Sample {
		sa.Transform(dic, pp)
	}
	pp.SetTime(p.TimeNanos)
	pp.SetDuration(p.DurationNanos)
	p.PeriodType.CopyTo(dic, pp.PeriodType())
	pp.SetPeriod(p.Period)
	for _, c := range p.Comment {
		pp.CommentStrindices().Append(addString(dic, c))
	}
	p.DefaultSampleType.Transform(dic, pp)
	pp.SetProfileID(p.ProfileID)
	pp.SetDroppedAttributesCount(p.DroppedAttributesCount)
	pp.SetOriginalPayloadFormat(p.OriginalPayloadFormat)
	pp.OriginalPayload().FromRaw(p.OriginalPayload)
	for _, at := range p.Attributes {
		pp.AttributeIndices().Append(at.Transform(dic))
	}
	for _, au := range p.AttributeUnits {
		au.Transform(dic)
	}

	return pp
}

func addString(dic pprofile.ProfilesDictionary, s string) int32 {
	for i := range dic.StringTable().Len() {
		if dic.StringTable().At(i) == s {
			return int32(i)
		}
	}
	dic.StringTable().Append(s)
	return int32(dic.StringTable().Len() - 1)
}

type ValueTypes []ValueType

func (vts *ValueTypes) Transform(dic pprofile.ProfilesDictionary, pp pprofile.Profile) {
	for _, vt := range *vts {
		vt.Transform(dic, pp)
	}
}

type ValueType struct {
	Typ                    string
	Unit                   string
	AggregationTemporality pprofile.AggregationTemporality
}

func (vt *ValueType) exists(dic pprofile.ProfilesDictionary, pp pprofile.Profile) bool {
	for i := range pp.SampleType().Len() {
		st := pp.SampleType().At(i)
		if vt.Typ == dic.StringTable().At(int(st.TypeStrindex())) &&
			vt.Unit == dic.StringTable().At(int(st.UnitStrindex())) &&
			vt.AggregationTemporality == st.AggregationTemporality() {
			return true
		}
	}
	return false
}

func (vt *ValueType) CopyTo(dic pprofile.ProfilesDictionary, pvt pprofile.ValueType) {
	pvt.SetTypeStrindex(addString(dic, vt.Typ))
	pvt.SetUnitStrindex(addString(dic, vt.Unit))
	pvt.SetAggregationTemporality(vt.AggregationTemporality)
}

func (vt *ValueType) Transform(dic pprofile.ProfilesDictionary, pp pprofile.Profile) {
	if !vt.exists(dic, pp) {
		vt.CopyTo(dic, pp.SampleType().AppendEmpty())
	}
}

type Sample struct {
	Link               *Link // optional
	Value              []int64
	Locations          []Location
	Attributes         []Attribute
	TimestampsUnixNano []uint64
}

func (sa *Sample) Transform(dic pprofile.ProfilesDictionary, pp pprofile.Profile) {
	if len(sa.Value) != pp.SampleType().Len() {
		panic("length of profile.sample_type must be equal to the length of sample.value")
	}
	psa := pp.Sample().AppendEmpty()
	psa.SetLocationsStartIndex(int32(pp.LocationIndices().Len()))
	for _, loc := range sa.Locations {
		pp.LocationIndices().Append(int32(pp.LocationIndices().Len()))
		ploc := dic.LocationTable().AppendEmpty()
		if loc.Mapping != nil {
			loc.Mapping.Transform(dic)
		}
		ploc.SetAddress(loc.Address)
		ploc.SetIsFolded(loc.IsFolded)
		for _, l := range loc.Line {
			pl := ploc.Line().AppendEmpty()
			pl.SetLine(l.Line)
			pl.SetColumn(l.Column)
			pl.SetFunctionIndex(l.Function.Transform(dic))
		}
		for _, at := range loc.Attributes {
			ploc.AttributeIndices().Append(at.Transform(dic))
		}
	}
	psa.SetLocationsLength(int32(pp.LocationIndices().Len()) - psa.LocationsStartIndex())
	psa.Value().FromRaw(sa.Value)
	for _, at := range sa.Attributes {
		psa.AttributeIndices().Append(at.Transform(dic))
	}
	//nolint:revive,staticcheck
	if sa.Link != nil {
		// psa.SetLinkIndex(sa.Link.Transform(pp)) <-- undefined yet
	}
	psa.TimestampsUnixNano().FromRaw(sa.TimestampsUnixNano)
}

type Location struct {
	Mapping    *Mapping
	Address    uint64
	Line       []Line
	IsFolded   bool
	Attributes []Attribute
}

type Link struct {
	TraceID pcommon.TraceID
	SpanID  pcommon.SpanID
}

func (l *Link) Transform(dic pprofile.ProfilesDictionary) int32 {
	pl := dic.LinkTable().AppendEmpty()
	pl.SetTraceID(l.TraceID)
	pl.SetSpanID(l.SpanID)
	return int32(dic.LinkTable().Len() - 1)
}

type Mapping struct {
	MemoryStart     uint64
	MemoryLimit     uint64
	FileOffset      uint64
	Filename        string
	Attributes      []Attribute
	HasFunctions    bool
	HasFileNames    bool
	HasLineNumbers  bool
	HasInlineFrames bool
}

func (m *Mapping) Transform(dic pprofile.ProfilesDictionary) {
	pm := dic.MappingTable().AppendEmpty()
	pm.SetMemoryStart(m.MemoryStart)
	pm.SetMemoryLimit(m.MemoryLimit)
	pm.SetFileOffset(m.FileOffset)
	pm.SetFilenameStrindex(addString(dic, m.Filename))
	for _, at := range m.Attributes {
		pm.AttributeIndices().Append(at.Transform(dic))
	}
	pm.SetHasFunctions(m.HasFunctions)
	pm.SetHasFilenames(m.HasFileNames)
	pm.SetHasLineNumbers(m.HasLineNumbers)
	pm.SetHasInlineFrames(m.HasInlineFrames)
}

type Attribute struct {
	Key   string
	Value any
}

func (a *Attribute) Transform(dic pprofile.ProfilesDictionary) int32 {
	pa := dic.AttributeTable().AppendEmpty()
	pa.SetKey(a.Key)
	if pa.Value().FromRaw(a.Value) != nil {
		panic(fmt.Sprintf("unsupported attribute value: {%s: %v (type %T)}",
			a.Key, a.Value, a.Value))
	}
	return int32(dic.AttributeTable().Len() - 1)
}

type AttributeUnit struct {
	AttributeKey string
	Unit         string
}

func (a *AttributeUnit) Transform(dic pprofile.ProfilesDictionary) int32 {
	pa := dic.AttributeUnits().AppendEmpty()
	pa.SetAttributeKeyStrindex(addString(dic, a.AttributeKey))
	pa.SetUnitStrindex(addString(dic, a.Unit))
	return int32(dic.AttributeTable().Len() - 1)
}

type Line struct {
	Line     int64
	Column   int64
	Function Function
}

type Function struct {
	Name       string
	SystemName string
	Filename   string
	StartLine  int64
}

func (f *Function) Transform(dic pprofile.ProfilesDictionary) int32 {
	pf := dic.FunctionTable().AppendEmpty()
	pf.SetNameStrindex(addString(dic, f.Name))
	pf.SetSystemNameStrindex(addString(dic, f.SystemName))
	pf.SetFilenameStrindex(addString(dic, f.Filename))
	pf.SetStartLine(f.StartLine)
	return int32(dic.FunctionTable().Len() - 1)
}
