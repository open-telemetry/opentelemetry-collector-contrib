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
)

func TestSerializeProfile(t *testing.T) {
	tests := []struct {
		name              string
		profileCustomizer func(resource pcommon.Resource, scope pcommon.InstrumentationScope, record pprofile.Profile)
		wantErr           bool
		expected          []map[string]any
	}{
		{
			name: "with a simple sample",
			profileCustomizer: func(_ pcommon.Resource, _ pcommon.InstrumentationScope, profile pprofile.Profile) {
				profile.StringTable().Append("samples", "count", "cpu", "nanoseconds")
				st := profile.SampleType().AppendEmpty()
				st.SetTypeStrindex(0)
				st.SetUnitStrindex(1)
				pt := profile.PeriodType()
				pt.SetTypeStrindex(2)
				pt.SetUnitStrindex(3)

				a := profile.AttributeTable().AppendEmpty()
				a.SetKey("process.executable.build_id.htlhash")
				a.Value().SetStr("600DCAFE4A110000F2BF38C493F5FB92")
				a = profile.AttributeTable().AppendEmpty()
				a.SetKey("profile.frame.type")
				a.Value().SetStr("native")
				a = profile.AttributeTable().AppendEmpty()
				a.SetKey("host.id")
				a.Value().SetStr("localhost")

				profile.AttributeIndices().Append(2)

				sample := profile.Sample().AppendEmpty()
				sample.TimestampsUnixNano().Append(0)
				sample.SetLocationsLength(1)

				m := profile.MappingTable().AppendEmpty()
				m.AttributeIndices().Append(0)

				l := profile.LocationTable().AppendEmpty()
				l.SetMappingIndex(0)
				l.SetAddress(111)
				l.AttributeIndices().Append(1)
			},
			wantErr: false,
			expected: []map[string]any{
				{
					"Stacktrace.frame.ids":   "YA3K_koRAADyvzjEk_X7kgAAAAAAAABv",
					"Stacktrace.frame.types": "AQM",
					"ecs.version":            "1.12.0",
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
					"@timestamp":          "1970-01-01T00:00:00Z",
					"Stacktrace.count":    json.Number("1"),
					"Stacktrace.id":       "02VzuClbpt_P3xxwox83Ng",
					"ecs.version":         "1.12.0",
					"host.id":             "localhost",
					"process.thread.name": "",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			profiles := pprofile.NewProfiles()
			resource := profiles.ResourceProfiles().AppendEmpty()
			scope := resource.ScopeProfiles().AppendEmpty()
			profile := scope.Profiles().AppendEmpty()
			tt.profileCustomizer(resource.Resource(), scope.Scope(), profile)
			profiles.MarkReadOnly()

			buf := []*bytes.Buffer{}
			err := SerializeProfile(resource.Resource(), scope.Scope(), profile, func(b *bytes.Buffer, _ string, _ string) error {
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
				err := decoder.Decode(&d)

				require.NoError(t, err)
				results = append(results, d)
			}

			assert.Equal(t, tt.expected, results)
		})
	}
}
