// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package snmpreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/snmpreceiver"

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/snmpreceiver/internal/metadata"
)

func TestLoadConfigConnectionConfigs(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	factory := NewFactory()

	type testCase struct {
		name        string
		nameVal     string
		expectedCfg *Config
		expectedErr string
	}

	metrics := map[string]*MetricConfig{
		"m3": {
			Unit: "By",
			Gauge: &GaugeMetric{
				ValueType: "double",
			},
			ScalarOIDs: []ScalarOID{
				{
					OID: "1",
				},
			},
		},
	}
	expectedConfigSimple := factory.CreateDefaultConfig().(*Config)
	expectedConfigSimple.Metrics = metrics

	expectedConfigInvalidEndpoint := factory.CreateDefaultConfig().(*Config)
	expectedConfigInvalidEndpoint.Endpoint = "udp://a:a:a:a:a:a"
	expectedConfigInvalidEndpoint.Metrics = metrics

	expectedConfigNoPort := factory.CreateDefaultConfig().(*Config)
	expectedConfigNoPort.Endpoint = "udp://localhost"
	expectedConfigNoPort.Metrics = metrics

	expectedConfigNoPortTrailingColon := factory.CreateDefaultConfig().(*Config)
	expectedConfigNoPortTrailingColon.Endpoint = "udp://localhost:"
	expectedConfigNoPortTrailingColon.Metrics = metrics

	expectedConfigBadEndpointScheme := factory.CreateDefaultConfig().(*Config)
	expectedConfigBadEndpointScheme.Endpoint = "http://localhost:161"
	expectedConfigBadEndpointScheme.Metrics = metrics

	expectedConfigNoEndpointScheme := factory.CreateDefaultConfig().(*Config)
	expectedConfigNoEndpointScheme.Endpoint = "localhost:161"
	expectedConfigNoEndpointScheme.Metrics = metrics

	expectedConfigBadVersion := factory.CreateDefaultConfig().(*Config)
	expectedConfigBadVersion.Version = "9999"
	expectedConfigBadVersion.Metrics = metrics

	expectedConfigV3NoUser := factory.CreateDefaultConfig().(*Config)
	expectedConfigV3NoUser.Version = "v3"
	expectedConfigV3NoUser.SecurityLevel = "no_auth_no_priv"
	expectedConfigV3NoUser.Metrics = metrics

	expectedConfigV3NoSecurityLevel := factory.CreateDefaultConfig().(*Config)
	expectedConfigV3NoSecurityLevel.Version = "v3"
	expectedConfigV3NoSecurityLevel.User = "u"
	expectedConfigV3NoSecurityLevel.Metrics = metrics

	expectedConfigV3BadSecurityLevel := factory.CreateDefaultConfig().(*Config)
	expectedConfigV3BadSecurityLevel.Version = "v3"
	expectedConfigV3BadSecurityLevel.SecurityLevel = "super"
	expectedConfigV3BadSecurityLevel.User = "u"
	expectedConfigV3BadSecurityLevel.Metrics = metrics

	expectedConfigV3NoAuthType := factory.CreateDefaultConfig().(*Config)
	expectedConfigV3NoAuthType.Version = "v3"
	expectedConfigV3NoAuthType.User = "u"
	expectedConfigV3NoAuthType.SecurityLevel = "auth_no_priv"
	expectedConfigV3NoAuthType.AuthPassword = "p"
	expectedConfigV3NoAuthType.Metrics = metrics

	expectedConfigV3BadAuthType := factory.CreateDefaultConfig().(*Config)
	expectedConfigV3BadAuthType.Version = "v3"
	expectedConfigV3BadAuthType.User = "u"
	expectedConfigV3BadAuthType.SecurityLevel = "auth_no_priv"
	expectedConfigV3BadAuthType.AuthType = "super"
	expectedConfigV3BadAuthType.AuthPassword = "p"
	expectedConfigV3BadAuthType.Metrics = metrics

	expectedConfigV3NoAuthPassword := factory.CreateDefaultConfig().(*Config)
	expectedConfigV3NoAuthPassword.Version = "v3"
	expectedConfigV3NoAuthPassword.User = "u"
	expectedConfigV3NoAuthPassword.SecurityLevel = "auth_no_priv"
	expectedConfigV3NoAuthPassword.Metrics = metrics

	expectedConfigV3Simple := factory.CreateDefaultConfig().(*Config)
	expectedConfigV3Simple.Version = "v3"
	expectedConfigV3Simple.User = "u"
	expectedConfigV3Simple.SecurityLevel = "auth_priv"
	expectedConfigV3Simple.AuthPassword = "p"
	expectedConfigV3Simple.PrivacyPassword = "pp"
	expectedConfigV3Simple.Metrics = metrics

	expectedConfigV3BadPrivacyType := factory.CreateDefaultConfig().(*Config)
	expectedConfigV3BadPrivacyType.Version = "v3"
	expectedConfigV3BadPrivacyType.User = "u"
	expectedConfigV3BadPrivacyType.SecurityLevel = "auth_priv"
	expectedConfigV3BadPrivacyType.AuthPassword = "p"
	expectedConfigV3BadPrivacyType.PrivacyType = "super"
	expectedConfigV3BadPrivacyType.PrivacyPassword = "pp"
	expectedConfigV3BadPrivacyType.Metrics = metrics

	expectedConfigV3NoPrivacyPassword := factory.CreateDefaultConfig().(*Config)
	expectedConfigV3NoPrivacyPassword.Version = "v3"
	expectedConfigV3NoPrivacyPassword.User = "u"
	expectedConfigV3NoPrivacyPassword.SecurityLevel = "auth_priv"
	expectedConfigV3NoPrivacyPassword.AuthPassword = "p"
	expectedConfigV3NoPrivacyPassword.Metrics = metrics

	testCases := []testCase{
		{
			name:        "NoEndpointUsesDefault",
			nameVal:     "no_endpoint",
			expectedCfg: expectedConfigSimple,
			expectedErr: "",
		},
		{
			name:        "InvalidEndpointErrors",
			nameVal:     "invalid_endpoint",
			expectedCfg: expectedConfigInvalidEndpoint,
			expectedErr: fmt.Sprintf(errMsgInvalidEndpoint[:len(errMsgInvalidEndpoint)-2], "udp://a:a:a:a:a:a"),
		},
		{
			name:        "NoPortErrors",
			nameVal:     "no_port",
			expectedCfg: expectedConfigNoPort,
			expectedErr: fmt.Sprintf(errMsgInvalidEndpoint[:len(errMsgInvalidEndpoint)-2], "udp://localhost"),
		},
		{
			name:        "NoPortTrailingColonErrors",
			nameVal:     "no_port_trailing_colon",
			expectedCfg: expectedConfigNoPortTrailingColon,
			expectedErr: fmt.Sprintf(errMsgInvalidEndpoint[:len(errMsgInvalidEndpoint)-2], "udp://localhost:"),
		},
		{
			name:        "BadEndpointSchemeErrors",
			nameVal:     "bad_endpoint_scheme",
			expectedCfg: expectedConfigBadEndpointScheme,
			expectedErr: errEndpointBadScheme.Error(),
		},
		{
			name:        "NoEndpointSchemeErrors",
			nameVal:     "no_endpoint_scheme",
			expectedCfg: expectedConfigNoEndpointScheme,
			expectedErr: fmt.Sprintf(errMsgInvalidEndpoint[:len(errMsgInvalidEndpoint)-2], "localhost:161"),
		},
		{
			name:        "NoVersionUsesDefault",
			nameVal:     "no_version",
			expectedCfg: expectedConfigSimple,
			expectedErr: "",
		},
		{
			name:        "BadVersionErrors",
			nameVal:     "bad_version",
			expectedCfg: expectedConfigBadVersion,
			expectedErr: errBadVersion.Error(),
		},
		{
			name:        "V3NoUserErrors",
			nameVal:     "v3_no_user",
			expectedCfg: expectedConfigV3NoUser,
			expectedErr: errEmptyUser.Error(),
		},
		{
			name:        "V3NoSecurityLevelUsesDefault",
			nameVal:     "v3_no_security_level",
			expectedCfg: expectedConfigV3NoSecurityLevel,
			expectedErr: "",
		},
		{
			name:        "V3BadSecurityLevelErrors",
			nameVal:     "v3_bad_security_level",
			expectedCfg: expectedConfigV3BadSecurityLevel,
			expectedErr: errBadSecurityLevel.Error(),
		},
		{
			name:        "V3NoAuthTypeUsesDefault",
			nameVal:     "v3_no_auth_type",
			expectedCfg: expectedConfigV3NoAuthType,
			expectedErr: "",
		},
		{
			name:        "V3BadAuthTypeErrors",
			nameVal:     "v3_bad_auth_type",
			expectedCfg: expectedConfigV3BadAuthType,
			expectedErr: errBadAuthType.Error(),
		},
		{
			name:        "V3NoAuthPasswordErrors",
			nameVal:     "v3_no_auth_password",
			expectedCfg: expectedConfigV3NoAuthPassword,
			expectedErr: errEmptyAuthPassword.Error(),
		},
		{
			name:        "V3NoPrivacyTypeUsesDefault",
			nameVal:     "v3_no_privacy_type",
			expectedCfg: expectedConfigV3Simple,
			expectedErr: "",
		},
		{
			name:        "V3BadPrivacyTypeErrors",
			nameVal:     "v3_bad_privacy_type",
			expectedCfg: expectedConfigV3BadPrivacyType,
			expectedErr: errBadPrivacyType.Error(),
		},
		{
			name:        "V3NoPrivacyPasswordErrors",
			nameVal:     "v3_no_privacy_password",
			expectedCfg: expectedConfigV3NoPrivacyPassword,
			expectedErr: errEmptyPrivacyPassword.Error(),
		},
		{
			name:        "GoodV2CConnectionNoErrors",
			nameVal:     "v2c_connection_good",
			expectedCfg: expectedConfigSimple,
			expectedErr: "",
		},
		{
			name:        "GoodV3ConnectionNoErrors",
			nameVal:     "v3_connection_good",
			expectedCfg: expectedConfigV3Simple,
			expectedErr: "",
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			sub, err := cm.Sub(component.NewIDWithName(metadata.Type, test.nameVal).String())
			require.NoError(t, err)

			cfg := factory.CreateDefaultConfig()
			require.NoError(t, component.UnmarshalConfig(sub, cfg))
			if test.expectedErr == "" {
				require.NoError(t, component.ValidateConfig(cfg))
			} else {
				require.ErrorContains(t, component.ValidateConfig(cfg), test.expectedErr)
			}

			require.Equal(t, test.expectedCfg, cfg)
		})
	}
}

func getBaseMetricConfig(gauge bool, scalar bool) map[string]*MetricConfig {
	metricCfg := map[string]*MetricConfig{
		"m3": {
			Unit: "By",
		},
	}

	if gauge {
		metricCfg["m3"].Gauge = &GaugeMetric{
			ValueType: "double",
		}
	} else {
		metricCfg["m3"].Sum = &SumMetric{
			Aggregation: "cumulative",
			Monotonic:   true,
			ValueType:   "double",
		}
	}

	if scalar {
		metricCfg["m3"].ScalarOIDs = []ScalarOID{
			{
				OID: "1",
			},
		}
	} else {
		metricCfg["m3"].ColumnOIDs = []ColumnOID{
			{
				OID: "1",
			},
		}
	}

	return metricCfg
}

func getBaseAttrConfig(attrType string) map[string]*AttributeConfig {
	switch attrType {
	case "oid":
		return map[string]*AttributeConfig{
			"a2": {
				OID: "1",
			},
		}
	case "prefix":
		return map[string]*AttributeConfig{
			"a2": {
				IndexedValuePrefix: "p",
			},
		}
	default:
		return map[string]*AttributeConfig{
			"a2": {
				Enum: []string{"val1", "val2"},
			},
		}
	}
}

func getBaseResourceAttrConfig(attrType string) map[string]*ResourceAttributeConfig {
	switch attrType {
	case "oid":
		return map[string]*ResourceAttributeConfig{
			"ra1": {
				OID: "2",
			},
		}
	default:
		return map[string]*ResourceAttributeConfig{
			"ra1": {
				IndexedValuePrefix: "p",
			},
		}
	}
}

func TestLoadConfigMetricConfigs(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	factory := NewFactory()
	type testCase struct {
		name        string
		nameVal     string
		expectedCfg *Config
		expectedErr string
	}

	expectedConfigNoMetricConfig := factory.CreateDefaultConfig().(*Config)

	expectedConfigNoMetricUnit := factory.CreateDefaultConfig().(*Config)
	expectedConfigNoMetricUnit.Metrics = getBaseMetricConfig(true, true)
	expectedConfigNoMetricUnit.Metrics["m3"].Unit = ""

	expectedConfigNoMetricGaugeOrSum := factory.CreateDefaultConfig().(*Config)
	expectedConfigNoMetricGaugeOrSum.Metrics = getBaseMetricConfig(true, true)
	expectedConfigNoMetricGaugeOrSum.Metrics["m3"].Gauge = nil

	expectedConfigNoMetricOIDs := factory.CreateDefaultConfig().(*Config)
	expectedConfigNoMetricOIDs.Metrics = getBaseMetricConfig(true, true)
	expectedConfigNoMetricOIDs.Metrics["m3"].ScalarOIDs = nil

	expectedConfigNoMetricGaugeType := factory.CreateDefaultConfig().(*Config)
	expectedConfigNoMetricGaugeType.Metrics = getBaseMetricConfig(true, true)
	expectedConfigNoMetricGaugeType.Metrics["m3"].Gauge.ValueType = ""

	expectedConfigBadMetricGaugeType := factory.CreateDefaultConfig().(*Config)
	expectedConfigBadMetricGaugeType.Metrics = getBaseMetricConfig(true, true)
	expectedConfigBadMetricGaugeType.Metrics["m3"].Gauge.ValueType = "Counter"

	expectedConfigNoMetricSumType := factory.CreateDefaultConfig().(*Config)
	expectedConfigNoMetricSumType.Metrics = getBaseMetricConfig(false, true)
	expectedConfigNoMetricSumType.Metrics["m3"].Sum.ValueType = ""

	expectedConfigBadMetricSumType := factory.CreateDefaultConfig().(*Config)
	expectedConfigBadMetricSumType.Metrics = getBaseMetricConfig(false, true)
	expectedConfigBadMetricSumType.Metrics["m3"].Sum.ValueType = "Counter"

	expectedConfigNoMetricSumAggregation := factory.CreateDefaultConfig().(*Config)
	expectedConfigNoMetricSumAggregation.Metrics = getBaseMetricConfig(false, true)
	expectedConfigNoMetricSumAggregation.Metrics["m3"].Sum.Aggregation = ""

	expectedConfigBadMetricSumAggregation := factory.CreateDefaultConfig().(*Config)
	expectedConfigBadMetricSumAggregation.Metrics = getBaseMetricConfig(false, true)
	expectedConfigBadMetricSumAggregation.Metrics["m3"].Sum.Aggregation = "Counter"

	expectedConfigNoScalarOIDOID := factory.CreateDefaultConfig().(*Config)
	expectedConfigNoScalarOIDOID.Metrics = getBaseMetricConfig(true, true)
	expectedConfigNoScalarOIDOID.Metrics["m3"].ScalarOIDs[0].OID = ""

	expectedConfigNoAttrOIDPrefixOrEnum := factory.CreateDefaultConfig().(*Config)
	expectedConfigNoAttrOIDPrefixOrEnum.Metrics = getBaseMetricConfig(true, true)
	expectedConfigNoAttrOIDPrefixOrEnum.Attributes = getBaseAttrConfig("oid")
	expectedConfigNoAttrOIDPrefixOrEnum.Attributes["a2"].OID = ""

	expectedConfigNoScalarOIDAttrName := factory.CreateDefaultConfig().(*Config)
	expectedConfigNoScalarOIDAttrName.Metrics = getBaseMetricConfig(true, true)
	expectedConfigNoScalarOIDAttrName.Metrics["m3"].ScalarOIDs[0].Attributes = []Attribute{
		{
			Value: "val1",
		},
	}

	expectedConfigBadScalarOIDAttrName := factory.CreateDefaultConfig().(*Config)
	expectedConfigBadScalarOIDAttrName.Metrics = getBaseMetricConfig(true, true)
	expectedConfigBadScalarOIDAttrName.Attributes = getBaseAttrConfig("enum")
	expectedConfigBadScalarOIDAttrName.Metrics["m3"].ScalarOIDs[0].Attributes = []Attribute{
		{
			Name:  "a1",
			Value: "val1",
		},
	}

	expectedConfigBadScalarOIDAttr := factory.CreateDefaultConfig().(*Config)
	expectedConfigBadScalarOIDAttr.Metrics = getBaseMetricConfig(true, true)
	expectedConfigBadScalarOIDAttr.Attributes = getBaseAttrConfig("oid")
	expectedConfigBadScalarOIDAttr.Metrics["m3"].ScalarOIDs[0].Attributes = []Attribute{
		{
			Name:  "a2",
			Value: "val1",
		},
	}

	expectedConfigBadScalarOIDAttrValue := factory.CreateDefaultConfig().(*Config)
	expectedConfigBadScalarOIDAttrValue.Metrics = getBaseMetricConfig(true, true)
	expectedConfigBadScalarOIDAttrValue.Attributes = getBaseAttrConfig("enum")
	expectedConfigBadScalarOIDAttrValue.Metrics["m3"].ScalarOIDs[0].Attributes = []Attribute{
		{
			Name:  "a2",
			Value: "val3",
		},
	}

	expectedConfigNoColumnOIDOID := factory.CreateDefaultConfig().(*Config)
	expectedConfigNoColumnOIDOID.Metrics = getBaseMetricConfig(true, false)
	expectedConfigNoColumnOIDOID.Metrics["m3"].ColumnOIDs[0].OID = ""

	expectedConfigNoColumnOIDAttrName := factory.CreateDefaultConfig().(*Config)
	expectedConfigNoColumnOIDAttrName.Metrics = getBaseMetricConfig(true, false)
	expectedConfigNoColumnOIDAttrName.Metrics["m3"].ColumnOIDs[0].Attributes = []Attribute{{}}

	expectedConfigBadColumnOIDAttrName := factory.CreateDefaultConfig().(*Config)
	expectedConfigBadColumnOIDAttrName.Metrics = getBaseMetricConfig(true, false)
	expectedConfigBadColumnOIDAttrName.Attributes = getBaseAttrConfig("oid")
	expectedConfigBadColumnOIDAttrName.Metrics["m3"].ColumnOIDs[0].Attributes = []Attribute{
		{
			Name: "a1",
		},
	}

	expectedConfigBadColumnOIDAttrValue := factory.CreateDefaultConfig().(*Config)
	expectedConfigBadColumnOIDAttrValue.Metrics = getBaseMetricConfig(true, false)
	expectedConfigBadColumnOIDAttrValue.Attributes = getBaseAttrConfig("enum")
	expectedConfigBadColumnOIDAttrValue.Metrics["m3"].ColumnOIDs[0].Attributes = []Attribute{
		{
			Name:  "a2",
			Value: "val3",
		},
	}

	expectedConfigBadColumnOIDResourceAttrName := factory.CreateDefaultConfig().(*Config)
	expectedConfigBadColumnOIDResourceAttrName.Metrics = getBaseMetricConfig(true, false)
	expectedConfigBadColumnOIDResourceAttrName.ResourceAttributes = getBaseResourceAttrConfig("oid")
	expectedConfigBadColumnOIDResourceAttrName.Metrics["m3"].ColumnOIDs[0].ResourceAttributes = []string{"a2"}

	expectedConfigColumnOIDWithoutIndexAttributeOrResourceAttribute := factory.CreateDefaultConfig().(*Config)
	expectedConfigColumnOIDWithoutIndexAttributeOrResourceAttribute.Metrics = getBaseMetricConfig(true, false)
	expectedConfigColumnOIDWithoutIndexAttributeOrResourceAttribute.Attributes = getBaseAttrConfig("enum")
	expectedConfigColumnOIDWithoutIndexAttributeOrResourceAttribute.Metrics["m3"].ColumnOIDs[0].Attributes = []Attribute{
		{
			Name:  "a2",
			Value: "val1",
		},
	}

	expectedConfigNoResourceAttributeOIDOrPrefix := factory.CreateDefaultConfig().(*Config)
	expectedConfigNoResourceAttributeOIDOrPrefix.Metrics = getBaseMetricConfig(true, false)
	expectedConfigNoResourceAttributeOIDOrPrefix.ResourceAttributes = getBaseResourceAttrConfig("oid")
	expectedConfigNoResourceAttributeOIDOrPrefix.ResourceAttributes["ra1"].OID = ""
	expectedConfigNoResourceAttributeOIDOrPrefix.Metrics["m3"].ColumnOIDs[0].ResourceAttributes = []string{"ra1"}

	expectedConfigComplexGood := factory.CreateDefaultConfig().(*Config)
	expectedConfigComplexGood.ResourceAttributes = getBaseResourceAttrConfig("prefix")
	expectedConfigComplexGood.ResourceAttributes["ra2"] = &ResourceAttributeConfig{OID: "1"}
	expectedConfigComplexGood.Attributes = getBaseAttrConfig("enum")
	expectedConfigComplexGood.Attributes["a1"] = &AttributeConfig{
		Value: "v",
		Enum:  []string{"val1"},
	}
	expectedConfigComplexGood.Attributes["a3"] = &AttributeConfig{IndexedValuePrefix: "p"}
	expectedConfigComplexGood.Attributes["a4"] = &AttributeConfig{OID: "1"}
	expectedConfigComplexGood.Metrics = getBaseMetricConfig(true, true)
	expectedConfigComplexGood.Metrics["m3"].ScalarOIDs[0].Attributes = []Attribute{
		{
			Name:  "a1",
			Value: "val1",
		},
	}
	expectedConfigComplexGood.Metrics["m1"] = &MetricConfig{
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
	}
	expectedConfigComplexGood.Metrics["m2"] = &MetricConfig{
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
						Name:  "a2",
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
						Name:  "a2",
						Value: "val2",
					},
				},
			},
		},
	}
	expectedConfigComplexGood.Metrics["m4"] = &MetricConfig{
		Unit: "{things}",
		Sum: &SumMetric{
			Aggregation: "cumulative",
			Monotonic:   true,
			ValueType:   "int",
		},
		ScalarOIDs: []ScalarOID{
			{
				OID: "1",
			},
		},
	}
	expectedConfigComplexGood.Metrics["m5"] = &MetricConfig{
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
						Name:  "a1",
						Value: "val1",
					},
				},
			},
		},
	}
	expectedConfigComplexGood.Metrics["m6"] = &MetricConfig{
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
						Name:  "a2",
						Value: "val1",
					},
				},
			},
			{
				OID: "2",
				Attributes: []Attribute{
					{
						Name:  "a2",
						Value: "val2",
					},
				},
			},
		},
	}
	expectedConfigComplexGood.Metrics["m7"] = &MetricConfig{
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
	}
	expectedConfigComplexGood.Metrics["m8"] = &MetricConfig{
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
	}
	expectedConfigComplexGood.Metrics["m9"] = &MetricConfig{
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
	}
	expectedConfigComplexGood.Metrics["m10"] = &MetricConfig{
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
						Name:  "a1",
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
	}

	testCases := []testCase{
		{
			name:        "NoMetricConfigsErrors",
			nameVal:     "no_metric_config",
			expectedCfg: expectedConfigNoMetricConfig,
			expectedErr: errMetricRequired.Error(),
		},
		{
			name:        "NoMetricUnitErrors",
			nameVal:     "no_metric_unit",
			expectedCfg: expectedConfigNoMetricUnit,
			expectedErr: fmt.Sprintf(errMsgMetricNoUnit, "m3"),
		},
		{
			name:        "NoMetricGaugeOrSumErrors",
			nameVal:     "no_metric_gauge_or_sum",
			expectedCfg: expectedConfigNoMetricGaugeOrSum,
			expectedErr: fmt.Sprintf(errMsgMetricNoGaugeOrSum, "m3"),
		},
		{
			name:        "NoMetricOIDsErrors",
			nameVal:     "no_metric_oids",
			expectedCfg: expectedConfigNoMetricOIDs,
			expectedErr: fmt.Sprintf(errMsgMetricNoOIDs, "m3"),
		},
		{
			name:        "NoMetricGaugeTypeErrors",
			nameVal:     "no_metric_gauge_type",
			expectedCfg: expectedConfigNoMetricGaugeType,
			expectedErr: fmt.Sprintf(errMsgGaugeBadValueType, "m3"),
		},
		{
			name:        "BadMetricGaugeTypeErrors",
			nameVal:     "bad_metric_gauge_type",
			expectedCfg: expectedConfigBadMetricGaugeType,
			expectedErr: fmt.Sprintf(errMsgGaugeBadValueType, "m3"),
		},
		{
			name:        "NoMetricSumTypeErrors",
			nameVal:     "no_metric_sum_type",
			expectedCfg: expectedConfigNoMetricSumType,
			expectedErr: fmt.Sprintf(errMsgSumBadValueType, "m3"),
		},
		{
			name:        "BadMetricSumTypeErrors",
			nameVal:     "bad_metric_sum_type",
			expectedCfg: expectedConfigBadMetricSumType,
			expectedErr: fmt.Sprintf(errMsgSumBadValueType, "m3"),
		},
		{
			name:        "NoMetricSumAggregationErrors",
			nameVal:     "no_metric_sum_aggregation",
			expectedCfg: expectedConfigNoMetricSumAggregation,
			expectedErr: fmt.Sprintf(errMsgSumBadAggregation, "m3"),
		},
		{
			name:        "BadMetricSumAggregationErrors",
			nameVal:     "bad_metric_sum_aggregation",
			expectedCfg: expectedConfigBadMetricSumAggregation,
			expectedErr: fmt.Sprintf(errMsgSumBadAggregation, "m3"),
		},
		{
			name:        "NoScalarOIDOIDErrors",
			nameVal:     "no_scalar_oid_oid",
			expectedCfg: expectedConfigNoScalarOIDOID,
			expectedErr: fmt.Sprintf(errMsgScalarOIDNoOID, "m3"),
		},
		{
			name:        "NoAttributeConfigOIDPrefixOrEnumsErrors",
			nameVal:     "no_attribute_oid_prefix_or_enums",
			expectedCfg: expectedConfigNoAttrOIDPrefixOrEnum,
			expectedErr: fmt.Sprintf(errMsgAttributeConfigNoEnumOIDOrPrefix, "a2"),
		},
		{
			name:        "NoScalarOIDAttributeNameErrors",
			nameVal:     "no_scalar_oid_attribute_name",
			expectedCfg: expectedConfigNoScalarOIDAttrName,
			expectedErr: fmt.Sprintf(errMsgScalarAttributeNoName, "m3"),
		},
		{
			name:        "BadScalarOIDAttributeNameErrors",
			nameVal:     "bad_scalar_oid_attribute_name",
			expectedCfg: expectedConfigBadScalarOIDAttrName,
			expectedErr: fmt.Sprintf(errMsgScalarAttributeBadName, "m3", "a1"),
		},
		{
			name:        "BadScalarOIDAttributeErrors",
			nameVal:     "bad_scalar_oid_attribute",
			expectedCfg: expectedConfigBadScalarOIDAttr,
			expectedErr: fmt.Sprintf(errMsgScalarOIDBadAttribute, "m3", "a2"),
		},
		{
			name:        "BadScalarOIDAttributeValueErrors",
			nameVal:     "bad_scalar_oid_attribute_value",
			expectedCfg: expectedConfigBadScalarOIDAttrValue,
			expectedErr: fmt.Sprintf(errMsgScalarAttributeBadValue, "m3", "a2", "val3"),
		},
		{
			name:        "NoColumnOIDOIDErrors",
			nameVal:     "no_column_oid_oid",
			expectedCfg: expectedConfigNoColumnOIDOID,
			expectedErr: fmt.Sprintf(errMsgColumnOIDNoOID, "m3"),
		},
		{
			name:        "NoColumnOIDAttributeNameErrors",
			nameVal:     "no_column_oid_attribute_name",
			expectedCfg: expectedConfigNoColumnOIDAttrName,
			expectedErr: fmt.Sprintf(errMsgColumnAttributeNoName, "m3"),
		},
		{
			name:        "BadColumnOIDAttributeNameErrors",
			nameVal:     "bad_column_oid_attribute_name",
			expectedCfg: expectedConfigBadColumnOIDAttrName,
			expectedErr: fmt.Sprintf(errMsgColumnAttributeBadName, "m3", "a1"),
		},
		{
			name:        "BadColumnOIDAttributeValueErrors",
			nameVal:     "bad_column_oid_attribute_value",
			expectedCfg: expectedConfigBadColumnOIDAttrValue,
			expectedErr: fmt.Sprintf(errMsgColumnAttributeBadValue, "m3", "a2", "val3"),
		},
		{
			name:        "BadColumnOIDResourceAttributeNameErrors",
			nameVal:     "bad_column_oid_resource_attribute_name",
			expectedCfg: expectedConfigBadColumnOIDResourceAttrName,
			expectedErr: fmt.Sprintf(errMsgColumnResourceAttributeBadName, "m3", "a2"),
		},
		{
			name:        "ColumnOIDWithoutIndexedAttributeOrResourceAttributeErrors",
			nameVal:     "column_oid_no_indexed_attribute_or_resource_attribute",
			expectedCfg: expectedConfigColumnOIDWithoutIndexAttributeOrResourceAttribute,
			expectedErr: fmt.Sprintf(errMsgColumnIndexedAttributeRequired, "m3"),
		},
		{
			name:        "NoResourceAttributeConfigOIDOrPrefixErrors",
			nameVal:     "no_resource_attribute_oid_or_prefix",
			expectedCfg: expectedConfigNoResourceAttributeOIDOrPrefix,
			expectedErr: fmt.Sprintf(errMsgResourceAttributeNoOIDOrPrefix, "ra1"),
		},
		{
			name:        "ComplexConfigGood",
			nameVal:     "complex_good",
			expectedCfg: expectedConfigComplexGood,
			expectedErr: "",
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			sub, err := cm.Sub(component.NewIDWithName(metadata.Type, test.nameVal).String())
			require.NoError(t, err)

			cfg := factory.CreateDefaultConfig()
			require.NoError(t, component.UnmarshalConfig(sub, cfg))
			if test.expectedErr == "" {
				require.NoError(t, component.ValidateConfig(cfg))
			} else {
				require.ErrorContains(t, component.ValidateConfig(cfg), test.expectedErr)
			}

			require.Equal(t, test.expectedCfg, cfg)
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
				Metrics: map[string]*MetricConfig{
					"m3": {
						Unit: "By",
						Gauge: &GaugeMetric{
							ValueType: "double",
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
				Metrics: map[string]*MetricConfig{
					"m3": {
						Unit: "By",
						Gauge: &GaugeMetric{
							ValueType: "double",
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
				Metrics: map[string]*MetricConfig{
					"m3": {
						Unit: "By",
						Gauge: &GaugeMetric{
							ValueType: "double",
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
				Metrics: map[string]*MetricConfig{
					"m3": {
						Unit: "By",
						Gauge: &GaugeMetric{
							ValueType: "double",
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
				Metrics: map[string]*MetricConfig{
					"m3": {
						Unit: "By",
						Gauge: &GaugeMetric{
							ValueType: "double",
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
