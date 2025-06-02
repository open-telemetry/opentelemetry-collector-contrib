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
	dic := profiles.ProfilesDictionary()
	for l := 0; l < dic.AttributeTable().Len(); l++ {
		a := dic.AttributeTable().At(l)
		if a.Key() == opt.attributeName {
			a.Value().SetEmptyBytes()
		}
	}
}

// IgnoreProfileTimestampValues is a CompareProfilesOption that sets the value of start timestamp
// and duration to empty bytes for every profile
func IgnoreProfileTimestampValues() CompareProfilesOption {
	return ignoreProfileTimestampValues{}
}

type ignoreProfileTimestampValues struct{}

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
				lr := lrs.At(k)
				lr.SetStartTime(pcommon.NewTimestampFromTime(time.Time{}))
				lr.SetDuration(pcommon.NewTimestampFromTime(time.Time{}))
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
func IgnoreProfilesOrder() CompareProfilesOption {
	return compareProfilesOptionFunc(func(expected, actual pprofile.Profiles) {
		sortProfileSlices(expected)
		sortProfileSlices(actual)
	})
}

func sortProfileSlices(ls pprofile.Profiles) {
	for i := 0; i < ls.ResourceProfiles().Len(); i++ {
		for j := 0; j < ls.ResourceProfiles().At(i).ScopeProfiles().Len(); j++ {
			ls.ResourceProfiles().At(i).ScopeProfiles().At(j).Profiles().Sort(func(a, b pprofile.Profile) bool {
				if a.StartTime() != b.StartTime() {
					return a.StartTime() < b.StartTime()
				}
				if a.Duration() != b.Duration() {
					return a.Duration() < b.Duration()
				}
				as := a.ProfileID()
				bs := b.ProfileID()
				return bytes.Compare(as[:], bs[:]) < 0
			})
		}
	}
}

func profileAttributesToMap(dic pprofile.ProfilesDictionary, p pprofile.Profile) map[string]string {
	d := map[string]string{}
	for _, i := range p.AttributeIndices().AsRaw() {
		v := dic.AttributeTable().At(int(i))
		d[v.Key()] = v.Value().AsString()
	}

	return d
}
