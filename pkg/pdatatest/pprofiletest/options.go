// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pprofiletest // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pprofiletest"

import (
	"bytes"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pprofile"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil"
)

// CompareProfilesOption can be used to mutate expected and/or actual profiles before comparing.
type CompareProfilesOption interface {
	applyOnProfiles(expected, actual pprofile.Profiles)
}

type compareProfilesOptionFunc func(expected, actual pprofile.Profiles)

func (f compareProfilesOptionFunc) applyOnProfiles(expected, actual pprofile.Profiles) {
	f(expected, actual)
}

// IgnoreResourceAttributeValue is a CompareProfilesOption that removes a resource attribute
// from all resources.
func IgnoreResourceAttributeValue(attributeName string) CompareProfilesOption {
	return ignoreResourceAttributeValue{
		attributeName: attributeName,
	}
}

type ignoreResourceAttributeValue struct {
	attributeName string
}

func (opt ignoreResourceAttributeValue) applyOnProfiles(expected, actual pprofile.Profiles) {
	opt.maskProfilesResourceAttributeValue(expected)
	opt.maskProfilesResourceAttributeValue(actual)
}

func (opt ignoreResourceAttributeValue) maskProfilesResourceAttributeValue(profiles pprofile.Profiles) {
	rls := profiles.ResourceProfiles()
	for i := 0; i < rls.Len(); i++ {
		internal.MaskResourceAttributeValue(rls.At(i).Resource(), opt.attributeName)
	}
}

// IgnoreProfileRecordAttributeValue is a CompareProfilesOption that sets the value of an attribute
// to empty bytes for every profile record
func IgnoreProfileRecordAttributeValue(attributeName string) CompareProfilesOption {
	return ignoreProfileRecordAttributeValue{
		attributeName: attributeName,
	}
}

type ignoreProfileRecordAttributeValue struct {
	attributeName string
}

func (opt ignoreProfileRecordAttributeValue) applyOnProfiles(expected, actual pprofile.Profiles) {
	opt.maskProfileRecordAttributeValue(expected)
	opt.maskProfileRecordAttributeValue(actual)
}

func (opt ignoreProfileRecordAttributeValue) maskProfileRecordAttributeValue(profiles pprofile.Profiles) {
	rls := profiles.ResourceProfiles()
	for i := 0; i < profiles.ResourceProfiles().Len(); i++ {
		sls := rls.At(i).ScopeProfiles()
		for j := 0; j < sls.Len(); j++ {
			lrs := sls.At(j).ProfileRecords()
			for k := 0; k < lrs.Len(); k++ {
				lr := lrs.At(k)
				val, exists := lr.Attributes().Get(opt.attributeName)
				if exists {
					val.SetEmptyBytes()
				}
			}
		}
	}
}

func IgnoreTimestamp() CompareProfilesOption {
	return compareProfilesOptionFunc(func(expected, actual pprofile.Profiles) {
		now := pcommon.NewTimestampFromTime(time.Now())
		maskTimestamp(expected, now)
		maskTimestamp(actual, now)
	})
}

func maskTimestamp(profiles pprofile.Profiles, ts pcommon.Timestamp) {
	rls := profiles.ResourceProfiles()
	for i := 0; i < profiles.ResourceProfiles().Len(); i++ {
		sls := rls.At(i).ScopeProfiles()
		for j := 0; j < sls.Len(); j++ {
			lrs := sls.At(j).ProfileRecords()
			for k := 0; k < lrs.Len(); k++ {
				lrs.At(k).SetTimestamp(ts)
			}
		}
	}
}

func IgnoreObservedTimestamp() CompareProfilesOption {
	return compareProfilesOptionFunc(func(expected, actual pprofile.Profiles) {
		now := pcommon.NewTimestampFromTime(time.Now())
		maskObservedTimestamp(expected, now)
		maskObservedTimestamp(actual, now)
	})
}

func maskObservedTimestamp(profiles pprofile.Profiles, ts pcommon.Timestamp) {
	rls := profiles.ResourceProfiles()
	for i := 0; i < profiles.ResourceProfiles().Len(); i++ {
		sls := rls.At(i).ScopeProfiles()
		for j := 0; j < sls.Len(); j++ {
			lrs := sls.At(j).ProfileRecords()
			for k := 0; k < lrs.Len(); k++ {
				lrs.At(k).SetObservedTimestamp(ts)
			}
		}
	}
}

// IgnoreResourceProfilesOrder is a CompareProfilesOption that ignores the order of resource traces/metrics/profiles.
func IgnoreResourceProfilesOrder() CompareProfilesOption {
	return compareProfilesOptionFunc(func(expected, actual pprofile.Profiles) {
		sortResourceProfilesSlice(expected.ResourceProfiles())
		sortResourceProfilesSlice(actual.ResourceProfiles())
	})
}

func sortResourceProfilesSlice(rls pprofile.ResourceProfilesSlice) {
	rls.Sort(func(a, b pprofile.ResourceProfiles) bool {
		if a.SchemaUrl() != b.SchemaUrl() {
			return a.SchemaUrl() < b.SchemaUrl()
		}
		aAttrs := pdatautil.MapHash(a.Resource().Attributes())
		bAttrs := pdatautil.MapHash(b.Resource().Attributes())
		return bytes.Compare(aAttrs[:], bAttrs[:]) < 0
	})
}

// IgnoreScopeProfilesOrder is a CompareProfilesOption that ignores the order of instrumentation scope traces/metrics/profiles.
func IgnoreScopeProfilesOrder() CompareProfilesOption {
	return compareProfilesOptionFunc(func(expected, actual pprofile.Profiles) {
		sortScopeProfilesSlices(expected)
		sortScopeProfilesSlices(actual)
	})
}

func sortScopeProfilesSlices(ls pprofile.Profiles) {
	for i := 0; i < ls.ResourceProfiles().Len(); i++ {
		ls.ResourceProfiles().At(i).ScopeProfiles().Sort(func(a, b pprofile.ScopeProfiles) bool {
			if a.SchemaUrl() != b.SchemaUrl() {
				return a.SchemaUrl() < b.SchemaUrl()
			}
			if a.Scope().Name() != b.Scope().Name() {
				return a.Scope().Name() < b.Scope().Name()
			}
			return a.Scope().Version() < b.Scope().Version()
		})
	}
}

// IgnoreProfileRecordsOrder is a CompareProfilesOption that ignores the order of profile records.
func IgnoreProfileRecordsOrder() CompareProfilesOption {
	return compareProfilesOptionFunc(func(expected, actual pprofile.Profiles) {
		sortProfileRecordSlices(expected)
		sortProfileRecordSlices(actual)
	})
}

func sortProfileRecordSlices(ls pprofile.Profiles) {
	for i := 0; i < ls.ResourceProfiles().Len(); i++ {
		for j := 0; j < ls.ResourceProfiles().At(i).ScopeProfiles().Len(); j++ {
			ls.ResourceProfiles().At(i).ScopeProfiles().At(j).ProfileRecords().Sort(func(a, b pprofile.ProfileRecord) bool {
				if a.ObservedTimestamp() != b.ObservedTimestamp() {
					return a.ObservedTimestamp() < b.ObservedTimestamp()
				}
				if a.Timestamp() != b.Timestamp() {
					return a.Timestamp() < b.Timestamp()
				}
				if a.SeverityNumber() != b.SeverityNumber() {
					return a.SeverityNumber() < b.SeverityNumber()
				}
				if a.SeverityText() != b.SeverityText() {
					return a.SeverityText() < b.SeverityText()
				}
				at := a.TraceID()
				bt := b.TraceID()
				if !bytes.Equal(at[:], bt[:]) {
					return bytes.Compare(at[:], bt[:]) < 0
				}
				as := a.SpanID()
				bs := b.SpanID()
				if !bytes.Equal(as[:], bs[:]) {
					return bytes.Compare(as[:], bs[:]) < 0
				}
				aAttrs := pdatautil.MapHash(a.Attributes())
				bAttrs := pdatautil.MapHash(b.Attributes())
				if !bytes.Equal(aAttrs[:], bAttrs[:]) {
					return bytes.Compare(aAttrs[:], bAttrs[:]) < 0
				}
				ab := pdatautil.ValueHash(a.Body())
				bb := pdatautil.ValueHash(b.Body())
				return bytes.Compare(ab[:], bb[:]) < 0
			})
		}
	}
}
