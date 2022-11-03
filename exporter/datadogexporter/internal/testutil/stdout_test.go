// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package testutil // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/testutil"

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func TestReadAttributes(t *testing.T) {
	in := `     -> COMPANY: STRING(appsflyer)
     -> HOST_NAME: STRING(spotcluster-29025-143-prod.eu1.appsflyer.com)
     -> HOST_INDEX: INT(29025)
     -> ENABLED: BOOL(true)
     -> COST: DOUBLE(2.32)
     -> PROPS: MAP({"a":"x","b":"c"})
     -> LIST: SLICE(["1","2","3"])`
	scn := newScanner(strings.NewReader(in))
	mp := pcommon.NewMap()
	if err := readAttributes(scn, mp); err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, map[string]interface{}{
		"COMPANY":    "appsflyer",
		"HOST_INDEX": int64(29025),
		"HOST_NAME":  "spotcluster-29025-143-prod.eu1.appsflyer.com",
		"ENABLED":    true,
		"COST":       float64(2.32),
		"PROPS":      map[string]interface{}{"a": "x", "b": "c"},
		"LIST":       []interface{}{"1", "2", "3"},
	}, mp.AsRaw())
}

func TestReadNumberDataPoints(t *testing.T) {
	type datapoint struct {
		StartTimeNano uint64
		TimeNano      uint64
		Value         interface{} // float64 or int
		Attributes    map[string]interface{}
	}
	for _, tt := range []struct {
		in  string
		out []datapoint
	}{
		{
			in: `NumberDataPoints #0
Data point attributes:
     -> http_client_path: STRING(/v1/agent/service/register)
     -> http_client_status: STRING(200)
StartTimestamp: 1970-01-01 00:00:00 +0000 UTC
Timestamp: 2022-10-26 13:05:17.430579183 +0000 UTC
Value: 22.470483
NumberDataPoints #1
Data point attributes:
     -> http_client_path: STRING(/v1/kv/yas-agent/run/yas-143/medic)
     -> http_client_status: STRING(200)
StartTimestamp: 1970-01-01 00:00:00 +0000 UTC
Timestamp: 2022-10-26 13:05:17.436338284 +0000 UTC
Value: 1.133581
NumberDataPoints #2
Data point attributes:
     -> http_client_path: STRING(/v1/agent/services)
     -> http_client_status: STRING(200)
StartTimestamp: 1970-01-01 00:00:00 +0000 UTC
Timestamp: 2022-10-26 13:05:17.430555849 +0000 UTC
Value: 0.539633
NumberDataPoints #3
StartTimestamp: 1970-01-01 00:00:00 +0000 UTC
Timestamp: 2022-10-26 13:05:17.430555849 +0000 UTC
Value: 1
NumberDataPoints #4
Data point attributes:
     -> http_client_path: STRING(/v1/agent/service/yas-agent-yas-143)
     -> http_client_status: STRING(404)
StartTimestamp: 1970-01-01 00:00:00 +0000 UTC
Timestamp: 2022-10-26 13:05:17.430543597 +0000 UTC
Value: 2099.891909`,
			out: []datapoint{
				{
					Attributes: map[string]interface{}{
						"http_client_path":   "/v1/agent/service/register",
						"http_client_status": "200",
					},
					TimeNano: 0x1721a03c2dcb5fef,
					Value:    float64(22.470483),
				},
				{
					Attributes: map[string]interface{}{
						"http_client_path":   "/v1/kv/yas-agent/run/yas-143/medic",
						"http_client_status": "200",
					},
					TimeNano: 0x1721a03c2e23406c,
					Value:    float64(1.133581),
				},
				{
					Attributes: map[string]interface{}{
						"http_client_path":   "/v1/agent/services",
						"http_client_status": "200",
					},
					TimeNano: 0x1721a03c2dcb04c9,
					Value:    float64(0.539633),
				},
				{
					Attributes: map[string]interface{}{},
					TimeNano:   0x1721a03c2dcb04c9,
					Value:      int64(1),
				},
				{
					Attributes: map[string]interface{}{
						"http_client_path":   "/v1/agent/service/yas-agent-yas-143",
						"http_client_status": "404",
					},
					TimeNano: 0x1721a03c2dcad4ed,
					Value:    float64(2099.891909),
				},
			},
		},
	} {
		scn := newScanner(strings.NewReader(tt.in))
		dps := pmetric.NewNumberDataPointSlice()
		if err := readNumberDataPoints(scn, dps); err != nil {
			t.Fatal(err)
		}
		for i := 0; i < dps.Len(); i++ {
			dp := dps.At(i)
			dpo := tt.out[i]
			assert.Equal(t, dp.Timestamp(), pcommon.Timestamp(dpo.TimeNano))
			assert.Equal(t, dp.StartTimestamp(), pcommon.Timestamp(dpo.StartTimeNano))
			switch v := dpo.Value.(type) {
			case float64:
				assert.Equal(t, dp.DoubleValue(), v)
			case int, int64:
				assert.Equal(t, dp.IntValue(), v)
			default:
				t.Fatal("unknown value type")
			}
			assert.Equal(t, dp.Attributes().AsRaw(), dpo.Attributes)
		}
	}
}
