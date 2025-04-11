// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package receivercreator

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
)

func Test_expandConfigValue(t *testing.T) {
	type args struct {
		env         map[string]any
		configValue string
	}
	localhostEnv := map[string]any{
		"endpoint": "localhost",
		"nested": map[string]any{
			"outer": map[string]any{
				"inner": map[string]any{
					"value": 123,
				},
			},
		},
	}
	tests := []struct {
		name    string
		args    args
		want    any
		wantErr bool
	}{
		// Non-error cases.
		{"normal string", args{nil, "str"}, "str", false},
		{"expanded string", args{localhostEnv, "`endpoint + ':1234'`"}, "localhost:1234", false},
		{"expanded boolean", args{map[string]any{
			"secure": "true",
		}, "`secure == 'true'`"}, true, false},
		{"expanded number", args{map[string]any{
			"secure": "true",
		}, "`secure == 'true' ? 443 : 80`"}, 443, false},
		{"multiple expressions", args{map[string]any{
			"endpoint": "localhost",
			"port":     1234,
		}, "`endpoint`:`port`"}, "localhost:1234", false},
		{"escaped backticks", args{nil, "\\`foo bar\\`"}, "`foo bar`", false},
		{"single escaped backtick", args{nil, "foo\\` bar"}, "foo` bar", false},
		{"expression at beginning", args{localhostEnv, "`endpoint`:1234"}, "localhost:1234", false},
		{"expression in middle", args{localhostEnv, "https://`endpoint`:1234"}, "https://localhost:1234", false},
		{"expression at end", args{localhostEnv, "https://`endpoint`"}, "https://localhost", false},
		{"extra whitespace should not impact returned type", args{nil, "`       true ==           true`"}, true, false},
		{"nested map value", args{localhostEnv, "`endpoint`:`nested[\"outer\"]['inner'][\"value\"]`"}, "localhost:123", false},
		{
			"regexp value",
			args{
				localhostEnv,
				`^(?P<source_ip>\d+\.\d+.\d+\.\d+)\s+-\s+-\s+\[(?P<timestamp_log>\d+/\w+/\d+:\d+:\d+:\d+\s+\+\d+)\]\s"(?P<http_method>\w+)\s+(?P<http_path>.*)\s+(?P<http_version>.*)"\s+(?P<http_code>\d+)\s+(?P<http_size>\d+)$`,
			},
			`^(?P<source_ip>\d+\.\d+.\d+\.\d+)\s+-\s+-\s+\[(?P<timestamp_log>\d+/\w+/\d+:\d+:\d+:\d+\s+\+\d+)\]\s"(?P<http_method>\w+)\s+(?P<http_path>.*)\s+(?P<http_version>.*)"\s+(?P<http_code>\d+)\s+(?P<http_size>\d+)$`,
			false,
		},
		{
			"regexp value with escaped backtick",
			args{
				localhostEnv,
				"^(?P<source_ip>\\d+\\.\\d+.\\d+\\.\\d+)\\s+\\`-\\s+\"-\\s+\\[(?P<timestamp_log>)]",
			},
			"^(?P<source_ip>\\d+\\.\\d+.\\d+\\.\\d+)\\s+`-\\s+\"-\\s+\\[(?P<timestamp_log>)]",
			false,
		},

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
			"one": map[string]any{
				"two": "`endpoint`",
			},
		}, args{observer.EndpointEnv{"endpoint": "localhost"}}, userConfigMap{
			"one": map[string]any{
				"two": "localhost",
			},
		}, false},
		{"map with regural expression", userConfigMap{
			"one": map[string]any{
				"regex": `^(?P<source_ip>\d+\.\d+.\d+\.\d+)\s+-\s+-\s+\[(?P<timestamp_log>\d+/\w+/\d+:\d+:\d+:\d+\s+\+\d+)\]\s"(?P<http_method>\w+)\s+(?P<http_path>.*)\s+(?P<http_version>.*)"\s+(?P<http_code>\d+)\s+(?P<http_size>\d+)$`,
			},
		}, args{observer.EndpointEnv{"endpoint": "localhost"}}, userConfigMap{
			"one": map[string]any{
				"regex": `^(?P<source_ip>\d+\.\d+.\d+\.\d+)\s+-\s+-\s+\[(?P<timestamp_log>\d+/\w+/\d+:\d+:\d+:\d+\s+\+\d+)\]\s"(?P<http_method>\w+)\s+(?P<http_path>.*)\s+(?P<http_version>.*)"\s+(?P<http_code>\d+)\s+(?P<http_size>\d+)$`,
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
				"one": []any{
					"`one`:6379",
					map[string]any{
						"two": []any{
							"`two`:6379",
						},
						"three": []any{
							map[string]any{
								"three": "abc`three`xyz",
							},
							map[string]any{
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
				"one": []any{
					"one.value:6379",
					map[string]any{
						"two": []any{
							"two.value:6379",
						},
						"three": []any{
							map[string]any{
								"three": "abcthree.valuexyz",
							},
							map[string]any{
								"four": []any{
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
