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

package joinattrprocessor

import (
	"context"
	"testing"

	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/stretchr/testify/assert"
)

// All the data we need to define a test
type testConfig struct {
	name            string
	processorConfig *Config
	spanAttributes  pcommon.Map
	expected        string
	isError         bool
}

var (
	validateConfigTests = []testConfig{
		{
			name:            "nil-config",
			processorConfig: nil,
			isError:         true,
			expected:        "no configuration provided",
			spanAttributes:  pcommon.NewMap(),
		},
		{
			name:            "empty-config",
			processorConfig: &Config{},
			isError:         true,
			expected:        "no configuration provided",
			spanAttributes:  pcommon.NewMap(),
		},
		{
			name:            "no-attrs-provided",
			processorConfig: &Config{TargetAttribute: "a", Separator: "b"},
			isError:         true,
			expected:        "attributes field is required",
			spanAttributes:  pcommon.NewMap(),
		},
		{
			name:            "no-target-provided",
			processorConfig: &Config{JoinAttributes: []string{"a"}, Separator: "b"},
			isError:         true,
			expected:        "target_attribute field is required",
			spanAttributes:  pcommon.NewMap(),
		},
		{
			name:            "no-separator-provided",
			processorConfig: &Config{JoinAttributes: []string{"a"}, TargetAttribute: "b"},
			isError:         true,
			expected:        "separator field is required",
			spanAttributes:  pcommon.NewMap(),
		},
	}
	overrideTests = []testConfig{
		{
			name:            "no-override-provided",
			processorConfig: &Config{JoinAttributes: []string{"a"}, TargetAttribute: "b", Separator: "c"},
			isError:         false,
			expected:        "",
			spanAttributes:  pcommon.NewMapFromRaw(map[string]interface{}{"a": "willnotoverride", "b": "somevalue"}),
		},
		{
			name:            "override",
			processorConfig: &Config{JoinAttributes: []string{"a"}, TargetAttribute: "b", Separator: "c", Override: true},
			isError:         false,
			expected:        "",
			spanAttributes:  pcommon.NewMapFromRaw(map[string]interface{}{"a": "overrides", "b": "somevalue"}),
		},
	}
	joinTests = []testConfig{
		{
			name:            "join-attributes-simple",
			processorConfig: &Config{JoinAttributes: []string{"exists1", "exists2"}, TargetAttribute: "joined", Separator: " "},
			isError:         false,
			expected:        "Hello World!",
			spanAttributes:  pcommon.NewMapFromRaw(map[string]interface{}{"exists1": "Hello", "exists2": "World!", "exists3": "notpartofthejoin"}),
		},
		{
			name:            "non-existing-attribute",
			processorConfig: &Config{JoinAttributes: []string{"exists1", "exists2", "doesnotexist"}, TargetAttribute: "joined", Separator: " "},
			isError:         false,
			expected:        "Hello World!",
			spanAttributes:  pcommon.NewMapFromRaw(map[string]interface{}{"exists1": "Hello", "exists2": "World!", "exists3": "notpartofthejoin"}),
		},
	}
)

func TestCreateJoinAttrProcessor(t *testing.T) {
	for _, test := range validateConfigTests {
		t.Run(test.name, func(t *testing.T) {
			cfg := test.processorConfig

			tp, err := newJoinAttrProcessor(context.Background(), cfg)
			if test.isError {
				assert.Error(t, err, test.expected)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, tp)
				assert.Equal(t, true, tp.Capabilities().MutatesData)
			}
		})
	}

	for _, test := range overrideTests {
		t.Run(test.name, func(t *testing.T) {
			cfg := test.processorConfig

			tp, err := newJoinAttrProcessor(context.Background(), cfg)
			if test.isError {
				assert.Error(t, err, test.expected)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, tp)
				assert.Equal(t, true, tp.Capabilities().MutatesData)
			}
		})
	}
}

func TestGetJoinedAttr(t *testing.T) {
	for _, test := range joinTests {
		t.Run(test.name, func(t *testing.T) {
			cfg := test.processorConfig

			tp, err := newJoinAttrProcessor(context.Background(), cfg)
			assert.NoError(t, err)
			assert.NotNil(t, tp)
			assert.Equal(t, true, tp.Capabilities().MutatesData)

			assert.Equal(t, test.expected, tp.GetJoinedAttr(&test.spanAttributes))
		})
	}

}
