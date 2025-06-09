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
	return pprofiletest.Profiles{
		ResourceProfiles: []pprofiletest.ResourceProfile{
			{
				Resource: pprofiletest.Resource{
					Attributes: []pprofiletest.Attribute{
						{Key: "key1", Value: "value1"},
					},
				},
				ScopeProfiles: []pprofiletest.ScopeProfile{
					{
						Profile: []pprofiletest.Profile{
							{
								SampleType: []pprofiletest.ValueType{
									{Typ: "samples", Unit: "count"},
								},
								PeriodType: pprofiletest.ValueType{Typ: "cpu", Unit: "nanoseconds"},
								Attributes: []pprofiletest.Attribute{
									{Key: "process.executable.build_id.htlhash", Value: "600DCAFE4A110000F2BF38C493F5FB92"},
									{Key: "profile.frame.type", Value: "native"},
									{Key: "host.id", Value: "localhost"},
								},
								Sample: []pprofiletest.Sample{
									{
										TimestampsUnixNano: []uint64{0},
										Value:              []int64{1},
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
				a.SetKey("process.executable.build_id.htlhash")
				a.Value().SetStr("600DCAFE4A110000F2BF38C493F5FB92")
				a = dic.AttributeTable().AppendEmpty()
				a.SetKey("profile.frame.type")
				a.Value().SetStr("native")
				a = dic.AttributeTable().AppendEmpty()
				a.SetKey("host.id")
				a.Value().SetStr("localhost")

				m := dic.MappingTable().AppendEmpty()
				m.AttributeIndices().Append(0)

				l := dic.LocationTable().AppendEmpty()
				l.SetMappingIndex(0)
				l.SetAddress(111)
				l.AttributeIndices().Append(1)

				return dic
			},
			profileCustomizer: func(_ pcommon.Resource, _ pcommon.InstrumentationScope, profile pprofile.Profile) {
				st := profile.SampleType().AppendEmpty()
				st.SetTypeStrindex(0)
				st.SetUnitStrindex(1)
				pt := profile.PeriodType()
				pt.SetTypeStrindex(2)
				pt.SetUnitStrindex(3)

				profile.AttributeIndices().Append(2)

				sample := profile.Sample().AppendEmpty()
				sample.TimestampsUnixNano().Append(0)
				sample.SetLocationsLength(1)
			},
			wantErr: false,
			expected: []map[string]any{
				{
					"Stacktrace.frame.ids":   "YA3K_koRAADyvzjEk_X7kgAAAAAAAABv",
					"Stacktrace.frame.types": "AQM",
					"ecs.version":            "1.12.0",
				},
				{
					"@timestamp":          "1970-01-01T00:00:00Z",
					"Stacktrace.count":    json.Number("1"),
					"Stacktrace.id":       "02VzuClbpt_P3xxwox83Ng",
					"ecs.version":         "1.12.0",
					"host.id":             "localhost",
					"process.thread.name": "",
				},
				{
					"script": map[string]any{
						"params": map[string]any{
							"buildid":    "YA3K_koRAADyvzjEk_X7kg",
							"ecsversion": "1.12.0",
							"filename":   "samples",
							"timestamp":  json.Number(fmt.Sprintf("%d", serializeprofiles.GetStartOfWeekFromTime(time.Now()))),
						},
						"source": serializeprofiles.ExeMetadataUpsertScript,
					},
					"scripted_upsert": true,
					"upsert":          map[string]any{},
				},
				{
					"Stacktrace.frame.id":     []any{"YA3K_koRAADyvzjEk_X7kgAAAAAAAABv"},
					"Symbolization.retries":   json.Number("0"),
					"Symbolization.time.next": "",
					"Time.created":            "",
					"ecs.version":             serializeprofiles.EcsVersionString,
				},
				{
					"Executable.file.id":      []any{"YA3K_koRAADyvzjEk_X7kg"},
					"Symbolization.retries":   json.Number("0"),
					"Symbolization.time.next": "",
					"Time.created":            "",
					"ecs.version":             serializeprofiles.EcsVersionString,
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
			err = ser.SerializeProfile(dic, resource.Resource(), scope.Scope(), profile, func(b *bytes.Buffer, _ string, _ string) error {
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

				// Remove timestamps to allow comparing test results with expected values.
				for k, v := range d {
					switch k {
					case "Symbolization.time.next", "Time.created":
						tm, err := time.Parse(time.RFC3339Nano, v.(string))
						require.NoError(t, err)
						assert.True(t, isWithinLastSecond(tm))
						d[k] = ""
					}
				}
				results = append(results, d)
			}

			assert.Equal(t, tt.expected, results)
		})
	}
}

func isWithinLastSecond(t time.Time) bool {
	return time.Since(t) < time.Second
}

func BenchmarkSerializeProfile(b *testing.B) {
	ser, err := New()
	require.NoError(b, err)

	profiles := basicProfiles().Transform()
	resource := profiles.ResourceProfiles().At(0)
	scope := resource.ScopeProfiles().At(0)
	profile := scope.Profiles().At(0)
	pushData := func(_ *bytes.Buffer, _ string, _ string) error {
		return nil
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = ser.SerializeProfile(profiles.ProfilesDictionary(), resource.Resource(), scope.Scope(), profile, pushData)
	}
}
