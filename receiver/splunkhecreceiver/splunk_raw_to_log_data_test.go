// Copyright 2020, OpenTelemetry Authors
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

package splunkhecreceiver

import (
	"bufio"
	"bytes"
	"io"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/splunk"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
)

func Test_SplunkHecRawToLogData(t *testing.T) {
	hecConfig := &splunk.HecToOtelAttrs{
		Source:     "mysource",
		SourceType: "mysourcetype",
		Index:      "myindex",
		Host:       "myhost",
	}
	tests := []struct {
		name           string
		sc             *bufio.Scanner
		query          map[string][]string
		assertResource func(t *testing.T, got plog.Logs, slLen int)
	}{
		{
			name: "all_mapping",
			sc: func() *bufio.Scanner {
				reader := io.NopCloser(bytes.NewReader([]byte("test")))
				return bufio.NewScanner(reader)
			}(),
			query: func() map[string][]string {
				m := make(map[string][]string)
				k := []string{"foo"}
				m[host] = k
				m[sourcetype] = k
				m[source] = k
				m[index] = k
				return m
			}(),
			assertResource: func(t *testing.T, got plog.Logs, slLen int) {
				assert.Equal(t, 1, slLen)
				attrs := got.ResourceLogs().At(0).Resource().Attributes()
				assert.Equal(t, 4, attrs.Len())
				if v, ok := attrs.Get("myhost"); ok {
					assert.Equal(t, "foo", v.AsString())
				} else {
					assert.Fail(t, "host is not added to attributes")
				}
				if v, ok := attrs.Get("mysourcetype"); ok {
					assert.Equal(t, "foo", v.AsString())
				} else {
					assert.Fail(t, "sourcetype is not added to attributes")
				}
				if v, ok := attrs.Get("mysource"); ok {
					assert.Equal(t, "foo", v.AsString())
				} else {
					assert.Fail(t, "source is not added to attributes")
				}
				if v, ok := attrs.Get("myindex"); ok {
					assert.Equal(t, "foo", v.AsString())
				} else {
					assert.Fail(t, "index is not added to attributes")
				}
			},
		},
		{
			name: "some_mapping",
			sc: func() *bufio.Scanner {
				reader := io.NopCloser(bytes.NewReader([]byte("test")))
				return bufio.NewScanner(reader)
			}(),
			query: func() map[string][]string {
				m := make(map[string][]string)
				k := []string{"foo"}
				m[host] = k
				m[sourcetype] = k
				return m
			}(),
			assertResource: func(t *testing.T, got plog.Logs, slLen int) {
				assert.Equal(t, 1, slLen)
				attrs := got.ResourceLogs().At(0).Resource().Attributes()
				assert.Equal(t, 2, attrs.Len())
				if v, ok := attrs.Get("myhost"); ok {
					assert.Equal(t, "foo", v.AsString())
				} else {
					assert.Fail(t, "host is not added to attributes")
				}
				if v, ok := attrs.Get("mysourcetype"); ok {
					assert.Equal(t, "foo", v.AsString())
				} else {
					assert.Fail(t, "sourcetype is not added to attributes")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, slLen := splunkHecRawToLogData(tt.sc, tt.query, func(resource pcommon.Resource) {}, hecConfig)
			tt.assertResource(t, result, slLen)
		})
	}
}
