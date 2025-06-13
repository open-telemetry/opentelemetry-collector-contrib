// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package serializeprofiles

import (
	"testing"

	"go.opentelemetry.io/collector/pdata/pprofile"
)

func BenchmarkTransform(b *testing.B) {
	for _, bb := range []struct {
		name                  string
		buildResourceProfiles func() pprofile.ResourceProfiles
	}{
		{
			name: "with a basic recorded sample",
			buildResourceProfiles: func() pprofile.ResourceProfiles {
				rp := pprofile.NewResourceProfiles()

				sp := rp.ScopeProfiles().AppendEmpty()
				p := sp.Profiles().AppendEmpty()

				a := p.AttributeTable().AppendEmpty()
				a.SetKey("profile.frame.type")
				a.Value().SetStr("native")
				a = p.AttributeTable().AppendEmpty()
				a.SetKey("process.executable.build_id.htlhash")
				a.Value().SetStr(buildIDEncoded)
				a = p.AttributeTable().AppendEmpty()
				a.SetKey("process.executable.build_id.htlhash")
				a.Value().SetStr(buildID2Encoded)

				p.StringTable().Append("firefox", "libc.so", "samples", "count", "cpu", "nanoseconds")
				st := p.SampleType().AppendEmpty()
				st.SetTypeStrindex(2)
				st.SetUnitStrindex(3)
				pt := p.PeriodType()
				pt.SetTypeStrindex(4)
				pt.SetUnitStrindex(5)

				m := p.MappingTable().AppendEmpty()
				m.AttributeIndices().Append(1)
				m.SetFilenameStrindex(0)
				m = p.MappingTable().AppendEmpty()
				m.AttributeIndices().Append(2)
				m.SetFilenameStrindex(1)

				l := p.LocationTable().AppendEmpty()
				l.SetAddress(address)
				l.AttributeIndices().Append(0)
				l.SetMappingIndex(0)
				l = p.LocationTable().AppendEmpty()
				l.SetAddress(address2)
				l.AttributeIndices().Append(0)
				l.SetMappingIndex(1)

				s := p.Sample().AppendEmpty()
				s.TimestampsUnixNano().Append(42)
				s.Value().Append(1)
				s.SetLocationsLength(2)
				s.SetLocationsStartIndex(0)

				return rp
			},
		},
	} {
		b.Run(bb.name, func(b *testing.B) {
			rp := bb.buildResourceProfiles()
			sp := rp.ScopeProfiles().At(0)
			p := sp.Profiles().At(0)

			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				_, _ = Transform(rp.Resource(), sp.Scope(), p)
			}
		})
	}
}
