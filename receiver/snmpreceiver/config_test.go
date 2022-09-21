// Copyright  The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package snmpreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/snmpreceiver"

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/service/servicetest"
)

func TestLoadConfigConnectionConfigs(t *testing.T) {
	type testCase struct {
		name        string
		cfgFile     string
		expectedCfg *Config
		expectedErr string
	}

	testCases := []testCase{
		{
			name:    "NoEndpointUsesDefault",
			cfgFile: "config_no_endpoint.yaml",
			expectedCfg: &Config{
				Endpoint:  "udp://localhost:161",
				Version:   "v2c",
				Community: "public",
				Metrics: map[string]MetricConfig{
					"m3": {
						Unit: "By",
						Gauge: &GaugeMetric{
							ValueType: "float",
						},
						ScalarOIDs: []ScalarOID{
							{
								OID: "1",
							},
						},
					},
				},
			},
			expectedErr: "",
		},
		{
			name:    "InvalidEndpointErrors",
			cfgFile: "config_invalid_endpoint.yaml",
			expectedCfg: &Config{
				Endpoint:  "udp://a:a:a:a:a:a",
				Version:   "v2c",
				Community: "public",
				Metrics: map[string]MetricConfig{
					"m3": {
						Unit: "By",
						Gauge: &GaugeMetric{
							ValueType: "float",
						},
						ScalarOIDs: []ScalarOID{
							{
								OID: "1",
							},
						},
					},
				},
			},
			expectedErr: fmt.Sprintf(errMsgInvalidEndpoint[:len(errMsgInvalidEndpoint)-2], "udp://a:a:a:a:a:a"),
		},
		{
			name:    "NoPortUsesDefault",
			cfgFile: "config_no_port.yaml",
			expectedCfg: &Config{
				Endpoint:  "udp://localhost:161",
				Version:   "v2c",
				Community: "public",
				Metrics: map[string]MetricConfig{
					"m3": {
						Unit: "By",
						Gauge: &GaugeMetric{
							ValueType: "float",
						},
						ScalarOIDs: []ScalarOID{
							{
								OID: "1",
							},
						},
					},
				},
			},
			expectedErr: "",
		},
		{
			name:    "NoPortTrailingColonUsesDefault",
			cfgFile: "config_no_port_trailing_colon.yaml",
			expectedCfg: &Config{
				Endpoint:  "udp://localhost:161",
				Version:   "v2c",
				Community: "public",
				Metrics: map[string]MetricConfig{
					"m3": {
						Unit: "By",
						Gauge: &GaugeMetric{
							ValueType: "float",
						},
						ScalarOIDs: []ScalarOID{
							{
								OID: "1",
							},
						},
					},
				},
			},
			expectedErr: "",
		},
		{
			name:    "BadEndpointSchemeErrors",
			cfgFile: "config_bad_endpoint_scheme.yaml",
			expectedCfg: &Config{
				Endpoint:  "http://localhost:161",
				Version:   "v2c",
				Community: "public",
				Metrics: map[string]MetricConfig{
					"m3": {
						Unit: "By",
						Gauge: &GaugeMetric{
							ValueType: "float",
						},
						ScalarOIDs: []ScalarOID{
							{
								OID: "1",
							},
						},
					},
				},
			},
			expectedErr: errEndpointBadScheme.Error(),
		},
		{
			name:    "NoEndpointSchemeErrors",
			cfgFile: "config_no_endpoint_scheme.yaml",
			expectedCfg: &Config{
				Endpoint:  "localhost:161",
				Version:   "v2c",
				Community: "public",
				Metrics: map[string]MetricConfig{
					"m3": {
						Unit: "By",
						Gauge: &GaugeMetric{
							ValueType: "float",
						},
						ScalarOIDs: []ScalarOID{
							{
								OID: "1",
							},
						},
					},
				},
			},
			expectedErr: fmt.Sprintf(errMsgNoHostInvalidEndpoint, "localhost:161"),
		},
		{
			name:    "NoVersionUsesDefault",
			cfgFile: "config_no_version.yaml",
			expectedCfg: &Config{
				Endpoint:  "udp://localhost:161",
				Version:   "v2c",
				Community: "public",
				Metrics: map[string]MetricConfig{
					"m3": {
						Unit: "By",
						Gauge: &GaugeMetric{
							ValueType: "float",
						},
						ScalarOIDs: []ScalarOID{
							{
								OID: "1",
							},
						},
					},
				},
			},
			expectedErr: "",
		},
		{
			name:    "BadVersionErrors",
			cfgFile: "config_bad_version.yaml",
			expectedCfg: &Config{
				Endpoint:  "udp://localhost:161",
				Version:   "9999",
				Community: "public",
				Metrics: map[string]MetricConfig{
					"m3": {
						Unit: "By",
						Gauge: &GaugeMetric{
							ValueType: "float",
						},
						ScalarOIDs: []ScalarOID{
							{
								OID: "1",
							},
						},
					},
				},
			},
			expectedErr: errBadVersion.Error(),
		},
		{
			name:    "V3NoUserErrors",
			cfgFile: "config_v3_no_user.yaml",
			expectedCfg: &Config{
				Endpoint:      "udp://localhost:161",
				Version:       "v3",
				SecurityLevel: "no_auth_no_priv",
				Metrics: map[string]MetricConfig{
					"m3": {
						Unit: "By",
						Gauge: &GaugeMetric{
							ValueType: "float",
						},
						ScalarOIDs: []ScalarOID{
							{
								OID: "1",
							},
						},
					},
				},
			},
			expectedErr: errEmptyUser.Error(),
		},
		{
			name:    "V3NoSecurityLevelUsesDefault",
			cfgFile: "config_v3_no_security_level.yaml",
			expectedCfg: &Config{
				Endpoint:      "udp://localhost:161",
				Version:       "v3",
				SecurityLevel: "no_auth_no_priv",
				User:          "u",
				Metrics: map[string]MetricConfig{
					"m3": {
						Unit: "By",
						Gauge: &GaugeMetric{
							ValueType: "float",
						},
						ScalarOIDs: []ScalarOID{
							{
								OID: "1",
							},
						},
					},
				},
			},
			expectedErr: "",
		},
		{
			name:    "V3BadSecurityLevelErrors",
			cfgFile: "config_v3_bad_security_level.yaml",
			expectedCfg: &Config{
				Endpoint:      "udp://localhost:161",
				Version:       "v3",
				SecurityLevel: "super",
				User:          "u",
				Metrics: map[string]MetricConfig{
					"m3": {
						Unit: "By",
						Gauge: &GaugeMetric{
							ValueType: "float",
						},
						ScalarOIDs: []ScalarOID{
							{
								OID: "1",
							},
						},
					},
				},
			},
			expectedErr: errBadSecurityLevel.Error(),
		},
		{
			name:    "V3NoAuthTypeUsesDefault",
			cfgFile: "config_v3_no_auth_type.yaml",
			expectedCfg: &Config{
				Endpoint:      "udp://localhost:161",
				Version:       "v3",
				SecurityLevel: "auth_no_priv",
				User:          "u",
				AuthType:      "MD5",
				AuthPassword:  "p",
				Metrics: map[string]MetricConfig{
					"m3": {
						Unit: "By",
						Gauge: &GaugeMetric{
							ValueType: "float",
						},
						ScalarOIDs: []ScalarOID{
							{
								OID: "1",
							},
						},
					},
				},
			},
			expectedErr: "",
		},
		{
			name:    "V3BadAuthTypeErrors",
			cfgFile: "config_v3_bad_auth_type.yaml",
			expectedCfg: &Config{
				Endpoint:      "udp://localhost:161",
				Version:       "v3",
				SecurityLevel: "auth_no_priv",
				User:          "u",
				AuthType:      "super",
				AuthPassword:  "p",
				Metrics: map[string]MetricConfig{
					"m3": {
						Unit: "By",
						Gauge: &GaugeMetric{
							ValueType: "float",
						},
						ScalarOIDs: []ScalarOID{
							{
								OID: "1",
							},
						},
					},
				},
			},
			expectedErr: errBadAuthType.Error(),
		},
		{
			name:    "V3NoAuthPasswordErrors",
			cfgFile: "config_v3_no_auth_password.yaml",
			expectedCfg: &Config{
				Endpoint:      "udp://localhost:161",
				Version:       "v3",
				SecurityLevel: "auth_no_priv",
				User:          "u",
				AuthType:      "md5",
				Metrics: map[string]MetricConfig{
					"m3": {
						Unit: "By",
						Gauge: &GaugeMetric{
							ValueType: "float",
						},
						ScalarOIDs: []ScalarOID{
							{
								OID: "1",
							},
						},
					},
				},
			},
			expectedErr: errEmptyAuthPassword.Error(),
		},
		{
			name:    "V3NoPrivacyTypeUsesDefault",
			cfgFile: "config_v3_no_privacy_type.yaml",
			expectedCfg: &Config{
				Endpoint:        "udp://localhost:161",
				Version:         "v3",
				SecurityLevel:   "auth_priv",
				User:            "u",
				AuthType:        "md5",
				AuthPassword:    "p",
				PrivacyType:     "DES",
				PrivacyPassword: "pp",
				Metrics: map[string]MetricConfig{
					"m3": {
						Unit: "By",
						Gauge: &GaugeMetric{
							ValueType: "float",
						},
						ScalarOIDs: []ScalarOID{
							{
								OID: "1",
							},
						},
					},
				},
			},
			expectedErr: "",
		},
		{
			name:    "V3BadPrivacyTypeErrors",
			cfgFile: "config_v3_bad_privacy_type.yaml",
			expectedCfg: &Config{
				Endpoint:        "udp://localhost:161",
				Version:         "v3",
				SecurityLevel:   "auth_priv",
				User:            "u",
				AuthType:        "md5",
				AuthPassword:    "p",
				PrivacyType:     "super",
				PrivacyPassword: "pp",
				Metrics: map[string]MetricConfig{
					"m3": {
						Unit: "By",
						Gauge: &GaugeMetric{
							ValueType: "float",
						},
						ScalarOIDs: []ScalarOID{
							{
								OID: "1",
							},
						},
					},
				},
			},
			expectedErr: errBadPrivacyType.Error(),
		},
		{
			name:    "V3NoPrivacyPasswordErrors",
			cfgFile: "config_v3_no_privacy_password.yaml",
			expectedCfg: &Config{
				Endpoint:      "udp://localhost:161",
				Version:       "v3",
				SecurityLevel: "auth_priv",
				User:          "u",
				AuthType:      "md5",
				AuthPassword:  "p",
				PrivacyType:   "des",
				Metrics: map[string]MetricConfig{
					"m3": {
						Unit: "By",
						Gauge: &GaugeMetric{
							ValueType: "float",
						},
						ScalarOIDs: []ScalarOID{
							{
								OID: "1",
							},
						},
					},
				},
			},
			expectedErr: errEmptyPrivacyPassword.Error(),
		},
		{
			name:    "GoodV2CConnectionNoErrors",
			cfgFile: "config_v2c_connection_good.yaml",
			expectedCfg: &Config{
				Endpoint:  "udp://localhost:161",
				Version:   "v2c",
				Community: "public",
				Metrics: map[string]MetricConfig{
					"m3": {
						Unit: "By",
						Gauge: &GaugeMetric{
							ValueType: "float",
						},
						ScalarOIDs: []ScalarOID{
							{
								OID: "1",
							},
						},
					},
				},
			},
			expectedErr: "",
		},
		{
			name:    "GoodV3ConnectionNoErrors",
			cfgFile: "config_v3_connection_good.yaml",
			expectedCfg: &Config{
				Endpoint:        "udp://localhost:161",
				Version:         "v3",
				SecurityLevel:   "auth_priv",
				User:            "u",
				AuthType:        "md5",
				AuthPassword:    "p",
				PrivacyType:     "des",
				PrivacyPassword: "pp",
				Metrics: map[string]MetricConfig{
					"m3": {
						Unit: "By",
						Gauge: &GaugeMetric{
							ValueType: "float",
						},
						ScalarOIDs: []ScalarOID{
							{
								OID: "1",
							},
						},
					},
				},
			},
			expectedErr: "",
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			factories, err := componenttest.NopFactories()
			assert.NoError(t, err)

			factory := NewFactory()
			factories.Receivers[typeStr] = factory
			cfg, err := servicetest.LoadConfigAndValidate(filepath.Join("testdata", test.cfgFile), factories)
			snmpConfig := cfg.Receivers[config.NewComponentID(typeStr)].(*Config)

			if test.expectedErr != "" {
				assert.ErrorContains(t, err, test.expectedErr)
			} else {
				assert.NoError(t, err)
			}
			compareCfgs(t, test.expectedCfg, snmpConfig)
		})
	}
}

func TestLoadConfigMetricConfigs(t *testing.T) {
	type testCase struct {
		name        string
		cfgFile     string
		expectedCfg *Config
		expectedErr string
	}

	testCases := []testCase{
		{
			name:    "NoMetricConfigsErrors",
			cfgFile: "config_no_metric_config.yaml",
			expectedCfg: &Config{
				Endpoint:  "udp://localhost:161",
				Version:   "v2c",
				Community: "public",
				Metrics:   nil,
			},
			expectedErr: errMetricRequired.Error(),
		},
		{
			name:    "NoMetricUnitErrors",
			cfgFile: "config_no_metric_unit.yaml",
			expectedCfg: &Config{
				Endpoint:  "udp://localhost:161",
				Version:   "v2c",
				Community: "public",
				Metrics: map[string]MetricConfig{
					"m3": {
						Unit: "",
						Gauge: &GaugeMetric{
							ValueType: "float",
						},
						ScalarOIDs: []ScalarOID{
							{
								OID: "1",
							},
						},
					},
				},
			},
			expectedErr: fmt.Sprintf(errMsgMetricNoUnit, "m3"),
		},
		{
			name:    "NoMetricGaugeOrSumErrors",
			cfgFile: "config_no_metric_gauge_or_sum.yaml",
			expectedCfg: &Config{
				Endpoint:  "udp://localhost:161",
				Version:   "v2c",
				Community: "public",
				Metrics: map[string]MetricConfig{
					"m3": {
						Unit: "1",
						ScalarOIDs: []ScalarOID{
							{
								OID: "1",
							},
						},
					},
				},
			},
			expectedErr: fmt.Sprintf(errMsgMetricNoGaugeOrSum, "m3"),
		},
		{
			name:    "NoMetricOIDsErrors",
			cfgFile: "config_no_metric_oids.yaml",
			expectedCfg: &Config{
				Endpoint:  "udp://localhost:161",
				Version:   "v2c",
				Community: "public",
				Metrics: map[string]MetricConfig{
					"m3": {
						Unit: "By",
						Gauge: &GaugeMetric{
							ValueType: "float",
						},
					},
				},
			},
			expectedErr: fmt.Sprintf(errMsgMetricNoOIDs, "m3"),
		},
		{
			name:    "NoMetricGaugeTypeUsesDefault",
			cfgFile: "config_no_metric_gauge_type.yaml",
			expectedCfg: &Config{
				Endpoint:  "udp://localhost:161",
				Version:   "v2c",
				Community: "public",
				Metrics: map[string]MetricConfig{
					"m3": {
						Unit: "By",
						Gauge: &GaugeMetric{
							ValueType: "float",
						},
						ScalarOIDs: []ScalarOID{
							{
								OID: "1",
							},
						},
					},
				},
			},
			expectedErr: "",
		},
		{
			name:    "BadMetricGaugeTypeErrors",
			cfgFile: "config_bad_metric_gauge_type.yaml",
			expectedCfg: &Config{
				Endpoint:  "udp://localhost:161",
				Version:   "v2c",
				Community: "public",
				Metrics: map[string]MetricConfig{
					"m3": {
						Unit: "By",
						Gauge: &GaugeMetric{
							ValueType: "Counter",
						},
						ScalarOIDs: []ScalarOID{
							{
								OID: "1",
							},
						},
					},
				},
			},
			expectedErr: fmt.Sprintf(errMsgGaugeBadValueType, "m3"),
		},
		{
			name:    "NoMetricSumTypeUsesDefault",
			cfgFile: "config_no_metric_sum_type.yaml",
			expectedCfg: &Config{
				Endpoint:  "udp://localhost:161",
				Version:   "v2c",
				Community: "public",
				Metrics: map[string]MetricConfig{
					"m3": {
						Unit: "By",
						Sum: &SumMetric{
							Monotonic:   true,
							Aggregation: "cumulative",
							ValueType:   "float",
						},
						ScalarOIDs: []ScalarOID{
							{
								OID: "1",
							},
						},
					},
				},
			},
			expectedErr: "",
		},
		{
			name:    "BadMetricSumTypeErrors",
			cfgFile: "config_bad_metric_sum_type.yaml",
			expectedCfg: &Config{
				Endpoint:  "udp://localhost:161",
				Version:   "v2c",
				Community: "public",
				Metrics: map[string]MetricConfig{
					"m3": {
						Unit: "By",
						Sum: &SumMetric{
							Monotonic:   true,
							Aggregation: "cumulative",
							ValueType:   "Counter",
						},
						ScalarOIDs: []ScalarOID{
							{
								OID: "1",
							},
						},
					},
				},
			},
			expectedErr: fmt.Sprintf(errMsgSumBadValueType, "m3"),
		},
		{
			name:    "NoMetricSumAggregationUsesDefault",
			cfgFile: "config_no_metric_sum_aggregation.yaml",
			expectedCfg: &Config{
				Endpoint:  "udp://localhost:161",
				Version:   "v2c",
				Community: "public",
				Metrics: map[string]MetricConfig{
					"m3": {
						Unit: "By",
						Sum: &SumMetric{
							Monotonic:   true,
							Aggregation: "cumulative",
							ValueType:   "float",
						},
						ScalarOIDs: []ScalarOID{
							{
								OID: "1",
							},
						},
					},
				},
			},
			expectedErr: "",
		},
		{
			name:    "BadMetricSumAggregationErrors",
			cfgFile: "config_bad_metric_sum_aggregation.yaml",
			expectedCfg: &Config{
				Endpoint:  "udp://localhost:161",
				Version:   "v2c",
				Community: "public",
				Metrics: map[string]MetricConfig{
					"m3": {
						Unit: "By",
						Sum: &SumMetric{
							Monotonic:   true,
							Aggregation: "Counter",
							ValueType:   "float",
						},
						ScalarOIDs: []ScalarOID{
							{
								OID: "1",
							},
						},
					},
				},
			},
			expectedErr: fmt.Sprintf(errMsgSumBadAggregation, "m3"),
		},
		{
			name:    "NoScalarOIDOIDErrors",
			cfgFile: "config_no_scalar_oid_oid.yaml",
			expectedCfg: &Config{
				Endpoint:  "udp://localhost:161",
				Version:   "v2c",
				Community: "public",
				Attributes: map[string]AttributeConfig{
					"a2": {
						Enum: []string{
							"val1",
						},
					},
				},
				Metrics: map[string]MetricConfig{
					"m3": {
						Unit: "By",
						Sum: &SumMetric{
							Monotonic:   true,
							Aggregation: "cumulative",
							ValueType:   "float",
						},
						ScalarOIDs: []ScalarOID{
							{
								Attributes: []Attribute{
									{
										Name:  "a2",
										Value: "val1",
									},
								},
							},
						},
					},
				},
			},
			expectedErr: fmt.Sprintf(errMsgScalarOIDNoOID, "m3"),
		},
		{
			name:    "NoAttributeConfigOIDPrefixOrEnumsErrors",
			cfgFile: "config_no_attribute_oid_prefix_or_enums.yaml",
			expectedCfg: &Config{
				Endpoint:  "udp://localhost:161",
				Version:   "v2c",
				Community: "public",
				Attributes: map[string]AttributeConfig{
					"a2": {},
				},
				Metrics: map[string]MetricConfig{
					"m3": {
						Unit: "By",
						Sum: &SumMetric{
							Monotonic:   true,
							Aggregation: "cumulative",
							ValueType:   "float",
						},
						ScalarOIDs: []ScalarOID{
							{
								OID: "1",
								Attributes: []Attribute{
									{
										Name:  "a2",
										Value: "val1",
									},
								},
							},
						},
					},
				},
			},
			expectedErr: fmt.Sprintf(errMsgAttributeConfigNoEnumOIDOrPrefix, "a2"),
		},
		{
			name:    "NoScalarOIDAttributeNameErrors",
			cfgFile: "config_no_scalar_oid_attribute_name.yaml",
			expectedCfg: &Config{
				Endpoint:  "udp://localhost:161",
				Version:   "v2c",
				Community: "public",
				Attributes: map[string]AttributeConfig{
					"a2": {
						Enum: []string{
							"val1",
						},
					},
				},
				Metrics: map[string]MetricConfig{
					"m3": {
						Unit: "By",
						Sum: &SumMetric{
							Monotonic:   true,
							Aggregation: "cumulative",
							ValueType:   "float",
						},
						ScalarOIDs: []ScalarOID{
							{
								OID: "1",
								Attributes: []Attribute{
									{
										Value: "val1",
									},
								},
							},
						},
					},
				},
			},
			expectedErr: fmt.Sprintf(errMsgScalarAttributeNoName, "m3"),
		},
		{
			name:    "BadScalarOIDAttributeNameErrors",
			cfgFile: "config_bad_scalar_oid_attribute_name.yaml",
			expectedCfg: &Config{
				Endpoint:  "udp://localhost:161",
				Version:   "v2c",
				Community: "public",
				Attributes: map[string]AttributeConfig{
					"a2": {
						Enum: []string{
							"val1",
						},
					},
				},
				Metrics: map[string]MetricConfig{
					"m3": {
						Unit: "By",
						Sum: &SumMetric{
							Monotonic:   true,
							Aggregation: "cumulative",
							ValueType:   "float",
						},
						ScalarOIDs: []ScalarOID{
							{
								OID: "1",
								Attributes: []Attribute{
									{
										Name:  "a1",
										Value: "val1",
									},
								},
							},
						},
					},
				},
			},
			expectedErr: fmt.Sprintf(errMsgScalarAttributeBadName, "m3", "a1"),
		},
		{
			name:    "BadScalarOIDAttributeErrors",
			cfgFile: "config_bad_scalar_oid_attribute.yaml",
			expectedCfg: &Config{
				Endpoint:  "udp://localhost:161",
				Version:   "v2c",
				Community: "public",
				Attributes: map[string]AttributeConfig{
					"a2": {
						OID: "2",
					},
				},
				Metrics: map[string]MetricConfig{
					"m3": {
						Unit: "By",
						Sum: &SumMetric{
							Monotonic:   true,
							Aggregation: "cumulative",
							ValueType:   "float",
						},
						ScalarOIDs: []ScalarOID{
							{
								OID: "1",
								Attributes: []Attribute{
									{
										Name:  "a2",
										Value: "val1",
									},
								},
							},
						},
					},
				},
			},
			expectedErr: fmt.Sprintf(errMsgScalarOIDBadAttribute, "m3", "a2"),
		},
		{
			name:    "BadScalarOIDAttributeValueErrors",
			cfgFile: "config_bad_scalar_oid_attribute_value.yaml",
			expectedCfg: &Config{
				Endpoint:  "udp://localhost:161",
				Version:   "v2c",
				Community: "public",
				Attributes: map[string]AttributeConfig{
					"a2": {
						Enum: []string{
							"val1",
						},
					},
				},
				Metrics: map[string]MetricConfig{
					"m3": {
						Unit: "By",
						Sum: &SumMetric{
							Monotonic:   true,
							Aggregation: "cumulative",
							ValueType:   "float",
						},
						ScalarOIDs: []ScalarOID{
							{
								OID: "1",
								Attributes: []Attribute{
									{
										Name:  "a2",
										Value: "val2",
									},
								},
							},
						},
					},
				},
			},
			expectedErr: fmt.Sprintf(errMsgScalarAttributeBadValue, "m3", "a2", "val2"),
		},
		{
			name:    "NoColumnOIDOIDErrors",
			cfgFile: "config_no_column_oid_oid.yaml",
			expectedCfg: &Config{
				Endpoint:  "udp://localhost:161",
				Version:   "v2c",
				Community: "public",
				Attributes: map[string]AttributeConfig{
					"a2": {
						Enum: []string{
							"val1",
						},
					},
				},
				Metrics: map[string]MetricConfig{
					"m3": {
						Unit: "By",
						Sum: &SumMetric{
							Monotonic:   true,
							Aggregation: "cumulative",
							ValueType:   "float",
						},
						ColumnOIDs: []ColumnOID{
							{
								Attributes: []Attribute{
									{
										Name:  "a2",
										Value: "val1",
									},
								},
							},
						},
					},
				},
			},
			expectedErr: fmt.Sprintf(errMsgColumnOIDNoOID, "m3"),
		},
		{
			name:    "NoColumnOIDAttributeNameErrors",
			cfgFile: "config_no_column_oid_attribute_name.yaml",
			expectedCfg: &Config{
				Endpoint:  "udp://localhost:161",
				Version:   "v2c",
				Community: "public",
				Attributes: map[string]AttributeConfig{
					"a2": {
						Enum: []string{
							"val1",
						},
					},
				},
				Metrics: map[string]MetricConfig{
					"m3": {
						Unit: "By",
						Sum: &SumMetric{
							Monotonic:   true,
							Aggregation: "cumulative",
							ValueType:   "float",
						},
						ColumnOIDs: []ColumnOID{
							{
								OID: "1",
								Attributes: []Attribute{
									{
										Value: "val1",
									},
								},
							},
						},
					},
				},
			},
			expectedErr: fmt.Sprintf(errMsgColumnAttributeNoName, "m3"),
		},
		{
			name:    "BadColumnOIDAttributeNameErrors",
			cfgFile: "config_bad_column_oid_attribute_name.yaml",
			expectedCfg: &Config{
				Endpoint:  "udp://localhost:161",
				Version:   "v2c",
				Community: "public",
				Attributes: map[string]AttributeConfig{
					"a2": {
						Enum: []string{
							"val1",
						},
					},
				},
				Metrics: map[string]MetricConfig{
					"m3": {
						Unit: "By",
						Sum: &SumMetric{
							Monotonic:   true,
							Aggregation: "cumulative",
							ValueType:   "float",
						},
						ColumnOIDs: []ColumnOID{
							{
								OID: "1",
								Attributes: []Attribute{
									{
										Name:  "a1",
										Value: "val1",
									},
								},
							},
						},
					},
				},
			},
			expectedErr: fmt.Sprintf(errMsgColumnAttributeBadName, "m3", "a1"),
		},
		{
			name:    "BadColumnOIDAttributeValueErrors",
			cfgFile: "config_bad_column_oid_attribute_value.yaml",
			expectedCfg: &Config{
				Endpoint:  "udp://localhost:161",
				Version:   "v2c",
				Community: "public",
				Attributes: map[string]AttributeConfig{
					"a2": {
						Enum: []string{
							"val1",
						},
					},
				},
				Metrics: map[string]MetricConfig{
					"m3": {
						Unit: "By",
						Sum: &SumMetric{
							Monotonic:   true,
							Aggregation: "cumulative",
							ValueType:   "float",
						},
						ColumnOIDs: []ColumnOID{
							{
								OID: "1",
								Attributes: []Attribute{
									{
										Name:  "a2",
										Value: "val2",
									},
								},
							},
						},
					},
				},
			},
			expectedErr: fmt.Sprintf(errMsgColumnAttributeBadValue, "m3", "a2", "val2"),
		},
		{
			name:    "BadColumnOIDResourceAttributeNameErrors",
			cfgFile: "config_bad_column_oid_resource_attribute_name.yaml",
			expectedCfg: &Config{
				Endpoint:  "udp://localhost:161",
				Version:   "v2c",
				Community: "public",
				ResourceAttributes: map[string]ResourceAttributeConfig{
					"a2": {
						OID: "2",
					},
				},
				Metrics: map[string]MetricConfig{
					"m3": {
						Unit: "By",
						Sum: &SumMetric{
							Monotonic:   true,
							Aggregation: "cumulative",
							ValueType:   "float",
						},
						ColumnOIDs: []ColumnOID{
							{
								OID: "1",
								ResourceAttributes: []string{
									"a1",
								},
							},
						},
					},
				},
			},
			expectedErr: fmt.Sprintf(errMsgColumnResourceAttributeBadName, "m3", "a1"),
		},
		{
			name:    "NoResourceAttributeConfigOIDOrPrefixErrors",
			cfgFile: "config_no_resource_attribute_oid_or_prefix.yaml",
			expectedCfg: &Config{
				Endpoint:  "udp://localhost:161",
				Version:   "v2c",
				Community: "public",
				ResourceAttributes: map[string]ResourceAttributeConfig{
					"a2": {},
				},
				Metrics: map[string]MetricConfig{
					"m3": {
						Unit: "By",
						Sum: &SumMetric{
							Monotonic:   true,
							Aggregation: "cumulative",
							ValueType:   "float",
						},
						ColumnOIDs: []ColumnOID{
							{
								OID: "1",
								ResourceAttributes: []string{
									"a2",
								},
							},
						},
					},
				},
			},
			expectedErr: fmt.Sprintf(errMsgResourceAttributeNoOIDOrPrefix, "a2"),
		},
		{
			name:    "ComplexConfigGood",
			cfgFile: "config_complex_good.yaml",
			expectedCfg: &Config{
				Endpoint:  "udp://localhost:161",
				Version:   "v2c",
				Community: "public",
				ResourceAttributes: map[string]ResourceAttributeConfig{
					"ra1": {
						IndexedValuePrefix: "p",
					},
					"ra2": {
						OID: "1",
					},
				},
				Attributes: map[string]AttributeConfig{
					"a1": {
						Enum: []string{
							"val1",
							"val2",
						},
					},
					"a2": {
						Value: "v",
						Enum: []string{
							"val1",
						},
					},
					"a3": {
						IndexedValuePrefix: "p",
					},
					"a4": {
						OID: "1",
					},
				},
				Metrics: map[string]MetricConfig{
					"m1": {
						Unit: "1",
						Sum: &SumMetric{
							Monotonic:   true,
							Aggregation: "cumulative",
							ValueType:   "int",
						},
						ColumnOIDs: []ColumnOID{
							{
								OID: "1",
								Attributes: []Attribute{
									{
										Name: "a4",
									},
								},
							},
						},
					},
					"m2": {
						Unit: "By",
						Gauge: &GaugeMetric{
							ValueType: "int",
						},
						ColumnOIDs: []ColumnOID{
							{
								OID: "1",
								Attributes: []Attribute{
									{
										Name: "a3",
									},
									{
										Name:  "a1",
										Value: "val1",
									},
								},
							},
							{
								OID: "2",
								Attributes: []Attribute{
									{
										Name: "a3",
									},
									{
										Name:  "a1",
										Value: "val2",
									},
								},
							},
						},
					},
					"m3": {
						Unit: "By",
						Gauge: &GaugeMetric{
							ValueType: "float",
						},
						ScalarOIDs: []ScalarOID{
							{
								OID: "1",
								Attributes: []Attribute{
									{
										Name:  "a2",
										Value: "val1",
									},
								},
							},
						},
					},
					"m4": {
						Unit: "{things}",
						Sum: &SumMetric{
							Aggregation: "cumulative",
							Monotonic:   true,
							ValueType:   "bool",
						},
						ScalarOIDs: []ScalarOID{
							{
								OID: "1",
							},
						},
					},
					"m5": {
						Unit: "{things}",
						Sum: &SumMetric{
							Aggregation: "cumulative",
							Monotonic:   false,
							ValueType:   "int",
						},
						ScalarOIDs: []ScalarOID{
							{
								OID: "1",
								Attributes: []Attribute{
									{
										Name:  "a2",
										Value: "val1",
									},
								},
							},
						},
					},
					"m6": {
						Unit: "1",
						Sum: &SumMetric{
							Aggregation: "delta",
							Monotonic:   true,
							ValueType:   "int",
						},
						ScalarOIDs: []ScalarOID{
							{
								OID: "1",
								Attributes: []Attribute{
									{
										Name:  "a1",
										Value: "val1",
									},
								},
							},
							{
								OID: "2",
								Attributes: []Attribute{
									{
										Name:  "a1",
										Value: "val2",
									},
								},
							},
						},
					},
					"m7": {
						Unit: "By",
						Gauge: &GaugeMetric{
							ValueType: "int",
						},
						ColumnOIDs: []ColumnOID{
							{
								OID: "1",
								ResourceAttributes: []string{
									"ra1",
								},
							},
						},
					},
					"m8": {
						Unit: "By",
						Gauge: &GaugeMetric{
							ValueType: "int",
						},
						ColumnOIDs: []ColumnOID{
							{
								OID: "1",
								ResourceAttributes: []string{
									"ra2",
								},
							},
						},
					},
					"m9": {
						Unit: "By",
						Gauge: &GaugeMetric{
							ValueType: "int",
						},
						ColumnOIDs: []ColumnOID{
							{
								OID: "1",
								ResourceAttributes: []string{
									"ra1",
									"ra2",
								},
							},
						},
					},
					"m10": {
						Unit: "By",
						Gauge: &GaugeMetric{
							ValueType: "int",
						},
						ColumnOIDs: []ColumnOID{
							{
								OID: "1",
								ResourceAttributes: []string{
									"ra1",
									"ra2",
								},
								Attributes: []Attribute{
									{
										Name:  "a2",
										Value: "val1",
									},
									{
										Name: "a3",
									},
									{
										Name: "a4",
									},
								},
							},
						},
					},
				},
			},
			expectedErr: "",
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			factories, err := componenttest.NopFactories()
			assert.NoError(t, err)

			factory := NewFactory()
			factories.Receivers[typeStr] = factory
			cfg, err := servicetest.LoadConfigAndValidate(filepath.Join("testdata", test.cfgFile), factories)
			snmpConfig := cfg.Receivers[config.NewComponentID(typeStr)].(*Config)

			if test.expectedErr != "" {
				assert.ErrorContains(t, err, test.expectedErr)
			} else {
				assert.NoError(t, err)
			}
			compareCfgs(t, test.expectedCfg, snmpConfig)
		})
	}
}

// Testing Validate directly to test that missing data errors when no defaults are provided
func TestValidate(t *testing.T) {
	type testCase struct {
		name        string
		cfg         *Config
		expectedErr string
	}

	testCases := []testCase{
		{
			name: "NoEndpointErrors",
			cfg: &Config{
				Version:   "v2c",
				Community: "public",
				Metrics: map[string]MetricConfig{
					"m3": {
						Unit: "By",
						Gauge: &GaugeMetric{
							ValueType: "float",
						},
						ScalarOIDs: []ScalarOID{
							{
								OID: "1",
							},
						},
					},
				},
			},
			expectedErr: errEmptyEndpoint.Error(),
		},
		{
			name: "NoVersionErrors",
			cfg: &Config{
				Endpoint:  "udp://localhost:161",
				Community: "public",
				Metrics: map[string]MetricConfig{
					"m3": {
						Unit: "By",
						Gauge: &GaugeMetric{
							ValueType: "float",
						},
						ScalarOIDs: []ScalarOID{
							{
								OID: "1",
							},
						},
					},
				},
			},
			expectedErr: errEmptyVersion.Error(),
		},
		{
			name: "V3NoSecurityLevelErrors",
			cfg: &Config{
				Endpoint: "udp://localhost:161",
				Version:  "v3",
				User:     "u",
				Metrics: map[string]MetricConfig{
					"m3": {
						Unit: "By",
						Gauge: &GaugeMetric{
							ValueType: "float",
						},
						ScalarOIDs: []ScalarOID{
							{
								OID: "1",
							},
						},
					},
				},
			},
			expectedErr: errEmptySecurityLevel.Error(),
		},
		{
			name: "V3NoAuthTypeErrors",
			cfg: &Config{
				Endpoint:      "udp://localhost:161",
				Version:       "v3",
				SecurityLevel: "auth_no_priv",
				User:          "u",
				AuthPassword:  "p",
				Metrics: map[string]MetricConfig{
					"m3": {
						Unit: "By",
						Gauge: &GaugeMetric{
							ValueType: "float",
						},
						ScalarOIDs: []ScalarOID{
							{
								OID: "1",
							},
						},
					},
				},
			},
			expectedErr: errEmptyAuthType.Error(),
		},
		{
			name: "V3NoPrivacyTypeErrors",
			cfg: &Config{
				Endpoint:        "udp://localhost:161",
				Version:         "v3",
				SecurityLevel:   "auth_priv",
				User:            "u",
				AuthType:        "md5",
				AuthPassword:    "p",
				PrivacyPassword: "pp",
				Metrics: map[string]MetricConfig{
					"m3": {
						Unit: "By",
						Gauge: &GaugeMetric{
							ValueType: "float",
						},
						ScalarOIDs: []ScalarOID{
							{
								OID: "1",
							},
						},
					},
				},
			},
			expectedErr: errEmptyPrivacyType.Error(),
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			err := test.cfg.Validate()
			assert.ErrorContains(t, err, test.expectedErr)
		})
	}
}

func compareCfgs(t assert.TestingT, expected *Config, actual *Config) {
	assert.Equal(t, expected.Endpoint, actual.Endpoint)
	assert.Equal(t, expected.Version, actual.Version)
	assert.Equal(t, expected.ResourceAttributes, actual.ResourceAttributes)
	assert.Equal(t, expected.Attributes, actual.Attributes)
	assert.Equal(t, expected.Metrics, actual.Metrics)
	if expected.Version == "v3" {
		assert.Equal(t, expected.SecurityLevel, actual.SecurityLevel)
		assert.Equal(t, expected.User, actual.User)
		if expected.SecurityLevel == "auth_no_priv" || expected.SecurityLevel == "auth_priv" {
			assert.Equal(t, expected.AuthType, actual.AuthType)
			assert.Equal(t, expected.AuthPassword, actual.AuthPassword)
			if expected.SecurityLevel == "auth_priv" {
				assert.Equal(t, expected.PrivacyPassword, actual.PrivacyPassword)
				assert.Equal(t, expected.PrivacyType, actual.PrivacyType)
			}
		}
	} else {
		assert.Equal(t, expected.Community, actual.Community)
	}
}
