// Copyright The OpenTelemetry Authors
// Copyright (c) 2018 The Jaeger Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package tracegen

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestKeyValueSet(t *testing.T) {
	tests := []struct {
		flag     string
		expected KeyValue
		err      error
	}{
		{
			flag:     "key=\"value\"",
			expected: KeyValue(map[string]string{"key": "value"}),
		},
		{
			flag:     "key=\"\"",
			expected: KeyValue(map[string]string{"key": ""}),
		},
		{
			flag: "key=\"",
			err:  errDoubleQuotesOTLPAttributes,
		},
		{
			flag: "key=value",
			err:  errDoubleQuotesOTLPAttributes,
		},
		{
			flag: "key",
			err:  errFormatOTLPAttributes,
		},
	}

	for _, tt := range tests {
		t.Run(tt.flag, func(t *testing.T) {
			kv := KeyValue(make(map[string]string))
			err := kv.Set(tt.flag)
			if err != nil || tt.err != nil {
				assert.Equal(t, err, tt.err)
			} else {
				assert.Equal(t, tt.expected, kv)
			}
		})
	}
}
