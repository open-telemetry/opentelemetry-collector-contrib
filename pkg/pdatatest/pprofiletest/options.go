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

// IgnoreResourceAttributeValue is a CompareProfilesOption that removes a resource attribute
// from all resources.
func IgnoreScopeAttributeValue(attributeName string) CompareProfilesOption {
	return ignoreScopeAttributeValue{
		attributeName: attributeName,
	}
}

type ignoreScopeAttributeValue struct {
	attributeName string
}

func (opt ignoreScopeAttributeValue) applyOnProfiles(expected, actual pprofile.Profiles) {
	opt.maskProfilesScopeAttributeValue(expected)
	opt.maskProfilesScopeAttributeValue(actual)
}

func (opt ignoreScopeAttributeValue) maskProfilesScopeAttributeValue(profiles pprofile.Profiles) {
	rls := profiles.ResourceProfiles()
	for i := 0; i < profiles.ResourceProfiles().Len(); i++ {
		sls := rls.At(i).ScopeProfiles()
		for j := 0; j < sls.Len(); j++ {
			lr := sls.At(j)
			val, exists := lr.Scope().Attributes().Get(opt.attributeName)
			if exists {
				val.SetEmptyBytes()
			}

		}
	}
}

// IgnoreProfileContainerAttributeValue is a CompareProfilesOption that sets the value of an attribute
// to empty bytes for every profile
func IgnoreProfileContainerAttributeValue(attributeName string) CompareProfilesOption {
	return ignoreProfileContainerAttributeValue{
		attributeName: attributeName,
	}
}

type ignoreProfileContainerAttributeValue struct {
	attributeName string
}

func (opt ignoreProfileContainerAttributeValue) applyOnProfiles(expected, actual pprofile.Profiles) {
	opt.maskProfileContainerAttributeValue(expected)
	opt.maskProfileContainerAttributeValue(actual)
}

func (opt ignoreProfileContainerAttributeValue) maskProfileContainerAttributeValue(profiles pprofile.Profiles) {
	rls := profiles.ResourceProfiles()
	for i := 0; i < profiles.ResourceProfiles().Len(); i++ {
		sls := rls.At(i).ScopeProfiles()
		for j := 0; j < sls.Len(); j++ {
			lrs := sls.At(j).Profiles()
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

// IgnoreProfileAttributeValue is a CompareProfilesOption that sets the value of an attribute
// to empty bytes for every profile
func IgnoreProfileAttributeValue(attributeName string) CompareProfilesOption {
	return ignoreProfileAttributeValue{
		attributeName: attributeName,
	}
}

type ignoreProfileAttributeValue struct {
	attributeName string
}

func (opt ignoreProfileAttributeValue) applyOnProfiles(expected, actual pprofile.Profiles) {
	opt.maskProfileAttributeValue(expected)
	opt.maskProfileAttributeValue(actual)
}

func (opt ignoreProfileAttributeValue) maskProfileAttributeValue(profiles pprofile.Profiles) {
	rls := profiles.ResourceProfiles()
	for i := 0; i < profiles.ResourceProfiles().Len(); i++ {
		sls := rls.At(i).ScopeProfiles()
		for j := 0; j < sls.Len(); j++ {
			lrs := sls.At(j).Profiles()
			for k := 0; k < lrs.Len(); k++ {
				lr := lrs.At(k).Profile()
				val, exists := lr.AttributeTable().Get(opt.attributeName)
				if exists {
					val.SetEmptyBytes()
				}
			}
		}
	}
}

// IgnoreProfileContainerTimestampValues is a CompareProfilesOption that sets the value of start and end timestamp
// to empty bytes for every profile
func IgnoreProfileContainerTimestampValues() CompareProfilesOption {
	return ignoreProfileContainerTimestampValues{}
}

type ignoreProfileContainerTimestampValues struct {
}

func (opt ignoreProfileContainerTimestampValues) applyOnProfiles(expected, actual pprofile.Profiles) {
	opt.maskProfileContainerTimestampValues(expected)
	opt.maskProfileContainerTimestampValues(actual)
}

func (opt ignoreProfileContainerTimestampValues) maskProfileContainerTimestampValues(profiles pprofile.Profiles) {
	rls := profiles.ResourceProfiles()
	for i := 0; i < profiles.ResourceProfiles().Len(); i++ {
		sls := rls.At(i).ScopeProfiles()
		for j := 0; j < sls.Len(); j++ {
			lrs := sls.At(j).Profiles()
			for k := 0; k < lrs.Len(); k++ {
				lr := lrs.At(k)
				lr.SetStartTime(pcommon.NewTimestampFromTime(time.Time{}))
				lr.SetEndTime(pcommon.NewTimestampFromTime(time.Time{}))
			}
		}
	}
}

// IgnoreProfileTimestampValues is a CompareProfilesOption that sets the value of start timestamp
// and duration to empty bytes for every profile
func IgnoreProfileTimestampValues() CompareProfilesOption {
	return ignoreProfileTimestampValues{}
}

type ignoreProfileTimestampValues struct {
}

func (opt ignoreProfileTimestampValues) applyOnProfiles(expected, actual pprofile.Profiles) {
	opt.maskProfileTimestampValues(expected)
	opt.maskProfileTimestampValues(actual)
}

func (opt ignoreProfileTimestampValues) maskProfileTimestampValues(profiles pprofile.Profiles) {
	rls := profiles.ResourceProfiles()
	for i := 0; i < profiles.ResourceProfiles().Len(); i++ {
		sls := rls.At(i).ScopeProfiles()
		for j := 0; j < sls.Len(); j++ {
			lrs := sls.At(j).Profiles()
			for k := 0; k < lrs.Len(); k++ {
				lr := lrs.At(k).Profile()
				lr.SetStartTime(pcommon.NewTimestampFromTime(time.Time{}))
				lr.SetDuration(pcommon.NewTimestampFromTime(time.Time{}))
			}
		}
	}
}

// IgnoreProfileContainerAttributesOrder is a CompareprofilesOption that ignores the order of profile container attributes.
func IgnoreProfileContainerAttributesOrder() CompareProfilesOption {
	return compareProfilesOptionFunc(func(expected, actual pprofile.Profiles) {
		orderProfileContainerAttributes(expected)
		orderProfileContainerAttributes(actual)
	})
}

func orderProfileContainerAttributes(metrics pprofile.Profiles) {
	rms := metrics.ResourceProfiles()
	for i := 0; i < rms.Len(); i++ {
		ilms := rms.At(i).ScopeProfiles()
		for j := 0; j < ilms.Len(); j++ {
			msl := ilms.At(j).Profiles()
			for g := 0; g < msl.Len(); g++ {
				for k := 0; k < msl.At(g).Attributes().Len(); k++ {
					rawOrdered := internal.OrderMapByKey(msl.At(g).Attributes().AsRaw())
					_ = msl.At(g).Attributes().FromRaw(rawOrdered)
				}
			}
		}
	}
}

// IgnoreProfileAttributesOrder is a CompareprofilesOption that ignores the order of profile attributes.
func IgnoreProfileAttributesOrder() CompareProfilesOption {
	return compareProfilesOptionFunc(func(expected, actual pprofile.Profiles) {
		orderProfileAttributes(expected)
		orderProfileAttributes(actual)
	})
}

func orderProfileAttributes(metrics pprofile.Profiles) {
	rms := metrics.ResourceProfiles()
	for i := 0; i < rms.Len(); i++ {
		ilms := rms.At(i).ScopeProfiles()
		for j := 0; j < ilms.Len(); j++ {
			msl := ilms.At(j).Profiles()
			for g := 0; g < msl.Len(); g++ {
				for k := 0; k < msl.At(g).Profile().AttributeTable().Len(); k++ {
					rawOrdered := internal.OrderMapByKey(msl.At(g).Profile().AttributeTable().AsRaw())
					_ = msl.At(g).Profile().AttributeTable().FromRaw(rawOrdered)
				}
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

// IgnoreProfilesOrder is a CompareProfilesOption that ignores the order of profile records.
func IgnoreProfileContainersOrder() CompareProfilesOption {
	return compareProfilesOptionFunc(func(expected, actual pprofile.Profiles) {
		sortProfileContainerSlices(expected)
		sortProfileContainerSlices(actual)
	})
}

func sortProfileContainerSlices(ls pprofile.Profiles) {
	for i := 0; i < ls.ResourceProfiles().Len(); i++ {
		for j := 0; j < ls.ResourceProfiles().At(i).ScopeProfiles().Len(); j++ {
			ls.ResourceProfiles().At(i).ScopeProfiles().At(j).Profiles().Sort(func(a, b pprofile.ProfileContainer) bool {
				if a.StartTime() != b.StartTime() {
					return a.StartTime() < b.StartTime()
				}
				if a.EndTime() != b.EndTime() {
					return a.EndTime() < b.EndTime()
				}
				as := a.ProfileID()
				bs := b.ProfileID()
				if !bytes.Equal(as[:], bs[:]) {
					return bytes.Compare(as[:], bs[:]) < 0
				}
				aAttrs := pdatautil.MapHash(a.Attributes())
				bAttrs := pdatautil.MapHash(b.Attributes())
				return bytes.Compare(aAttrs[:], bAttrs[:]) < 0
			})
		}
	}
}
