// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package azureeventhubreceiver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"

	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

func Test_newValueFromRaw(t *testing.T) {
	tests := []struct {
		name        string
		value       interface{}
		errExpected bool
	}{
		{
			name:        "nil",
			value:       nil,
			errExpected: false,
		},
		{
			name:        "string",
			value:       "foo",
			errExpected: false,
		},
		{
			name:        "int",
			value:       1234,
			errExpected: false,
		},
		{
			name:        "int8",
			value:       int8(12),
			errExpected: false,
		},
		{
			name:        "int16",
			value:       int16(123),
			errExpected: false,
		},
		{
			name:        "int32",
			value:       int32(123),
			errExpected: false,
		},
		{
			name:        "int64",
			value:       int64(123),
			errExpected: false,
		},
		{
			name:        "uint",
			value:       uint(123),
			errExpected: false,
		},
		{
			name:        "uint8",
			value:       uint8(12),
			errExpected: false,
		},
		{
			name:        "uint16",
			value:       uint16(1233),
			errExpected: false,
		},
		{
			name:        "uint32",
			value:       uint32(1233),
			errExpected: false,
		},
		{
			name:        "uint64",
			value:       uint64(1233),
			errExpected: false,
		},
		{
			name:        "float32",
			value:       float32(1233.4),
			errExpected: false,
		},
		{
			name:        "float64",
			value:       float64(1233.4),
			errExpected: false,
		},
		{
			name:        "bool",
			value:       true,
			errExpected: false,
		},
		{
			name:        "[]byte",
			value:       []byte("foo"),
			errExpected: false,
		},
		{
			name:        "map[string]interface{}",
			value:       map[string]interface{}{"foo": "bar"},
			errExpected: false,
		},
		{
			name:        "[]interface{}",
			value:       []interface{}{"foo"},
			errExpected: false,
		},
		{
			name:        "unsupported",
			value:       &Config{},
			errExpected: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			v, err := newValueFromRaw(test.value)
			if test.errExpected {
				assert.Error(t, err)
				assert.Equal(t, pcommon.NewValueEmpty(), v)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, v)
			}
		})
	}
}

func TestClient_Start(t *testing.T) {
	config := createDefaultConfig()
	config.(*Config).Connection = "Endpoint=sb://namespace.servicebus.windows.net/;SharedAccessKeyName=RootManageSharedAccessKey;SharedAccessKey=superSecret1234=;EntityPath=hubName"

	c := &client{
		logger:   zap.NewNop(),
		consumer: consumertest.NewNop(),
		config:   config.(*Config),
	}
	err := c.Start(context.Background(), componenttest.NewNopHost())
	assert.NoError(t, err)
}
