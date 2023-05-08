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

package receivercreator

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
)

func Test_expandConfigValue(t *testing.T) {
	type args struct {
		env         map[string]interface{}
		configValue string
	}
	localhostEnv := map[string]interface{}{
		"endpoint": "localhost",
		"nested": map[string]interface{}{
			"outer": map[string]interface{}{
				"inner": map[string]interface{}{
					"value": 123,
				},
			},
		},
	}
	tests := []struct {
		name    string
		args    args
		want    interface{}
		wantErr bool
	}{
		// Non-error cases.
		{"normal string", args{nil, "str"}, "str", false},
		{"expanded string", args{localhostEnv, "`endpoint + ':1234'`"}, "localhost:1234", false},
		{"expanded boolean", args{map[string]interface{}{
			"secure": "true",
		}, "`secure == 'true'`"}, true, false},
		{"expanded number", args{map[string]interface{}{
			"secure": "true",
		}, "`secure == 'true' ? 443 : 80`"}, 443, false},
		{"multiple expressions", args{map[string]interface{}{
			"endpoint": "localhost",
			"port":     1234,
		}, "`endpoint`:`port`"}, "localhost:1234", false},
		{"escaped backticks", args{nil, "\\`foo bar\\`"}, "`foo bar`", false},
		{"expression at beginning", args{localhostEnv, "`endpoint`:1234"}, "localhost:1234", false},
		{"expression in middle", args{localhostEnv, "https://`endpoint`:1234"}, "https://localhost:1234", false},
		{"expression at end", args{localhostEnv, "https://`endpoint`"}, "https://localhost", false},
		{"extra whitespace should not impact returned type", args{nil, "`       true ==           true`"}, true, false},

		{"nested map value", args{localhostEnv, "`endpoint`:`nested[\"outer\"]['inner'][\"value\"]`"}, "localhost:123", false},

		// Error cases.
		{"only backticks", args{observer.EndpointEnv{}, "``"}, nil, true},
		{"whitespace between backticks", args{observer.EndpointEnv{}, "`   `"}, nil, true},
		{"unbalanced backticks", args{nil, "foo ` bar"}, nil, true},
		{"invalid expression", args{observer.EndpointEnv{}, "`(`"}, nil, true},
		{"escape at end without value", args{localhostEnv, "foo \\"}, nil, true},
		{"missing variables", args{observer.EndpointEnv{}, "`missing + vars`"}, nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := evalBackticksInConfigValue(tt.args.configValue, tt.args.env)
			if (err != nil) != tt.wantErr {
				t.Errorf("eval() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_userConfigMap_resolve(t *testing.T) {
	type args struct {
		env observer.EndpointEnv
	}
	tests := []struct {
		name    string
		cm      userConfigMap
		args    args
		want    userConfigMap
		wantErr bool
	}{
		// Note:
		{"multi-level maps", userConfigMap{
			"one": map[string]interface{}{
				"two": "`endpoint`",
			},
		}, args{observer.EndpointEnv{"endpoint": "localhost"}}, userConfigMap{
			"one": map[string]interface{}{
				"two": "localhost",
			},
		}, false},
		{
			"single level map", userConfigMap{
				"endpoint": "`endpoint`:6379",
			}, args{observer.EndpointEnv{"endpoint": "localhost"}}, userConfigMap{
				"endpoint": "localhost:6379",
			}, false,
		},
		{
			"nested slices and maps", userConfigMap{
				"one": []interface{}{
					"`one`:6379",
					map[string]interface{}{
						"two": []interface{}{
							"`two`:6379",
						},
						"three": []interface{}{
							map[string]interface{}{
								"three": "abc`three`xyz",
							},
							map[string]interface{}{
								"four": []string{
									"`first`",
									"second",
									"abc `third` xyz",
								},
							},
						},
					},
				},
			}, args{observer.EndpointEnv{
				"one":   "one.value",
				"two":   "two.value",
				"three": "three.value",
				"first": "first.value",
				"third": "third.value",
			}}, userConfigMap{
				"one": []interface{}{
					"one.value:6379",
					map[string]interface{}{
						"two": []interface{}{
							"two.value:6379",
						},
						"three": []interface{}{
							map[string]interface{}{
								"three": "abcthree.valuexyz",
							},
							map[string]interface{}{
								"four": []interface{}{
									"first.value",
									"second",
									"abc third.value xyz",
								},
							},
						},
					},
				},
			}, false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := expandConfig(tt.cm, tt.args.env)
			if (err != nil) != tt.wantErr {
				t.Errorf("resolve() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			assert.Equal(t, tt.want, got)
		})
	}
}
