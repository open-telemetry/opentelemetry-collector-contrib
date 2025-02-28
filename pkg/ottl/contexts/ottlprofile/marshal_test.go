// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlprofile

import (
	"testing"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pprofile"
	"go.uber.org/zap"
)

func TestTransformContext_MarshalLogObject(_ *testing.T) {
	instrumentationScope := pcommon.NewInstrumentationScope()
	cache := pcommon.NewMap()

	logger := zap.NewExample()
	defer func() { _ = logger.Sync() }()

	rps := GenerateProfiles(1).ResourceProfiles()
	for i := range rps.Len() {
		rp := rps.At(i)
		resource := rp.Resource()
		sps := rp.ScopeProfiles()
		for j := range sps.Len() {
			sp := sps.At(j)
			profiles := sp.Profiles()
			for k := range profiles.Len() {
				profile := profiles.At(k)
				ctx := NewTransformContext(profile, instrumentationScope, resource, pprofile.NewScopeProfiles(), pprofile.NewResourceProfiles(), WithCache(&cache))
				logger.Info("test", zap.Object("context", ctx))
			}
		}
	}
}

var (
	profileStartTimestamp = pcommon.NewTimestampFromTime(time.Date(2020, 2, 11, 20, 26, 12, 321, time.UTC))
	profileEndTimestamp   = pcommon.NewTimestampFromTime(time.Date(2020, 2, 11, 20, 26, 13, 789, time.UTC))
)

// GenerateProfiles generates dummy profiling data for tests
func GenerateProfiles(profilesCount int) pprofile.Profiles {
	td := pprofile.NewProfiles()
	initResource(td.ResourceProfiles().AppendEmpty().Resource())
	ss := td.ResourceProfiles().At(0).ScopeProfiles().AppendEmpty().Profiles()
	ss.EnsureCapacity(profilesCount)
	for i := range profilesCount {
		switch i % 2 {
		case 0:
			fillProfileOne(ss.AppendEmpty())
		case 1:
			fillProfileTwo(ss.AppendEmpty())
		}
	}
	return td
}

func initResource(r pcommon.Resource) {
	r.Attributes().PutStr("resource-attr", "resource-attr-val-1")
}

func fillProfileOne(profile pprofile.Profile) {
	profile.SetProfileID([16]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F, 0x10})
	profile.SetTime(profileStartTimestamp)
	profile.SetDuration(profileEndTimestamp)
	profile.SetDroppedAttributesCount(1)

	profile.StringTable().Append("typeValue", "unitValue")

	st := profile.SampleType().AppendEmpty()
	st.SetTypeStrindex(0)
	st.SetUnitStrindex(1)
	st.SetAggregationTemporality(pprofile.AggregationTemporalityCumulative)

	attr := profile.AttributeTable().AppendEmpty()
	attr.SetKey("attry1key")
	attr.Value().SetStr("attr2value")

	profile.LocationTable().AppendEmpty()
	profile.LocationTable().AppendEmpty()
	profile.LocationTable().AppendEmpty()

	sample := profile.Sample().AppendEmpty()
	sample.SetLocationsStartIndex(1)
	sample.SetLocationsLength(2)
	sample.Value().Append(4)
	sample.AttributeIndices().Append(0)
}

func fillProfileTwo(profile pprofile.Profile) {
	profile.SetProfileID([16]byte{0x02, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F, 0x10})
	profile.SetTime(profileStartTimestamp)
	profile.SetDuration(profileEndTimestamp)

	attr := profile.AttributeTable().AppendEmpty()
	attr.SetKey("key")
	attr.Value().SetStr("value")

	sample := profile.Sample().AppendEmpty()
	sample.SetLocationsStartIndex(7)
	sample.SetLocationsLength(20)
	sample.Value().Append(9)
	sample.AttributeIndices().Append(0)
}
