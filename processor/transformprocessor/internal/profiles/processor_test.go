// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package profiles

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pprofile"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlprofile"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pprofiletest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/common"
)

var (
	TestProfileTime      = time.Date(2020, 2, 11, 20, 26, 12, 321, time.UTC)
	TestProfileTimestamp = pcommon.NewTimestampFromTime(TestProfileTime)

	TestObservedTime      = time.Date(2020, 2, 11, 20, 26, 13, 789, time.UTC)
	TestObservedTimestamp = pcommon.NewTimestampFromTime(TestObservedTime)

	profileID = [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}

	DefaultProfileFunctions = ProfileFunctions()
)

func Test_ProcessProfiles_ResourceContext(t *testing.T) {
	tests := []struct {
		statement string
		want      func(td pprofile.Profiles)
	}{
		{
			statement: `set(attributes["test"], "pass")`,
			want: func(td pprofile.Profiles) {
				td.ResourceProfiles().At(0).Resource().Attributes().PutStr("test", "pass")
			},
		},
		{
			statement: `set(attributes["test"], "pass") where attributes["host.name"] == "wrong"`,
			want: func(pprofile.Profiles) {
			},
		},
		{
			statement: `set(schema_url, "test_schema_url")`,
			want: func(td pprofile.Profiles) {
				td.ResourceProfiles().At(0).SetSchemaUrl("test_schema_url")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.statement, func(t *testing.T) {
			td := constructProfiles()
			processor, err := NewProcessor([]common.ContextStatements{{Context: "resource", Statements: []string{tt.statement}}}, ottl.IgnoreError, componenttest.NewNopTelemetrySettings(), DefaultProfileFunctions)
			require.NoError(t, err)

			_, err = processor.ProcessProfiles(t.Context(), td)
			assert.NoError(t, err)

			exTd := constructProfiles()
			tt.want(exTd)

			assert.Equal(t, exTd, td)
		})
	}
}

func Test_ProcessProfiles_InferredResourceContext(t *testing.T) {
	tests := []struct {
		statement string
		want      func(td pprofile.Profiles)
	}{
		{
			statement: `set(resource.attributes["test"], "pass")`,
			want: func(td pprofile.Profiles) {
				td.ResourceProfiles().At(0).Resource().Attributes().PutStr("test", "pass")
			},
		},
		{
			statement: `set(resource.attributes["test"], "pass") where resource.attributes["host.name"] == "wrong"`,
			want: func(pprofile.Profiles) {
			},
		},
		{
			statement: `set(resource.schema_url, "test_schema_url")`,
			want: func(td pprofile.Profiles) {
				td.ResourceProfiles().At(0).SetSchemaUrl("test_schema_url")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.statement, func(t *testing.T) {
			td := constructProfiles()
			processor, err := NewProcessor([]common.ContextStatements{{Context: "", Statements: []string{tt.statement}}}, ottl.IgnoreError, componenttest.NewNopTelemetrySettings(), DefaultProfileFunctions)
			require.NoError(t, err)

			_, err = processor.ProcessProfiles(t.Context(), td)
			assert.NoError(t, err)

			exTd := constructProfiles()
			tt.want(exTd)

			assert.Equal(t, exTd, td)
		})
	}
}

func Test_ProcessProfiles_ScopeContext(t *testing.T) {
	tests := []struct {
		statement string
		want      func(td pprofile.Profiles)
	}{
		{
			statement: `set(attributes["test"], "pass") where name == "scope"`,
			want: func(td pprofile.Profiles) {
				td.ResourceProfiles().At(0).ScopeProfiles().At(0).Scope().Attributes().PutStr("test", "pass")
			},
		},
		{
			statement: `set(attributes["test"], "pass") where version == 2`,
			want: func(pprofile.Profiles) {
			},
		},
		{
			statement: `set(schema_url, "test_schema_url")`,
			want: func(td pprofile.Profiles) {
				td.ResourceProfiles().At(0).ScopeProfiles().At(0).SetSchemaUrl("test_schema_url")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.statement, func(t *testing.T) {
			td := constructProfiles()
			processor, err := NewProcessor([]common.ContextStatements{{Context: "scope", Statements: []string{tt.statement}}}, ottl.IgnoreError, componenttest.NewNopTelemetrySettings(), DefaultProfileFunctions)
			require.NoError(t, err)

			_, err = processor.ProcessProfiles(t.Context(), td)
			assert.NoError(t, err)

			exTd := constructProfiles()
			tt.want(exTd)

			assert.Equal(t, exTd, td)
		})
	}
}

func Test_ProcessProfiles_InferredScopeContext(t *testing.T) {
	tests := []struct {
		statement string
		want      func(td pprofile.Profiles)
	}{
		{
			statement: `set(scope.attributes["test"], "pass") where scope.name == "scope"`,
			want: func(td pprofile.Profiles) {
				td.ResourceProfiles().At(0).ScopeProfiles().At(0).Scope().Attributes().PutStr("test", "pass")
			},
		},
		{
			statement: `set(scope.attributes["test"], "pass") where scope.version == 2`,
			want: func(pprofile.Profiles) {
			},
		},
		{
			statement: `set(scope.schema_url, "test_schema_url")`,
			want: func(td pprofile.Profiles) {
				td.ResourceProfiles().At(0).ScopeProfiles().At(0).SetSchemaUrl("test_schema_url")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.statement, func(t *testing.T) {
			td := constructProfiles()
			processor, err := NewProcessor([]common.ContextStatements{{Context: "", Statements: []string{tt.statement}}}, ottl.IgnoreError, componenttest.NewNopTelemetrySettings(), DefaultProfileFunctions)
			require.NoError(t, err)

			_, err = processor.ProcessProfiles(t.Context(), td)
			assert.NoError(t, err)

			exTd := constructProfiles()
			tt.want(exTd)

			assert.Equal(t, exTd, td)
		})
	}
}

func Test_ProcessProfiles_ProfileContext(t *testing.T) {
	tests := []struct {
		statement string
		want      func(td pprofile.Profiles)
	}{
		{
			statement: `set(attributes["test"], "pass") where original_payload_format == "operationA"`,
			want: func(td pprofile.Profiles) {
				putProfileAttribute(t, td, 0, "test", "pass")
			},
		},
		{
			statement: `set(original_payload_format, "pass") where original_payload_format == "operationA"`,
			want: func(td pprofile.Profiles) {
				td.ResourceProfiles().At(0).ScopeProfiles().At(0).Profiles().At(0).SetOriginalPayloadFormat("pass")
			},
		},
		{
			statement: `set(attributes["test"], "pass") where resource.attributes["host.name"] == "localhost"`,
			want: func(td pprofile.Profiles) {
				putProfileAttribute(t, td, 0, "test", "pass")
				putProfileAttribute(t, td, 1, "test", "pass")
			},
		},
		{
			statement: `set(original_payload_format, "pass") where resource.attributes["host.name"] == "localhost"`,
			want: func(td pprofile.Profiles) {
				td.ResourceProfiles().At(0).ScopeProfiles().At(0).Profiles().At(0).SetOriginalPayloadFormat("pass")
				td.ResourceProfiles().At(0).ScopeProfiles().At(0).Profiles().At(1).SetOriginalPayloadFormat("pass")
			},
		},
		{
			statement: `keep_keys(attributes, ["http.method"]) where original_payload_format == "operationA"`,
			want: func(td pprofile.Profiles) {
				clearProfileAttributes(td, 0)
				putProfileAttribute(t, td, 0, "http.method", "get")
			},
		},
		{
			statement: `replace_pattern(attributes["http.method"], "get", "post")`,
			want: func(td pprofile.Profiles) {
				putProfileAttribute(t, td, 0, "http.method", "post")
				putProfileAttribute(t, td, 1, "http.method", "post")
			},
		},
		{
			statement: `replace_all_patterns(attributes, "value", "get", "post")`,
			want: func(td pprofile.Profiles) {
				putProfileAttribute(t, td, 0, "http.method", "post")
				putProfileAttribute(t, td, 1, "http.method", "post")
			},
		},
		{
			statement: `replace_all_patterns(attributes, "key", "http.url", "url")`,
			want: func(td pprofile.Profiles) {
				clearProfileAttributes(td, 0)
				putProfileAttribute(t, td, 0, "http.method", "get")
				putProfileAttribute(t, td, 0, "http.path", "/health")
				putProfileAttribute(t, td, 0, "url", "http://localhost/health")
				putProfileAttribute(t, td, 0, "flags", "A|B|C")
				putProfileAttribute(t, td, 0, "total.string", "123456789")
				clearProfileAttributes(td, 1)
				putProfileAttribute(t, td, 1, "http.method", "get")
				putProfileAttribute(t, td, 1, "http.path", "/health")
				putProfileAttribute(t, td, 1, "url", "http://localhost/health")
				putProfileAttribute(t, td, 1, "flags", "C|D")
				putProfileAttribute(t, td, 1, "total.string", "345678")
			},
		},
		{
			statement: `set(original_payload_format, "pass") where dropped_attributes_count == 1`,
			want: func(td pprofile.Profiles) {
				td.ResourceProfiles().At(0).ScopeProfiles().At(0).Profiles().At(0).SetOriginalPayloadFormat("pass")
				td.ResourceProfiles().At(0).ScopeProfiles().At(0).Profiles().At(1).SetOriginalPayloadFormat("pass")
			},
		},
		{
			statement: `set(original_payload_format, "pass") where profile_id == ProfileID(0x0102030405060708090a0b0c0d0e0f10)`,
			want: func(td pprofile.Profiles) {
				td.ResourceProfiles().At(0).ScopeProfiles().At(0).Profiles().At(0).SetOriginalPayloadFormat("pass")
				td.ResourceProfiles().At(0).ScopeProfiles().At(0).Profiles().At(1).SetOriginalPayloadFormat("pass")
			},
		},
		{
			statement: `set(original_payload_format, "pass") where IsMatch(original_payload_format, "operation[AC]")`,
			want: func(td pprofile.Profiles) {
				td.ResourceProfiles().At(0).ScopeProfiles().At(0).Profiles().At(0).SetOriginalPayloadFormat("pass")
			},
		},
		{
			statement: `delete_key(attributes, "http.url") where original_payload_format == "operationA"`,
			want: func(td pprofile.Profiles) {
				deleteProfileAttribute(td, 0, "http.url")
			},
		},
		{
			statement: `delete_matching_keys(attributes, "http.*t.*") where original_payload_format == "operationA"`,
			want: func(td pprofile.Profiles) {
				deleteProfileAttributeSequential(td, 0, "http.path")
				deleteProfileAttributeSequential(td, 0, "http.method")
			},
		},
		{
			statement: `set(original_payload_format, Concat([original_payload_format, original_payload_format], ": ")) where original_payload_format == Concat(["operation", "A"], "")`,
			want: func(td pprofile.Profiles) {
				td.ResourceProfiles().At(0).ScopeProfiles().At(0).Profiles().At(0).SetOriginalPayloadFormat("operationA: operationA")
			},
		},
		{
			statement: `set(attributes["test"], Split(attributes["flags"], "|"))`,
			want: func(td pprofile.Profiles) {
				putProfileAttribute(t, td, 0, "test", []any{"A", "B", "C"})
				putProfileAttribute(t, td, 1, "test", []any{"C", "D"})
			},
		},
		{
			statement: `set(attributes["test"], Split(attributes["flags"], "|")) where original_payload_format == "operationA"`,
			want: func(td pprofile.Profiles) {
				putProfileAttribute(t, td, 0, "test", []any{"A", "B", "C"})
			},
		},
		{
			statement: `set(original_payload_format, Split(resource.attributes["not_exist"], "|"))`,
			want:      func(pprofile.Profiles) {},
		},
		{
			statement: `set(original_payload_format, Substring(original_payload_format, 3, 3))`,
			want: func(td pprofile.Profiles) {
				td.ResourceProfiles().At(0).ScopeProfiles().At(0).Profiles().At(0).SetOriginalPayloadFormat("rat")
			},
		},
		{
			statement: `set(original_payload_format, Substring(original_payload_format, 3, 3)) where original_payload_format == "operationA"`,
			want: func(td pprofile.Profiles) {
				td.ResourceProfiles().At(0).ScopeProfiles().At(0).Profiles().At(0).SetOriginalPayloadFormat("rat")
			},
		},
		{
			statement: `set(original_payload_format, Substring(attributes["not_exist"], 3, 3))`,
			want:      func(pprofile.Profiles) {},
		},
		{
			statement: `set(attributes["test"], ["A", "B", "C"]) where original_payload_format == "operationA"`,
			want: func(td pprofile.Profiles) {
				putProfileAttribute(t, td, 0, "test", []any{"A", "B", "C"})
			},
		},
		{
			statement: `set(original_payload_format, ConvertCase(original_payload_format, "lower")) where original_payload_format == "operationA"`,
			want: func(td pprofile.Profiles) {
				td.ResourceProfiles().At(0).ScopeProfiles().At(0).Profiles().At(0).SetOriginalPayloadFormat("operationa")
			},
		},
		{
			statement: `set(original_payload_format, ConvertCase(original_payload_format, "upper")) where original_payload_format == "operationA"`,
			want: func(td pprofile.Profiles) {
				td.ResourceProfiles().At(0).ScopeProfiles().At(0).Profiles().At(0).SetOriginalPayloadFormat("OPERATIONA")
			},
		},
		{
			statement: `set(original_payload_format, ConvertCase(original_payload_format, "snake")) where original_payload_format == "operationA"`,
			want: func(td pprofile.Profiles) {
				td.ResourceProfiles().At(0).ScopeProfiles().At(0).Profiles().At(0).SetOriginalPayloadFormat("operation_a")
			},
		},
		{
			statement: `set(original_payload_format, ConvertCase(original_payload_format, "camel")) where original_payload_format == "operationA"`,
			want: func(td pprofile.Profiles) {
				td.ResourceProfiles().At(0).ScopeProfiles().At(0).Profiles().At(0).SetOriginalPayloadFormat("OperationA")
			},
		},
		{
			statement: `merge_maps(attributes, ParseJSON("{\"json_test\":\"pass\"}"), "insert") where original_payload_format == "operationA"`,
			want: func(td pprofile.Profiles) {
				putProfileAttribute(t, td, 0, "json_test", "pass")
			},
		},
		{
			statement: `limit(attributes, 0, []) where original_payload_format == "operationA"`,
			want: func(td pprofile.Profiles) {
				clearProfileAttributes(td, 0)
			},
		},
		{
			statement: `set(original_payload_format, String(Log(1))) where original_payload_format == "operationA"`,
			want: func(td pprofile.Profiles) {
				td.ResourceProfiles().At(0).ScopeProfiles().At(0).Profiles().At(0).SetOriginalPayloadFormat("0")
			},
		},
		{
			statement: `replace_match(original_payload_format, "*", "12345")`,
			want: func(td pprofile.Profiles) {
				td.ResourceProfiles().At(0).ScopeProfiles().At(0).Profiles().At(0).SetOriginalPayloadFormat("12345")
				td.ResourceProfiles().At(0).ScopeProfiles().At(0).Profiles().At(1).SetOriginalPayloadFormat("12345")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.statement, func(t *testing.T) {
			td := constructProfiles()
			processor, err := NewProcessor([]common.ContextStatements{{Context: "profile", Statements: []string{tt.statement}}}, ottl.IgnoreError, componenttest.NewNopTelemetrySettings(), DefaultProfileFunctions)
			require.NoError(t, err)

			_, err = processor.ProcessProfiles(t.Context(), td)
			assert.NoError(t, err)

			exTd := constructProfiles()
			tt.want(exTd)

			assert.Equal(t, exTd, td)
		})
	}
}

func Test_ProcessProfiles_InferredProfileContext(t *testing.T) {
	tests := []struct {
		statement string
		want      func(td pprofile.Profiles)
	}{
		{
			statement: `set(profile.attributes["test"], "pass") where profile.original_payload_format == "operationA"`,
			want: func(td pprofile.Profiles) {
				putProfileAttribute(t, td, 0, "test", "pass")
			},
		},
		{
			statement: `set(profile.original_payload_format, "pass") where profile.original_payload_format == "operationA"`,
			want: func(td pprofile.Profiles) {
				td.ResourceProfiles().At(0).ScopeProfiles().At(0).Profiles().At(0).SetOriginalPayloadFormat("pass")
			},
		},
		{
			statement: `set(profile.attributes["test"], "pass") where resource.attributes["host.name"] == "localhost"`,
			want: func(td pprofile.Profiles) {
				putProfileAttribute(t, td, 0, "test", "pass")
				putProfileAttribute(t, td, 1, "test", "pass")
			},
		},
		{
			statement: `set(profile.original_payload_format, "pass") where resource.attributes["host.name"] == "localhost"`,
			want: func(td pprofile.Profiles) {
				td.ResourceProfiles().At(0).ScopeProfiles().At(0).Profiles().At(0).SetOriginalPayloadFormat("pass")
				td.ResourceProfiles().At(0).ScopeProfiles().At(0).Profiles().At(1).SetOriginalPayloadFormat("pass")
			},
		},
		{
			statement: `keep_keys(profile.attributes, ["http.method"]) where profile.original_payload_format == "operationA"`,
			want: func(td pprofile.Profiles) {
				clearProfileAttributes(td, 0)
				putProfileAttribute(t, td, 0, "http.method", "get")
			},
		},
		{
			statement: `replace_pattern(profile.attributes["http.method"], "get", "post")`,
			want: func(td pprofile.Profiles) {
				putProfileAttribute(t, td, 0, "http.method", "post")
				putProfileAttribute(t, td, 1, "http.method", "post")
			},
		},
		{
			statement: `replace_all_patterns(profile.attributes, "value", "get", "post")`,
			want: func(td pprofile.Profiles) {
				putProfileAttribute(t, td, 0, "http.method", "post")
				putProfileAttribute(t, td, 1, "http.method", "post")
			},
		},
		{
			statement: `replace_all_patterns(profile.attributes, "key", "http.url", "url")`,
			want: func(td pprofile.Profiles) {
				clearProfileAttributes(td, 0)
				putProfileAttribute(t, td, 0, "http.method", "get")
				putProfileAttribute(t, td, 0, "http.path", "/health")
				putProfileAttribute(t, td, 0, "url", "http://localhost/health")
				putProfileAttribute(t, td, 0, "flags", "A|B|C")
				putProfileAttribute(t, td, 0, "total.string", "123456789")
				clearProfileAttributes(td, 1)
				putProfileAttribute(t, td, 1, "http.method", "get")
				putProfileAttribute(t, td, 1, "http.path", "/health")
				putProfileAttribute(t, td, 1, "url", "http://localhost/health")
				putProfileAttribute(t, td, 1, "flags", "C|D")
				putProfileAttribute(t, td, 1, "total.string", "345678")
			},
		},
		{
			statement: `set(profile.original_payload_format, "pass") where profile.dropped_attributes_count == 1`,
			want: func(td pprofile.Profiles) {
				td.ResourceProfiles().At(0).ScopeProfiles().At(0).Profiles().At(0).SetOriginalPayloadFormat("pass")
				td.ResourceProfiles().At(0).ScopeProfiles().At(0).Profiles().At(1).SetOriginalPayloadFormat("pass")
			},
		},
		{
			statement: `set(profile.original_payload_format, "pass") where IsMatch(profile.original_payload_format, "operation[AC]")`,
			want: func(td pprofile.Profiles) {
				td.ResourceProfiles().At(0).ScopeProfiles().At(0).Profiles().At(0).SetOriginalPayloadFormat("pass")
			},
		},
		{
			statement: `delete_key(profile.attributes, "http.url") where profile.original_payload_format == "operationA"`,
			want: func(td pprofile.Profiles) {
				deleteProfileAttribute(td, 0, "http.url")
			},
		},
		{
			statement: `delete_matching_keys(profile.attributes, "http.*t.*") where profile.original_payload_format == "operationA"`,
			want: func(td pprofile.Profiles) {
				deleteProfileAttributeSequential(td, 0, "http.path")
				deleteProfileAttributeSequential(td, 0, "http.method")
			},
		},
		{
			statement: `set(profile.original_payload_format, Concat([profile.original_payload_format, profile.original_payload_format], ": ")) where profile.original_payload_format == Concat(["operation", "A"], "")`,
			want: func(td pprofile.Profiles) {
				td.ResourceProfiles().At(0).ScopeProfiles().At(0).Profiles().At(0).SetOriginalPayloadFormat("operationA: operationA")
			},
		},
		{
			statement: `set(profile.attributes["test"], Split(profile.attributes["flags"], "|"))`,
			want: func(td pprofile.Profiles) {
				putProfileAttribute(t, td, 0, "test", []any{"A", "B", "C"})
				putProfileAttribute(t, td, 1, "test", []any{"C", "D"})
			},
		},
		{
			statement: `set(profile.attributes["test"], Split(profile.attributes["flags"], "|")) where profile.original_payload_format == "operationA"`,
			want: func(td pprofile.Profiles) {
				putProfileAttribute(t, td, 0, "test", []any{"A", "B", "C"})
			},
		},
		{
			statement: `set(profile.original_payload_format, Split(resource.attributes["not_exist"], "|"))`,
			want:      func(pprofile.Profiles) {},
		},
		{
			statement: `set(profile.original_payload_format, Substring(profile.original_payload_format, 3, 3))`,
			want: func(td pprofile.Profiles) {
				td.ResourceProfiles().At(0).ScopeProfiles().At(0).Profiles().At(0).SetOriginalPayloadFormat("rat")
			},
		},
		{
			statement: `set(profile.original_payload_format, Substring(profile.original_payload_format, 3, 3)) where profile.original_payload_format == "operationA"`,
			want: func(td pprofile.Profiles) {
				td.ResourceProfiles().At(0).ScopeProfiles().At(0).Profiles().At(0).SetOriginalPayloadFormat("rat")
			},
		},
		{
			statement: `set(profile.original_payload_format, Substring(resource.attributes["not_exist"], 3, 3))`,
			want:      func(pprofile.Profiles) {},
		},
		{
			statement: `set(profile.attribute_indices, [1, 2, 3]) where profile.original_payload_format == "operationA"`,
			want: func(td pprofile.Profiles) {
				td.ResourceProfiles().At(0).ScopeProfiles().At(0).Profiles().At(0).AttributeIndices().FromRaw([]int32{1, 2, 3})
			},
		},
		{
			statement: `set(profile.original_payload_format, ConvertCase(profile.original_payload_format, "lower")) where profile.original_payload_format == "operationA"`,
			want: func(td pprofile.Profiles) {
				td.ResourceProfiles().At(0).ScopeProfiles().At(0).Profiles().At(0).SetOriginalPayloadFormat("operationa")
			},
		},
		{
			statement: `set(profile.original_payload_format, ConvertCase(profile.original_payload_format, "upper")) where profile.original_payload_format == "operationA"`,
			want: func(td pprofile.Profiles) {
				td.ResourceProfiles().At(0).ScopeProfiles().At(0).Profiles().At(0).SetOriginalPayloadFormat("OPERATIONA")
			},
		},
		{
			statement: `set(profile.original_payload_format, ConvertCase(profile.original_payload_format, "snake")) where profile.original_payload_format == "operationA"`,
			want: func(td pprofile.Profiles) {
				td.ResourceProfiles().At(0).ScopeProfiles().At(0).Profiles().At(0).SetOriginalPayloadFormat("operation_a")
			},
		},
		{
			statement: `set(profile.original_payload_format, ConvertCase(profile.original_payload_format, "camel")) where profile.original_payload_format == "operationA"`,
			want: func(td pprofile.Profiles) {
				td.ResourceProfiles().At(0).ScopeProfiles().At(0).Profiles().At(0).SetOriginalPayloadFormat("OperationA")
			},
		},
		{
			statement: `merge_maps(profile.attributes, ParseJSON("{\"json_test\":\"pass\"}"), "insert") where profile.original_payload_format == "operationA"`,
			want: func(td pprofile.Profiles) {
				putProfileAttribute(t, td, 0, "json_test", "pass")
			},
		},
		{
			statement: `limit(profile.attributes, 0, []) where profile.original_payload_format == "operationA"`,
			want: func(td pprofile.Profiles) {
				clearProfileAttributes(td, 0)
			},
		},
		{
			statement: `set(profile.original_payload_format, String(Log(1))) where profile.original_payload_format == "operationA"`,
			want: func(td pprofile.Profiles) {
				td.ResourceProfiles().At(0).ScopeProfiles().At(0).Profiles().At(0).SetOriginalPayloadFormat("0")
			},
		},
		{
			statement: `replace_match(profile.original_payload_format, "*", "12345")`,
			want: func(td pprofile.Profiles) {
				td.ResourceProfiles().At(0).ScopeProfiles().At(0).Profiles().At(0).SetOriginalPayloadFormat("12345")
				td.ResourceProfiles().At(0).ScopeProfiles().At(0).Profiles().At(1).SetOriginalPayloadFormat("12345")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.statement, func(t *testing.T) {
			td := constructProfiles()
			processor, err := NewProcessor([]common.ContextStatements{{Context: "", Statements: []string{tt.statement}}}, ottl.IgnoreError, componenttest.NewNopTelemetrySettings(), DefaultProfileFunctions)
			require.NoError(t, err)

			_, err = processor.ProcessProfiles(t.Context(), td)
			assert.NoError(t, err)

			exTd := constructProfiles()
			tt.want(exTd)

			assert.Equal(t, exTd, td)
		})
	}
}

func Test_ProcessProfiles_MixContext(t *testing.T) {
	tests := []struct {
		name              string
		contextStatements []common.ContextStatements
		want              func(td pprofile.Profiles)
	}{
		{
			name: "set resource and then use",
			contextStatements: []common.ContextStatements{
				{
					Context: "resource",
					Statements: []string{
						`set(attributes["test"], "pass")`,
					},
				},
				{
					Context: "profile",
					Statements: []string{
						`set(original_payload_format, "pass") where resource.attributes["test"] == "pass"`,
					},
				},
			},
			want: func(td pprofile.Profiles) {
				td.ResourceProfiles().At(0).Resource().Attributes().PutStr("test", "pass")
				td.ResourceProfiles().At(0).ScopeProfiles().At(0).Profiles().At(0).SetOriginalPayloadFormat("pass")
				td.ResourceProfiles().At(0).ScopeProfiles().At(0).Profiles().At(1).SetOriginalPayloadFormat("pass")
			},
		},
		{
			name: "set scope and then use",
			contextStatements: []common.ContextStatements{
				{
					Context: "scope",
					Statements: []string{
						`set(attributes["test"], "pass")`,
					},
				},
				{
					Context: "profile",
					Statements: []string{
						`set(original_payload_format, "pass") where instrumentation_scope.attributes["test"] == "pass"`,
					},
				},
			},
			want: func(td pprofile.Profiles) {
				td.ResourceProfiles().At(0).ScopeProfiles().At(0).Scope().Attributes().PutStr("test", "pass")
				td.ResourceProfiles().At(0).ScopeProfiles().At(0).Profiles().At(0).SetOriginalPayloadFormat("pass")
				td.ResourceProfiles().At(0).ScopeProfiles().At(0).Profiles().At(1).SetOriginalPayloadFormat("pass")
			},
		},
		{
			name: "order matters",
			contextStatements: []common.ContextStatements{
				{
					Context: "profile",
					Statements: []string{
						`set(original_payload_format, "pass") where instrumentation_scope.attributes["test"] == "pass"`,
					},
				},
				{
					Context: "scope",
					Statements: []string{
						`set(attributes["test"], "pass")`,
					},
				},
			},
			want: func(td pprofile.Profiles) {
				td.ResourceProfiles().At(0).ScopeProfiles().At(0).Scope().Attributes().PutStr("test", "pass")
			},
		},
		{
			name: "reuse context",
			contextStatements: []common.ContextStatements{
				{
					Context: "scope",
					Statements: []string{
						`set(attributes["test"], "pass")`,
					},
				},
				{
					Context: "profile",
					Statements: []string{
						`set(original_payload_format, "pass") where instrumentation_scope.attributes["test"] == "pass"`,
					},
				},
				{
					Context: "scope",
					Statements: []string{
						`set(attributes["test"], "fail")`,
					},
				},
			},
			want: func(td pprofile.Profiles) {
				td.ResourceProfiles().At(0).ScopeProfiles().At(0).Scope().Attributes().PutStr("test", "fail")
				td.ResourceProfiles().At(0).ScopeProfiles().At(0).Profiles().At(0).SetOriginalPayloadFormat("pass")
				td.ResourceProfiles().At(0).ScopeProfiles().At(0).Profiles().At(1).SetOriginalPayloadFormat("pass")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			td := constructProfiles()
			processor, err := NewProcessor(tt.contextStatements, ottl.IgnoreError, componenttest.NewNopTelemetrySettings(), DefaultProfileFunctions)
			require.NoError(t, err)

			_, err = processor.ProcessProfiles(t.Context(), td)
			assert.NoError(t, err)

			exTd := constructProfiles()
			tt.want(exTd)

			assert.Equal(t, exTd, td)
		})
	}
}

func Test_ProcessProfiles_InferredMixContext(t *testing.T) {
	tests := []struct {
		name              string
		contextStatements []common.ContextStatements
		want              func(td pprofile.Profiles)
	}{
		{
			name: "set resource and then use",
			contextStatements: []common.ContextStatements{
				{
					Statements: []string{`set(resource.attributes["test"], "pass")`},
				},
				{
					Statements: []string{`set(profile.original_payload_format, "pass") where resource.attributes["test"] == "pass"`},
				},
			},
			want: func(td pprofile.Profiles) {
				td.ResourceProfiles().At(0).Resource().Attributes().PutStr("test", "pass")
				td.ResourceProfiles().At(0).ScopeProfiles().At(0).Profiles().At(0).SetOriginalPayloadFormat("pass")
				td.ResourceProfiles().At(0).ScopeProfiles().At(0).Profiles().At(1).SetOriginalPayloadFormat("pass")
			},
		},
		{
			name: "set scope and then use",
			contextStatements: []common.ContextStatements{
				{
					Statements: []string{`set(scope.attributes["test"], "pass")`},
				},
				{
					Statements: []string{`set(profile.original_payload_format, "pass") where instrumentation_scope.attributes["test"] == "pass"`},
				},
			},
			want: func(td pprofile.Profiles) {
				td.ResourceProfiles().At(0).ScopeProfiles().At(0).Scope().Attributes().PutStr("test", "pass")
				td.ResourceProfiles().At(0).ScopeProfiles().At(0).Profiles().At(0).SetOriginalPayloadFormat("pass")
				td.ResourceProfiles().At(0).ScopeProfiles().At(0).Profiles().At(1).SetOriginalPayloadFormat("pass")
			},
		},
		{
			name: "order matters",
			contextStatements: []common.ContextStatements{
				{
					Statements: []string{`set(profile.original_payload_format, "pass") where instrumentation_scope.attributes["test"] == "pass"`},
				},
				{
					Statements: []string{`set(scope.attributes["test"], "pass")`},
				},
			},
			want: func(td pprofile.Profiles) {
				td.ResourceProfiles().At(0).ScopeProfiles().At(0).Scope().Attributes().PutStr("test", "pass")
			},
		},
		{
			name: "reuse context",
			contextStatements: []common.ContextStatements{
				{
					Statements: []string{`set(scope.attributes["test"], "pass")`},
				},
				{
					Statements: []string{`set(profile.original_payload_format, "pass") where instrumentation_scope.attributes["test"] == "pass"`},
				},
				{
					Statements: []string{`set(scope.attributes["test"], "fail")`},
				},
			},
			want: func(td pprofile.Profiles) {
				td.ResourceProfiles().At(0).ScopeProfiles().At(0).Scope().Attributes().PutStr("test", "fail")
				td.ResourceProfiles().At(0).ScopeProfiles().At(0).Profiles().At(0).SetOriginalPayloadFormat("pass")
				td.ResourceProfiles().At(0).ScopeProfiles().At(0).Profiles().At(1).SetOriginalPayloadFormat("pass")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			td := constructProfiles()
			processor, err := NewProcessor(tt.contextStatements, ottl.IgnoreError, componenttest.NewNopTelemetrySettings(), DefaultProfileFunctions)
			require.NoError(t, err)

			_, err = processor.ProcessProfiles(t.Context(), td)
			assert.NoError(t, err)

			exTd := constructProfiles()
			tt.want(exTd)

			assert.Equal(t, exTd, td)
		})
	}
}

func Test_ProcessProfiles_ErrorMode(t *testing.T) {
	tests := []struct {
		statement string
		context   common.ContextID
	}{
		{
			context:   "resource",
			statement: `set(attributes["test"], ParseJSON(1))`,
		},
		{
			context:   "scope",
			statement: `set(attributes["test"], ParseJSON(1))`,
		},
		{
			context:   "profile",
			statement: `set(original_payload_format, ParseJSON(1))`,
		},
	}

	for _, tt := range tests {
		t.Run(string(tt.context), func(t *testing.T) {
			td := constructProfiles()
			processor, err := NewProcessor([]common.ContextStatements{{Context: tt.context, Statements: []string{tt.statement}}}, ottl.PropagateError, componenttest.NewNopTelemetrySettings(), DefaultProfileFunctions)
			require.NoError(t, err)

			_, err = processor.ProcessProfiles(t.Context(), td)
			assert.Error(t, err)
		})
	}
}

func Test_ProcessProfiles_StatementsErrorMode(t *testing.T) {
	tests := []struct {
		name          string
		errorMode     ottl.ErrorMode
		statements    []common.ContextStatements
		want          func(td pprofile.Profiles)
		wantErrorWith string
	}{
		{
			name:      "profile: statements group with error mode",
			errorMode: ottl.PropagateError,
			statements: []common.ContextStatements{
				{Statements: []string{`set(profile.original_payload_format, ParseJSON(1))`}, ErrorMode: ottl.IgnoreError},
				{Statements: []string{`set(profile.original_payload_format, "pass") where profile.dropped_attributes_count == 1`}},
			},
			want: func(td pprofile.Profiles) {
				td.ResourceProfiles().At(0).ScopeProfiles().At(0).Profiles().At(0).SetOriginalPayloadFormat("pass")
				td.ResourceProfiles().At(0).ScopeProfiles().At(0).Profiles().At(1).SetOriginalPayloadFormat("pass")
			},
		},
		{
			name:      "profile: statements group error mode does not affect default",
			errorMode: ottl.PropagateError,
			statements: []common.ContextStatements{
				{Statements: []string{`set(profile.original_payload_format, ParseJSON(1))`}, ErrorMode: ottl.IgnoreError},
				{Statements: []string{`set(profile.original_payload_format, ParseJSON(true))`}},
			},
			wantErrorWith: "expected string but got bool",
		},
		{
			name:      "resource: statements group with error mode",
			errorMode: ottl.PropagateError,
			statements: []common.ContextStatements{
				{Statements: []string{`set(resource.attributes["pass"], ParseJSON(1))`}, ErrorMode: ottl.IgnoreError},
				{Statements: []string{`set(resource.attributes["test"], "pass")`}},
			},
			want: func(td pprofile.Profiles) {
				td.ResourceProfiles().At(0).Resource().Attributes().PutStr("test", "pass")
			},
		},
		{
			name:      "resource: statements group error mode does not affect default",
			errorMode: ottl.PropagateError,
			statements: []common.ContextStatements{
				{Statements: []string{`set(resource.attributes["pass"], ParseJSON(1))`}, ErrorMode: ottl.IgnoreError},
				{Statements: []string{`set(resource.attributes["pass"], ParseJSON(true))`}},
			},
			wantErrorWith: "expected string but got bool",
		},
		{
			name:      "scope: statements group with error mode",
			errorMode: ottl.PropagateError,
			statements: []common.ContextStatements{
				{Statements: []string{`set(scope.attributes["pass"], ParseJSON(1))`}, ErrorMode: ottl.IgnoreError},
				{Statements: []string{`set(scope.attributes["test"], "pass")`}},
			},
			want: func(td pprofile.Profiles) {
				td.ResourceProfiles().At(0).ScopeProfiles().At(0).Scope().Attributes().PutStr("test", "pass")
			},
		},
		{
			name:      "scope: statements group error mode does not affect default",
			errorMode: ottl.PropagateError,
			statements: []common.ContextStatements{
				{Statements: []string{`set(scope.attributes["pass"], ParseJSON(1))`}, ErrorMode: ottl.IgnoreError},
				{Statements: []string{`set(scope.attributes["pass"], ParseJSON(true))`}},
			},
			wantErrorWith: "expected string but got bool",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			td := constructProfiles()
			processor, err := NewProcessor(tt.statements, tt.errorMode, componenttest.NewNopTelemetrySettings(), DefaultProfileFunctions)
			require.NoError(t, err)

			_, err = processor.ProcessProfiles(t.Context(), td)
			if tt.wantErrorWith != "" {
				if err == nil {
					t.Errorf("expected error containing '%s', got: <nil>", tt.wantErrorWith)
				}
				assert.Contains(t, err.Error(), tt.wantErrorWith)
				return
			}
			assert.NoError(t, err)

			exTd := constructProfiles()
			tt.want(exTd)
			assert.Equal(t, exTd, td)
		})
	}
}

func Test_ProcessProfiles_CacheAccess(t *testing.T) {
	tests := []struct {
		name       string
		statements []common.ContextStatements
		want       func(td pprofile.Profiles)
	}{
		{
			name: "resource:resource.cache",
			statements: []common.ContextStatements{
				{Statements: []string{
					`set(resource.cache["test"], "pass")`,
					`set(resource.attributes["test"], resource.cache["test"])`,
				}},
			},
			want: func(td pprofile.Profiles) {
				td.ResourceProfiles().At(0).Resource().Attributes().PutStr("test", "pass")
			},
		},
		{
			name: "resource:cache",
			statements: []common.ContextStatements{
				{
					Context: common.Resource,
					Statements: []string{
						`set(cache["test"], "pass")`,
						`set(attributes["test"], cache["test"])`,
					},
				},
			},
			want: func(td pprofile.Profiles) {
				td.ResourceProfiles().At(0).Resource().Attributes().PutStr("test", "pass")
			},
		},
		{
			name: "scope:scope.cache",
			statements: []common.ContextStatements{
				{Statements: []string{
					`set(scope.cache["test"], "pass")`,
					`set(scope.attributes["test"], scope.cache["test"])`,
				}},
			},
			want: func(td pprofile.Profiles) {
				td.ResourceProfiles().At(0).ScopeProfiles().At(0).Scope().Attributes().PutStr("test", "pass")
			},
		},
		{
			name: "scope:cache",
			statements: []common.ContextStatements{{
				Context: common.Scope,
				Statements: []string{
					`set(cache["test"], "pass")`,
					`set(attributes["test"], cache["test"])`,
				},
			}},
			want: func(td pprofile.Profiles) {
				td.ResourceProfiles().At(0).ScopeProfiles().At(0).Scope().Attributes().PutStr("test", "pass")
			},
		},
		{
			name: "profile:profile.cache",
			statements: []common.ContextStatements{
				{Statements: []string{
					`set(profile.cache["test"], "pass")`,
					`set(profile.original_payload_format, profile.cache["test"])`,
				}},
			},
			want: func(td pprofile.Profiles) {
				td.ResourceProfiles().At(0).ScopeProfiles().At(0).Profiles().At(0).SetOriginalPayloadFormat("pass")
				td.ResourceProfiles().At(0).ScopeProfiles().At(0).Profiles().At(1).SetOriginalPayloadFormat("pass")
			},
		},
		{
			name: "profile:cache",
			statements: []common.ContextStatements{{
				Context: common.Profile,
				Statements: []string{
					`set(cache["test"], "pass")`,
					`set(original_payload_format, cache["test"])`,
				},
			}},
			want: func(td pprofile.Profiles) {
				td.ResourceProfiles().At(0).ScopeProfiles().At(0).Profiles().At(0).SetOriginalPayloadFormat("pass")
				td.ResourceProfiles().At(0).ScopeProfiles().At(0).Profiles().At(1).SetOriginalPayloadFormat("pass")
			},
		},
		{
			name: "cache isolation",
			statements: []common.ContextStatements{
				{
					Statements: []string{`set(profile.cache["shared"], "fail")`},
				},
				{
					Statements: []string{
						`set(profile.cache["test"], "pass")`,
						`set(profile.original_payload_format, profile.cache["test"])`,
						`set(profile.original_payload_format, profile.cache["shared"])`,
					},
				},
				{
					Context: common.Profile,
					Statements: []string{
						`set(cache["test"], "pass")`,
						`set(original_payload_format, cache["test"])`,
						`set(original_payload_format, cache["shared"])`,
						`set(original_payload_format, profile.cache["shared"])`,
					},
				},
				{
					Statements: []string{`set(profile.original_payload_format, "pass") where profile.cache["shared"] == "fail"`},
				},
			},
			want: func(td pprofile.Profiles) {
				td.ResourceProfiles().At(0).ScopeProfiles().At(0).Profiles().At(0).SetOriginalPayloadFormat("pass")
				td.ResourceProfiles().At(0).ScopeProfiles().At(0).Profiles().At(1).SetOriginalPayloadFormat("pass")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			td := constructProfiles()
			processor, err := NewProcessor(tt.statements, ottl.IgnoreError, componenttest.NewNopTelemetrySettings(), DefaultProfileFunctions)
			require.NoError(t, err)

			_, err = processor.ProcessProfiles(t.Context(), td)
			assert.NoError(t, err)

			exTd := constructProfiles()
			tt.want(exTd)

			assert.Equal(t, exTd, td)
		})
	}
}

func Test_NewProcessor_ConditionsParse(t *testing.T) {
	type testCase struct {
		name              string
		statements        []common.ContextStatements
		profileStatements []common.ContextStatements
		wantErrorWith     string
	}

	contextsTests := map[string][]testCase{"profile": nil, "resource": nil, "scope": nil}
	for ctx := range contextsTests {
		contextsTests[ctx] = []testCase{
			{
				name: "inferred: condition with context",
				statements: []common.ContextStatements{
					{
						Statements: []string{fmt.Sprintf(`set(%s.cache["test"], "pass")`, ctx)},
						Conditions: []string{fmt.Sprintf(`%s.cache["test"] == ""`, ctx)},
					},
				},
			},
			{
				name: "inferred: condition without context",
				statements: []common.ContextStatements{
					{
						Statements: []string{fmt.Sprintf(`set(%s.cache["test"], "pass")`, ctx)},
						Conditions: []string{`cache["test"] == ""`},
					},
				},
				wantErrorWith: `missing context name for path "cache[test]"`,
			},
			{
				name: "context defined: condition without context",
				statements: []common.ContextStatements{
					{
						Context:    common.ContextID(ctx),
						Statements: []string{`set(cache["test"], "pass")`},
						Conditions: []string{`cache["test"] == ""`},
					},
				},
			},
			{
				name: "context defined: condition with context",
				statements: []common.ContextStatements{
					{
						Context:    common.ContextID(ctx),
						Statements: []string{`set(attributes["test"], "pass")`},
						Conditions: []string{fmt.Sprintf(`%s.cache["test"] == ""`, ctx)},
					},
				},
				// remove after merging https://github.com/open-telemetry/opentelemetry-collector-contrib/pull/39681
				profileStatements: []common.ContextStatements{
					{
						Context:    common.ContextID(ctx),
						Statements: []string{`set(original_payload_format, "pass")`},
						Conditions: []string{fmt.Sprintf(`%s.cache["test"] == ""`, ctx)},
					},
				},
			},
		}
	}

	for ctx, tests := range contextsTests {
		t.Run(ctx, func(t *testing.T) {
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					statements := tt.statements
					if tt.profileStatements != nil && ctx == "profile" {
						statements = tt.profileStatements
					}
					_, err := NewProcessor(statements, ottl.PropagateError, componenttest.NewNopTelemetrySettings(), DefaultProfileFunctions)
					if tt.wantErrorWith != "" {
						if err == nil {
							t.Errorf("expected error containing '%s', got: <nil>", tt.wantErrorWith)
						}
						assert.Contains(t, err.Error(), tt.wantErrorWith)
						return
					}
					require.NoError(t, err)
				})
			}
		})
	}
}

func Test_ProcessProfiles_InferredContextFromConditions(t *testing.T) {
	tests := []struct {
		name              string
		contextStatements []common.ContextStatements
		want              func(td pprofile.Profiles)
	}{
		{
			name: "inferring from statements",
			contextStatements: []common.ContextStatements{
				{
					Conditions: []string{`resource.attributes["test"] == nil`},
					Statements: []string{`set(profile.original_payload_format, Concat([profile.original_payload_format, "pass"], "-"))`},
				},
			},
			want: func(td pprofile.Profiles) {
				for _, v := range td.ResourceProfiles().All() {
					for _, m := range v.ScopeProfiles().All() {
						for _, mm := range m.Profiles().All() {
							mm.SetOriginalPayloadFormat(mm.OriginalPayloadFormat() + "-pass")
						}
					}
				}
			},
		},
		{
			name: "inferring from conditions",
			contextStatements: []common.ContextStatements{
				{
					Conditions: []string{`profile.original_payload_format != nil`},
					Statements: []string{`set(resource.attributes["test"], "pass")`},
				},
			},
			want: func(td pprofile.Profiles) {
				for _, v := range td.ResourceProfiles().All() {
					v.Resource().Attributes().PutStr("test", "pass")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			td := constructProfiles()
			processor, err := NewProcessor(tt.contextStatements, ottl.IgnoreError, componenttest.NewNopTelemetrySettings(), DefaultProfileFunctions)
			assert.NoError(t, err)

			_, err = processor.ProcessProfiles(t.Context(), td)
			assert.NoError(t, err)

			exTd := constructProfiles()
			tt.want(exTd)

			assert.Equal(t, exTd, td)
		})
	}
}

type TestFuncArguments[K any] struct{}

func createTestFunc[K any](ottl.FunctionContext, ottl.Arguments) (ottl.ExprFunc[K], error) {
	return func(context.Context, K) (any, error) {
		return nil, nil
	}, nil
}

func NewTestProfileFuncFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("TestProfileFunc", &TestFuncArguments[K]{}, createTestFunc[K])
}

func Test_NewProcessor_NonDefaultFunctions(t *testing.T) {
	type testCase struct {
		name             string
		statements       []common.ContextStatements
		wantErrorWith    string
		profileFunctions map[string]ottl.Factory[ottlprofile.TransformContext]
	}

	tests := []testCase{
		{
			name: "profile funcs : statement with added profile func",
			statements: []common.ContextStatements{
				{
					Context:    common.ContextID("profile"),
					Statements: []string{`set(cache["attr"], TestProfileFunc())`},
				},
			},
			profileFunctions: map[string]ottl.Factory[ottlprofile.TransformContext]{
				"set":             DefaultProfileFunctions["set"],
				"TestProfileFunc": NewTestProfileFuncFactory[ottlprofile.TransformContext](),
			},
		},
		{
			name: "profile funcs : statement with missing profile func",
			statements: []common.ContextStatements{
				{
					Context:    common.ContextID("profile"),
					Statements: []string{`set(cache["attr"], TestProfileFunc())`},
				},
			},
			wantErrorWith:    `undefined function "TestProfileFunc"`,
			profileFunctions: DefaultProfileFunctions,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewProcessor(tt.statements, ottl.PropagateError, componenttest.NewNopTelemetrySettings(), tt.profileFunctions)
			if tt.wantErrorWith != "" {
				if err == nil {
					t.Errorf("expected error containing '%s', got: <nil>", tt.wantErrorWith)
				}
				assert.Contains(t, err.Error(), tt.wantErrorWith)
				return
			}
			require.NoError(t, err)
		})
	}
}

func constructProfiles() pprofile.Profiles {
	return constructTestProfiles().Transform()
}

func constructTestProfiles() pprofiletest.Profiles {
	return pprofiletest.Profiles{
		ResourceProfiles: []pprofiletest.ResourceProfile{
			{
				Resource: pprofiletest.Resource{
					Attributes: []pprofiletest.Attribute{{Key: "host.name", Value: "localhost"}},
				},
				ScopeProfiles: []pprofiletest.ScopeProfile{
					{
						SchemaURL: "test_schema_url",
						Scope: pprofiletest.Scope{
							Name: "scope",
						},
						Profile: []pprofiletest.Profile{
							{
								Attributes: []pprofiletest.Attribute{
									{Key: "http.method", Value: "get"},
									{Key: "http.path", Value: "/health"},
									{Key: "http.url", Value: "http://localhost/health"},
									{Key: "flags", Value: "A|B|C"},
									{Key: "total.string", Value: "123456789"},
								},
								ProfileID:              profileID,
								TimeNanos:              1111,
								DurationNanos:          222,
								DroppedAttributesCount: 1,
								OriginalPayloadFormat:  "operationA",
							},
							{
								Attributes: []pprofiletest.Attribute{
									{Key: "http.method", Value: "get"},
									{Key: "http.path", Value: "/health"},
									{Key: "http.url", Value: "http://localhost/health"},
									{Key: "flags", Value: "C|D"},
									{Key: "total.string", Value: "345678"},
								},
								ProfileID:              profileID,
								TimeNanos:              3333,
								DurationNanos:          444,
								DroppedAttributesCount: 1,
								OriginalPayloadFormat:  "",
							},
						},
					},
				},
			},
		},
	}
}

func putProfileAttribute(t *testing.T, td pprofile.Profiles, profileIndex int, key string, value any) {
	t.Helper()
	dic := td.Dictionary()
	profile := td.ResourceProfiles().At(0).ScopeProfiles().At(0).Profiles().At(profileIndex)
	switch v := value.(type) {
	case string, []any:
		putAttribute(t, dic, profile, key, value)
	default:
		t.Fatalf("unsupported value type: %T", v)
	}
}

func putAttribute(t *testing.T, dic pprofile.ProfilesDictionary, profile pprofile.Profile, key string, value any) {
	t.Helper()

	kvu := pprofile.NewKeyValueAndUnit()
	keyIdx, err := pprofile.SetString(dic.StringTable(), key)
	require.NoError(t, err)

	kvu.SetKeyStrindex(keyIdx)
	require.NoError(t, kvu.Value().FromRaw(value))
	idx, err := pprofile.SetAttribute(dic.AttributeTable(), kvu)
	require.NoError(t, err)

	for k, i := range profile.AttributeIndices().All() {
		if i == idx {
			return
		}

		attr := dic.AttributeTable().At(int(i))
		if attr.KeyStrindex() == keyIdx {
			profile.AttributeIndices().SetAt(k, idx)
			return
		}
	}
	profile.AttributeIndices().Append(idx)
}

func clearProfileAttributes(pp pprofile.Profiles, idx int) {
	profile := pp.ResourceProfiles().At(0).ScopeProfiles().At(0).Profiles().At(idx)
	profile.AttributeIndices().FromRaw([]int32{})
}

// deleteProfileAttribute works as deleteKey() in func_delete_key.go.
func deleteProfileAttribute(pp pprofile.Profiles, idx int, key string) {
	dic := pp.Dictionary()
	profile := pp.ResourceProfiles().At(0).ScopeProfiles().At(0).Profiles().At(idx)
	indices := profile.AttributeIndices().AsRaw()
	for i := range indices {
		if dic.StringTable().At(int(dic.AttributeTable().At(int(indices[i])).KeyStrindex())) == key {
			indices[i] = indices[len(indices)-1]
			profile.AttributeIndices().FromRaw(indices[:len(indices)-1])
			return
		}
	}
}

// deleteProfileAttributeSequential works as deleteMatchingKeys() in func_delete_matching_keys.go.
func deleteProfileAttributeSequential(pp pprofile.Profiles, idx int, key string) { //nolint:unparam
	dic := pp.Dictionary()
	profile := pp.ResourceProfiles().At(0).ScopeProfiles().At(0).Profiles().At(idx)
	indices := profile.AttributeIndices().AsRaw()
	j := 0
	for i := range indices {
		if dic.StringTable().At(int(dic.AttributeTable().At(int(indices[i])).KeyStrindex())) == key {
			continue
		}
		indices[j] = indices[i]
		j++
	}
	if j == len(indices) {
		// No matching keys found, nothing to do.
		return
	}
	profile.AttributeIndices().FromRaw(indices[:j])
}
