// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspan"
)

const (
	testServiceInstanceID = "627cc493-f310-47de-96bd-71410b7dec09"
	testServiceName       = "testsignaltometrics"
	testNamespace         = "test"
)

func TestFilterResourceAttributes(t *testing.T) {
	inputAttributes := map[string]any{
		"key.1": "val.1",
		"key.2": true,
		"key.3": 11,
		"key.4": "val.4",
	}
	for _, tc := range []struct {
		name                      string
		includeResourceAttributes []AttributeKeyValue
		expected                  map[string]any
	}{
		{
			name: "no_include_resource_attributes_configured",
			expected: map[string]any{
				// All resource attributes will be added as no
				// include resource attributes specified
				"key.1": "val.1",
				"key.2": true,
				"key.3": int64(11),
				"key.4": "val.4",
				// Collector instance info will be added
				"signaltometrics.service.instance.id": testServiceInstanceID,
				"signaltometrics.service.name":        testServiceName,
				"signaltometrics.service.namespace":   testNamespace,
			},
		},
		{
			name: "include_resource_attributes_configured",
			includeResourceAttributes: []AttributeKeyValue{
				testAttributeKeyValue(t, "key.1", false, nil),
				// Optional does not change the behavior for resource attribute
				// filtering.
				testAttributeKeyValue(t, "key.2", true, nil),
				// With default value configured and the attribute present, default
				// value will be ignored.
				testAttributeKeyValue(t, "key.3", false, 500),
				// With default value, even if the attribute is missing, it will
				// be added to the output.
				testAttributeKeyValue(t, "key.302", false, "anything"),
				// Without default value, the resource attribute will be ignored
				// if not present in the input
				testAttributeKeyValue(t, "key.404", false, nil),
				// Optional does not change the behavior for resource attribute
				// filtering.
				testAttributeKeyValue(t, "key.412", true, nil),
			},
			expected: map[string]any{
				// Include resource attributes are filtered
				"key.1": "val.1",
				// Optional attributes are copied if present
				"key.2": true,
				// Default value is ignored if attribute is present
				"key.3": int64(11),
				// Resource attributes with default values are added
				"key.302": "anything",
				// Collector instance info will be added
				"signaltometrics.service.instance.id": testServiceInstanceID,
				"signaltometrics.service.name":        testServiceName,
				"signaltometrics.service.namespace":   testNamespace,
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			md := MetricDef[ottlspan.TransformContext]{
				IncludeResourceAttributes: tc.includeResourceAttributes,
			}
			inputResourceAttrsM := pcommon.NewMap()
			require.NoError(t, inputResourceAttrsM.FromRaw(inputAttributes))
			actual := md.FilterResourceAttributes(
				inputResourceAttrsM,
				testCollectorInstanceInfo(t),
			)
			assert.Empty(t, cmp.Diff(tc.expected, actual.AsRaw()))
		})
	}
}

func TestFilterAttributes(t *testing.T) {
	inputAttributes := map[string]any{
		"key.1": "val.1",
		"key.2": true,
		"key.3": 11,
		"key.4": "val.4",
	}
	for _, tc := range []struct {
		name               string
		attributes         []AttributeKeyValue
		expectedDecision   bool           // whether to process the entity or not
		expectedAttributes map[string]any // map of filtered attributes
	}{
		{
			name:               "no_attribute_configured",
			expectedDecision:   true,
			expectedAttributes: map[string]any{
				// No attributes are filtered since nothing is configured.
			},
		},
		{
			// Attribute filter would process an entity (span, log record,
			// datapoint) iff all the configured attributes, other than with
			// optional, are present. If any of the required attributes
			// are absent in the incoming entity then the entity is not
			// processed.
			name: "attributes_configured_but_missing",
			attributes: []AttributeKeyValue{
				testAttributeKeyValue(t, "key.1", false, nil),
				// The below key has optional defined so should not
				// affect the processing decision.
				testAttributeKeyValue(t, "key.2", true, nil),
				// Below attribute would be missing in the input metric and
				// should return false to signal not to process the entity.
				testAttributeKeyValue(t, "key.404", false, nil),
			},
			expectedDecision: false,
		},
		{
			name: "attributes_configured_with_optional",
			attributes: []AttributeKeyValue{
				testAttributeKeyValue(t, "key.1", false, nil),
				// The below two key has optional defined so should not
				// affect the processing decision.
				testAttributeKeyValue(t, "key.2", true, nil),
				testAttributeKeyValue(t, "key.412", true, nil),
			},
			expectedDecision: true,
			expectedAttributes: map[string]any{
				"key.1": "val.1",
				"key.2": true,
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			md := MetricDef[ottlspan.TransformContext]{
				Attributes: tc.attributes,
			}
			inputAttrM := pcommon.NewMap()
			require.NoError(t, inputAttrM.FromRaw(inputAttributes))
			actual, ok := md.FilterAttributes(inputAttrM)
			assert.Equal(t, tc.expectedDecision, ok)
			if ok {
				// Only if the decision is true, the map should be compared
				assert.Empty(t, cmp.Diff(tc.expectedAttributes, actual.AsRaw()))
			}
		})
	}
}

func testCollectorInstanceInfo(t *testing.T) CollectorInstanceInfo {
	t.Helper()

	set := componenttest.NewNopTelemetrySettings()
	set.Resource.Attributes().PutStr(string(semconv.ServiceInstanceIDKey), testServiceInstanceID)
	set.Resource.Attributes().PutStr(string(semconv.ServiceNameKey), testServiceName)
	set.Resource.Attributes().PutStr(string(semconv.ServiceNamespaceKey), testNamespace)
	return NewCollectorInstanceInfo(set)
}

func testAttributeKeyValue(
	t *testing.T,
	k string,
	optional bool,
	val any,
) AttributeKeyValue {
	t.Helper()

	defaultVal := pcommon.NewValueEmpty()
	if val != nil {
		require.NoError(t, defaultVal.FromRaw(val))
	}
	return AttributeKeyValue{
		Key:          k,
		Optional:     optional,
		DefaultValue: defaultVal,
	}
}
