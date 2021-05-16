// Copyright 2021, OpenTelemetry Authors
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

package datadogreceiver

import (
	"bytes"
	"net/http"
	"testing"

	"github.com/DataDog/datadog-agent/pkg/trace/exportable/pb"
	"github.com/stretchr/testify/assert"
	vmsgp "github.com/vmihailenco/msgpack/v4"
)

func BenchmarkTranslator(b *testing.B) {
	var data = [2]interface{}{
		0: []string{
			0:  "baggage",
			1:  "item",
			2:  "elasticsearch.version",
			3:  "7.0",
			4:  "my-name",
			5:  "X",
			6:  "my-service",
			7:  "my-resource",
			8:  "_dd.sampling_rate_whatever",
			9:  "value whatever",
			10: "sql",
		},
		1: [][][12]interface{}{
			{
				{
					6,
					4,
					7,
					uint64(12345678901234561234),
					uint64(2),
					uint64(3),
					int64(123),
					int64(456),
					1,
					map[interface{}]interface{}{
						8: 9,
						0: 1,
						2: 3,
					},
					map[interface{}]float64{
						5: 1.2,
					},
					10,
				},
			},
		},
	}

	payload, err := vmsgp.Marshal(&data)
	assert.NoError(b, err)
	dc := pb.NewMsgpReader(bytes.NewReader(payload))
	defer pb.FreeMsgpReader(dc)

	var traces pb.Traces
	if err := traces.DecodeMsgDictionary(dc); err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	b.ReportAllocs()
	b.SetBytes(int64(len(payload)))
	req := &http.Request{RequestURI: "/v0.5/traces"}
	for i := 0; i < b.N; i++ {
		ToTraces(traces, req)
	}
}
