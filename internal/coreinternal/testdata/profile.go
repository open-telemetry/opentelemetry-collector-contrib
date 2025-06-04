// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package testdata

import (
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pprofile"
)

var (
	TestProfileStartTime      = time.Date(2020, 2, 11, 20, 26, 13, 789, time.UTC)
	TestProfileStartTimestamp = pcommon.NewTimestampFromTime(TestProfileStartTime)
)

func GenerateProfilesOneEmptyResourceProfiles() pprofile.Profiles {
	pd := pprofile.NewProfiles()
	pd.ResourceProfiles().AppendEmpty()
	return pd
}

func GenerateProfilesNoProfiles() pprofile.Profiles {
	pd := GenerateProfilesOneEmptyResourceProfiles()
	initResource1(pd.ResourceProfiles().At(0).Resource())
	return pd
}

func GenerateProfilesOneEmptyProfile() pprofile.Profiles {
	pd := GenerateProfilesNoProfiles()
	rs0 := pd.ResourceProfiles().At(0)
	rs0.ScopeProfiles().AppendEmpty().Profiles().AppendEmpty()
	return pd
}

func GenerateProfilesOneProfile() pprofile.Profiles {
	pd := GenerateProfilesOneEmptyProfile()
	fillProfileOne(pd.ProfilesDictionary(), pd.ResourceProfiles().At(0).ScopeProfiles().At(0).Profiles().At(0))
	return pd
}

func GenerateProfilesTwoProfilesSameResource() pprofile.Profiles {
	pd := GenerateProfilesOneEmptyProfile()
	profiles := pd.ResourceProfiles().At(0).ScopeProfiles().At(0).Profiles()
	fillProfileOne(pd.ProfilesDictionary(), profiles.At(0))
	fillProfileTwo(pd.ProfilesDictionary(), profiles.AppendEmpty())
	return pd
}

func fillProfileOne(dic pprofile.ProfilesDictionary, profile pprofile.Profile) {
	profile.SetStartTime(TestProfileStartTimestamp)
	profile.SetProfileID([16]byte{0x01, 0x02, 0x03, 0x04})

	profile.AttributeIndices().Append(0)
	a := dic.AttributeTable().AppendEmpty()
	a.SetKey("app")
	a.Value().SetStr("server")
	profile.AttributeIndices().Append(0)
	a = dic.AttributeTable().AppendEmpty()
	a.SetKey("instance_num")
	a.Value().SetInt(1)
}

func fillProfileTwo(dic pprofile.ProfilesDictionary, profile pprofile.Profile) {
	profile.SetStartTime(TestProfileStartTimestamp)
	profile.SetProfileID([16]byte{0x05, 0x06, 0x07, 0x08})

	profile.AttributeIndices().Append(0)
	a := dic.AttributeTable().AppendEmpty()
	a.SetKey("customer")
	a.Value().SetStr("acme")
	profile.AttributeIndices().Append(0)
	a = dic.AttributeTable().AppendEmpty()
	a.SetKey("env")
	a.Value().SetStr("dev")
}
