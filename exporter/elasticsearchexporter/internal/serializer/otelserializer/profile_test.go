// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otelserializer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pprofile"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/serializer/otelserializer/serializeprofiles"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pprofiletest"
)

func basicProfiles() pprofiletest.Profiles {
	r := pcommon.NewResource()
	r.Attributes().PutStr("key1", "value1")
	return pprofiletest.Profiles{
		ResourceProfiles: []pprofiletest.ResourceProfile{
			{
				Resource: r,
				ScopeProfiles: []pprofiletest.ScopeProfile{
					{
						Scope: pcommon.NewInstrumentationScope(),
						Profiles: []pprofiletest.Profile{
							{
								SampleType: pprofiletest.ValueType{Typ: "samples", Unit: "count"},
								PeriodType: pprofiletest.ValueType{Typ: "cpu", Unit: "nanoseconds"},
								Attributes: []pprofiletest.Attribute{
									{Key: "process.executable.build_id.htlhash", Value: "600DCAFE4A110000F2BF38C493F5FB92"},
									{Key: "profile.frame.type", Value: "native"},
									{Key: "host.id", Value: "localhost"},
								},
								Sample: []pprofiletest.Sample{
									{
										TimestampsUnixNano: []uint64{0},
										Values:             []int64{1},
										Locations: []pprofiletest.Location{
											{
												Mapping: &pprofiletest.Mapping{},
												Address: 111,
											},
										},
									},
								},
								ProfileID: pprofile.NewProfileIDEmpty(),
							},
						},
					},
				},
			},
		},
	}
}

func TestSerializeProfile(t *testing.T) {
	tests := []struct {
		name              string
		buildDictionary   func() pprofile.ProfilesDictionary
		profileCustomizer func(resource pcommon.Resource, scope pcommon.InstrumentationScope, record pprofile.Profile)
		wantErr           bool
		expected          []map[string]any
	}{
		{
			name: "with a simple sample",
			buildDictionary: func() pprofile.ProfilesDictionary {
				dic := pprofile.NewProfilesDictionary()
				dic.StringTable().Append("samples", "count", "cpu", "nanoseconds")

				a := dic.AttributeTable().AppendEmpty()
				a.SetKeyStrindex(4)
				dic.StringTable().Append("process.executable.build_id.htlhash")
				a.Value().SetStr("600DCAFE4A110000F2BF38C493F5FB92")
				a = dic.AttributeTable().AppendEmpty()
				a.SetKeyStrindex(5)
				dic.StringTable().Append("profile.frame.type")
				a.Value().SetStr("native")
				a = dic.AttributeTable().AppendEmpty()
				a.SetKeyStrindex(6)
				dic.StringTable().Append("host.id")
				a.Value().SetStr("localhost")

				dic.MappingTable().AppendEmpty()
				m := dic.MappingTable().AppendEmpty()
				m.AttributeIndices().Append(0)

				l := dic.LocationTable().AppendEmpty()
				l.SetMappingIndex(1)
				l.SetAddress(111)
				l.AttributeIndices().Append(1)

				stack := dic.StackTable().AppendEmpty()
				stack.LocationIndices().Append(0)

				return dic
			},
			profileCustomizer: func(r pcommon.Resource, _ pcommon.InstrumentationScope, profile pprofile.Profile) {
				st := profile.SampleType()
				st.SetTypeStrindex(0)
				st.SetUnitStrindex(1)
				pt := profile.PeriodType()
				pt.SetTypeStrindex(2)
				pt.SetUnitStrindex(3)
				profile.SetPeriod(1e9 / 20)

				profile.AttributeIndices().Append(2)

				sample := profile.Samples().AppendEmpty()
				sample.TimestampsUnixNano().Append(0)
				sample.AttributeIndices().Append(2)
				sample.SetStackIndex(0)

				r.Attributes().PutStr("process.executable.name", "libc.so.6")
			},
			wantErr: false,
			expected: []map[string]any{
				{
					"@timestamp":  "1970-01-01T00:00:00Z",
					"frame.ids":   "YA3K_koRAADyvzjEk_X7kgAAAAAAAABv",
					"frame.types": "AQM",
				},
				{
					"@timestamp":         "1970-01-01T00:00:00Z",
					"count":              json.Number("1"),
					"sampling_frequency": json.Number("20"),
					"stacktrace.id":      "02VzuClbpt_P3xxwox83Ng",
					"resource": map[string]any{
						"attributes": map[string]any{
							"host.id":                 "localhost",
							"process.executable.name": "libc.so.6",
							"thread.name":             "",
						},
					},
				},
				{
					"@timestamp": json.Number(fmt.Sprintf("%d", serializeprofiles.GetStartOfWeekFromTime(time.Now()))),
					"resource": map[string]any{
						"attributes": map[string]any{
							"process.executable.build_id.htlhash": "600DCAFE4A110000F2BF38C493F5FB92",
							"process.executable.name":             "samples",
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dic := tt.buildDictionary()
			profiles := pprofile.NewProfiles()
			resource := profiles.ResourceProfiles().AppendEmpty()
			scope := resource.ScopeProfiles().AppendEmpty()
			profile := scope.Profiles().AppendEmpty()
			tt.profileCustomizer(resource.Resource(), scope.Scope(), profile)
			profiles.MarkReadOnly()

			buf := []*bytes.Buffer{}
			ser, err := New()
			require.NoError(t, err)
			err = ser.SerializeProfile(dic, resource.Resource(), scope.Scope(), profile, func(b *bytes.Buffer, _, _ string) error {
				buf = append(buf, b)
				return nil
			})
			if !tt.wantErr {
				require.NoError(t, err)
			}

			var results []map[string]any
			for _, v := range buf {
				var d map[string]any
				decoder := json.NewDecoder(v)
				decoder.UseNumber()
				require.NoError(t, decoder.Decode(&d))
				results = append(results, d)
			}

			assert.Equal(t, tt.expected, results)
		})
	}
}

func BenchmarkSerializeProfile(b *testing.B) {
	ser, err := New()
	require.NoError(b, err)

	profiles := basicProfiles().Transform()
	resource := profiles.ResourceProfiles().At(0)
	scope := resource.ScopeProfiles().At(0)
	profile := scope.Profiles().At(0)
	pushData := func(_ *bytes.Buffer, _, _ string) error {
		return nil
	}

	b.ReportAllocs()

	for b.Loop() {
		_ = ser.SerializeProfile(profiles.Dictionary(), resource.Resource(), scope.Scope(), profile, pushData)
	}
}
