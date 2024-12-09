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
	TestProfileEndTime        = time.Date(2020, 2, 11, 20, 28, 13, 789, time.UTC)
	TestProfileStartTimestamp = pcommon.NewTimestampFromTime(TestProfileStartTime)
	TestProfileEndTimestamp   = pcommon.NewTimestampFromTime(TestProfileEndTime)
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
	fillProfileOne(pd.ResourceProfiles().At(0).ScopeProfiles().At(0).Profiles().At(0))
	return pd
}

func GenerateProfilesTwoProfilesSameResource() pprofile.Profiles {
	pd := GenerateProfilesOneEmptyProfile()
	profiles := pd.ResourceProfiles().At(0).ScopeProfiles().At(0).Profiles()
	fillProfileOne(profiles.At(0))
	fillProfileTwo(profiles.AppendEmpty())
	return pd
}

func fillProfileOne(profile pprofile.Profile) {
	profile.SetStartTime(TestProfileStartTimestamp)
	profile.SetProfileID([16]byte{0x01, 0x02, 0x03, 0x04})

	attrs := profile.Attributes()
	attrs.PutStr("app", "server")
	attrs.PutInt("instance_num", 1)
}

func fillProfileTwo(profile pprofile.Profile) {
	profile.SetStartTime(TestProfileStartTimestamp)
	profile.SetProfileID([16]byte{0x05, 0x06, 0x07, 0x08})

	attrs := profile.Attributes()
	attrs.PutStr("customer", "acme")
	attrs.PutStr("env", "dev")
}
