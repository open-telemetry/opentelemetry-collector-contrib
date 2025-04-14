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
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pprofiletest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/common"
)

var (
	TestProfileTime      = time.Date(2020, 2, 11, 20, 26, 12, 321, time.UTC)
	TestProfileTimestamp = pcommon.NewTimestampFromTime(TestProfileTime)

	TestObservedTime      = time.Date(2020, 2, 11, 20, 26, 13, 789, time.UTC)
	TestObservedTimestamp = pcommon.NewTimestampFromTime(TestObservedTime)

	profileID = [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
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
			want: func(_ pprofile.Profiles) {
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
			processor, err := NewProcessor([]common.ContextStatements{{Context: "resource", Statements: []string{tt.statement}}}, ottl.IgnoreError, componenttest.NewNopTelemetrySettings())
			require.NoError(t, err)

			_, err = processor.ProcessProfiles(context.Background(), td)
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
			want: func(_ pprofile.Profiles) {
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
			processor, err := NewProcessor([]common.ContextStatements{{Context: "", Statements: []string{tt.statement}}}, ottl.IgnoreError, componenttest.NewNopTelemetrySettings())
			require.NoError(t, err)

			_, err = processor.ProcessProfiles(context.Background(), td)
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
			want: func(_ pprofile.Profiles) {
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
			processor, err := NewProcessor([]common.ContextStatements{{Context: "scope", Statements: []string{tt.statement}}}, ottl.IgnoreError, componenttest.NewNopTelemetrySettings())
			require.NoError(t, err)

			_, err = processor.ProcessProfiles(context.Background(), td)
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
			want: func(_ pprofile.Profiles) {
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
			processor, err := NewProcessor([]common.ContextStatements{{Context: "", Statements: []string{tt.statement}}}, ottl.IgnoreError, componenttest.NewNopTelemetrySettings())
			require.NoError(t, err)

			_, err = processor.ProcessProfiles(context.Background(), td)
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
		/*		{
				statement: `set(attributes["test"], "pass") where body == "operationA"`,
				want: func(td pprofile.Profiles) {
					putProfileAttribute(t, td, 0, "test", "pass")
				},
			},*/
		{
			statement: `set(attributes["test"], "pass") where resource.attributes["host.name"] == "localhost"`,
			want: func(td pprofile.Profiles) {
				putProfileAttribute(t, td, 0, "test", "pass")
				putProfileAttribute(t, td, 1, "test", "pass")
			},
		},
		{
			statement: `keep_keys(attributes, ["http.method"]) where body == "operationA"`,
			want: func(td pprofile.Profiles) {
				clearProfileAttribute(td, 0)
				putProfileAttribute(t, td, 1, "http.method", "get")
			},
		},
		{
			statement: `set(original_payload_format, "json") where attributes["http.path"] == "/health"`,
			want: func(td pprofile.Profiles) {
				td.ResourceProfiles().At(0).ScopeProfiles().At(0).Profiles().At(0).SetOriginalPayloadFormat("json")
				td.ResourceProfiles().At(0).ScopeProfiles().At(0).Profiles().At(1).SetOriginalPayloadFormat("json")
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
				clearProfileAttribute(td, 0)
				putProfileAttribute(t, td, 0, "http.method", "get")
				putProfileAttribute(t, td, 0, "http.path", "/health")
				putProfileAttribute(t, td, 0, "url", "http://localhost/health")
				putProfileAttribute(t, td, 0, "flags", "A|B|C")
				putProfileAttribute(t, td, 0, "total.string", "123456789")
				clearProfileAttribute(td, 1)
				putProfileAttribute(t, td, 1, "http.method", "get")
				putProfileAttribute(t, td, 1, "http.path", "/health")
				putProfileAttribute(t, td, 1, "url", "http://localhost/health")
				putProfileAttribute(t, td, 1, "flags", "C|D")
				putProfileAttribute(t, td, 1, "total.string", "345678")
			},
		},
		{
			statement: `set(attributes["test"], "pass") where dropped_attributes_count == 1`,
			want: func(td pprofile.Profiles) {
				putProfileAttribute(t, td, 0, "test", "pass")
			},
		},
		{
			statement: `set(attributes["test"], "pass") where flags == 1`,
			want: func(td pprofile.Profiles) {
				putProfileAttribute(t, td, 0, "test", "pass")
			},
		},
		{
			statement: `set(attributes["test"], "pass") where severity_number == SEVERITY_NUMBER_TRACE`,
			want: func(td pprofile.Profiles) {
				putProfileAttribute(t, td, 0, "test", "pass")
			},
		},
		/*		{
				statement: `set(severity_number, SEVERITY_NUMBER_TRACE2) where severity_number == 1`,
				want: func(td pprofile.Profiles) {
					td.ResourceProfiles().At(0).ScopeProfiles().At(0).Profiles().At(0).SetSeverityNumber(2)
				},
			},*/
		{
			statement: `set(attributes["test"], "pass") where trace_id == TraceID(0x0102030405060708090a0b0c0d0e0f10)`,
			want: func(td pprofile.Profiles) {
				putProfileAttribute(t, td, 0, "test", "pass")
			},
		},
		{
			statement: `set(attributes["test"], "pass") where span_id == SpanID(0x0102030405060708)`,
			want: func(td pprofile.Profiles) {
				putProfileAttribute(t, td, 0, "test", "pass")
			},
		},
		{
			statement: `set(attributes["test"], "pass") where IsMatch(body, "operation[AC]")`,
			want: func(td pprofile.Profiles) {
				putProfileAttribute(t, td, 0, "test", "pass")
			},
		},
		{
			statement: `delete_key(attributes, "http.url") where body == "operationA"`,
			want: func(td pprofile.Profiles) {
				clearProfileAttribute(td, 0)
				putProfileAttribute(t, td, 0, "http.method", "get")
				putProfileAttribute(t, td, 0, "http.path", "/health")
				putProfileAttribute(t, td, 0, "total.string", "123456789")
				putProfileAttribute(t, td, 0, "flags", "A|B|C")
			},
		},
		{
			statement: `delete_matching_keys(attributes, "http.*t.*") where body == "operationA"`,
			want: func(td pprofile.Profiles) {
				clearProfileAttribute(td, 0)
				putProfileAttribute(t, td, 0, "http.url", "http://localhost/health")
				putProfileAttribute(t, td, 0, "flags", "A|B|C")
				putProfileAttribute(t, td, 0, "total.string", "123456789")
			},
		},
		{
			statement: `set(attributes["test"], Concat([attributes["http.method"], attributes["http.url"]], ": ")) where body == Concat(["operation", "A"], "")`,
			want: func(td pprofile.Profiles) {
				putProfileAttribute(t, td, 0, "test", "get: http://localhost/health")
			},
		},
		{
			statement: `set(attributes["test"], Split(attributes["flags"], "|"))`,
			want: func(td pprofile.Profiles) {
				putProfileAttribute(t, td, 0, "test", []string{"A", "B", "C"})
				putProfileAttribute(t, td, 1, "test", []string{"C", "D"})
			},
		},
		{
			statement: `set(attributes["test"], Split(attributes["flags"], "|")) where body == "operationA"`,
			want: func(td pprofile.Profiles) {
				putProfileAttribute(t, td, 0, "test", []string{"A", "B", "C"})
			},
		},
		{
			statement: `set(attributes["test"], Split(attributes["not_exist"], "|"))`,
			want:      func(_ pprofile.Profiles) {},
		},
		{
			statement: `set(attributes["test"], Substring(attributes["total.string"], 3, 3))`,
			want: func(td pprofile.Profiles) {
				putProfileAttribute(t, td, 0, "test", "456")
				putProfileAttribute(t, td, 1, "test", "678")
			},
		},
		{
			statement: `set(attributes["test"], Substring(attributes["total.string"], 3, 3)) where body == "operationA"`,
			want: func(td pprofile.Profiles) {
				putProfileAttribute(t, td, 0, "test", "456")
			},
		},
		{
			statement: `set(attributes["test"], Substring(attributes["not_exist"], 3, 3))`,
			want:      func(_ pprofile.Profiles) {},
		},
		{
			statement: `set(attributes["test"], ["A", "B", "C"]) where body == "operationA"`,
			want: func(td pprofile.Profiles) {
				putProfileAttribute(t, td, 0, "test", []string{"A", "B", "C"})
			},
		},
		{
			statement: `set(attributes["test"], ConvertCase(body, "lower")) where body == "operationA"`,
			want: func(td pprofile.Profiles) {
				putProfileAttribute(t, td, 0, "test", "operationa")
			},
		},
		{
			statement: `set(attributes["test"], ConvertCase(body, "upper")) where body == "operationA"`,
			want: func(td pprofile.Profiles) {
				putProfileAttribute(t, td, 0, "test", "OPERATIONA")
			},
		},
		{
			statement: `set(attributes["test"], ConvertCase(body, "snake")) where body == "operationA"`,
			want: func(td pprofile.Profiles) {
				putProfileAttribute(t, td, 0, "test", "operation_a")
			},
		},
		{
			statement: `set(attributes["test"], ConvertCase(body, "camel")) where body == "operationA"`,
			want: func(td pprofile.Profiles) {
				putProfileAttribute(t, td, 0, "test", "OperationA")
			},
		},
		{
			statement: `merge_maps(attributes, ParseJSON("{\"json_test\":\"pass\"}"), "insert") where body == "operationA"`,
			want: func(td pprofile.Profiles) {
				putProfileAttribute(t, td, 0, "json_test", "pass")
			},
		},
		/*		{
				statement: `limit(attributes, 0, []) where body == "operationA"`,
				want: func(td pprofile.Profiles) {
					td.ResourceProfiles().At(0).ScopeProfiles().At(0).Profiles().At(0).Attributes().RemoveIf(func(_ string, _ pcommon.Value) bool { return true })
				},
			}, */
		{
			statement: `set(attributes["test"], Profile(1)) where body == "operationA"`,
			want: func(td pprofile.Profiles) {
				putProfileAttribute(t, td, 0, "test", 0.0)
			},
		},
		{
			statement: `replace_match(body["metadata"]["uid"], "*", "12345")`,
			want:      func(_ pprofile.Profiles) {},
		},
	}

	for _, tt := range tests {
		t.Run(tt.statement, func(t *testing.T) {
			td := constructProfiles()
			processor, err := NewProcessor([]common.ContextStatements{{Context: "profile", Statements: []string{tt.statement}}}, ottl.IgnoreError, componenttest.NewNopTelemetrySettings())
			require.NoError(t, err)

			_, err = processor.ProcessProfiles(context.Background(), td)
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
			statement: `set(profile.attributes["test"], "pass") where profile.body == "operationA"`,
			want: func(td pprofile.Profiles) {
				putProfileAttribute(t, td, 0, "test", "pass")
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
			statement: `keep_keys(profile.attributes, ["http.method"]) where profile.body == "operationA"`,
			want: func(td pprofile.Profiles) {
				clearProfileAttribute(td, 0)
				putProfileAttribute(t, td, 0, "http.method", "get")
			},
		},
		{
			statement: `set(profile.original_payload_format, "json") where profile.attributes["http.path"] == "/health"`,
			want: func(td pprofile.Profiles) {
				td.ResourceProfiles().At(0).ScopeProfiles().At(0).Profiles().At(0).SetOriginalPayloadFormat("json")
				td.ResourceProfiles().At(0).ScopeProfiles().At(0).Profiles().At(1).SetOriginalPayloadFormat("json")
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
				clearProfileAttribute(td, 0)
				putProfileAttribute(t, td, 0, "http.method", "get")
				putProfileAttribute(t, td, 0, "http.path", "/health")
				putProfileAttribute(t, td, 0, "url", "http://localhost/health")
				putProfileAttribute(t, td, 0, "flags", "A|B|C")
				putProfileAttribute(t, td, 0, "total.string", "123456789")
				clearProfileAttribute(td, 1)
				putProfileAttribute(t, td, 1, "http.method", "get")
				putProfileAttribute(t, td, 1, "http.path", "/health")
				putProfileAttribute(t, td, 1, "url", "http://localhost/health")
				putProfileAttribute(t, td, 1, "flags", "C|D")
				putProfileAttribute(t, td, 1, "total.string", "345678")
			},
		},
		{
			statement: `set(profile.attributes["test"], "pass") where profile.dropped_attributes_count == 1`,
			want: func(td pprofile.Profiles) {
				putProfileAttribute(t, td, 0, "test", "pass")
			},
		},
		{
			statement: `set(profile.attributes["test"], "pass") where profile.flags == 1`,
			want: func(td pprofile.Profiles) {
				putProfileAttribute(t, td, 0, "test", "pass")
			},
		},
		{
			statement: `set(profile.attributes["test"], "pass") where profile.severity_number == SEVERITY_NUMBER_TRACE`,
			want: func(td pprofile.Profiles) {
				putProfileAttribute(t, td, 0, "test", "pass")
			},
		},
		/*		{
				statement: `set(profile.severity_number, SEVERITY_NUMBER_TRACE2) where profile.severity_number == 1`,
				want: func(td pprofile.Profiles) {
					td.ResourceProfiles().At(0).ScopeProfiles().At(0).Profiles().At(0).SetSeverityNumber(2)
				},
			},*/
		{
			statement: `set(profile.attributes["test"], "pass") where profile.trace_id == TraceID(0x0102030405060708090a0b0c0d0e0f10)`,
			want: func(td pprofile.Profiles) {
				putProfileAttribute(t, td, 0, "test", "pass")
			},
		},
		{
			statement: `set(profile.attributes["test"], "pass") where profile.span_id == SpanID(0x0102030405060708)`,
			want: func(td pprofile.Profiles) {
				putProfileAttribute(t, td, 0, "test", "pass")
			},
		},
		{
			statement: `set(profile.attributes["test"], "pass") where IsMatch(profile.body, "operation[AC]")`,
			want: func(td pprofile.Profiles) {
				putProfileAttribute(t, td, 0, "test", "pass")
			},
		},
		{
			statement: `delete_key(profile.attributes, "http.url") where profile.body == "operationA"`,
			want: func(td pprofile.Profiles) {
				clearProfileAttribute(td, 0)
				putProfileAttribute(t, td, 0, "http.method", "get")
				putProfileAttribute(t, td, 0, "http.path", "/health")
				putProfileAttribute(t, td, 0, "total.string", "123456789")
				putProfileAttribute(t, td, 0, "flags", "A|B|C")
			},
		},
		{
			statement: `delete_matching_keys(profile.attributes, "http.*t.*") where profile.body == "operationA"`,
			want: func(td pprofile.Profiles) {
				clearProfileAttribute(td, 0)
				putProfileAttribute(t, td, 0, "http.url", "http://localhost/health")
				putProfileAttribute(t, td, 0, "flags", "A|B|C")
				putProfileAttribute(t, td, 0, "total.string", "123456789")
			},
		},
		{
			statement: `set(profile.attributes["test"], Concat([profile.attributes["http.method"], profile.attributes["http.url"]], ": ")) where profile.body == Concat(["operation", "A"], "")`,
			want: func(td pprofile.Profiles) {
				putProfileAttribute(t, td, 0, "test", "get: http://localhost/health")
			},
		},
		{
			statement: `set(profile.attributes["test"], Split(profile.attributes["flags"], "|"))`,
			want: func(td pprofile.Profiles) {
				putProfileAttribute(t, td, 0, "test", []string{"A", "B", "C"})
				putProfileAttribute(t, td, 1, "test", []string{"C", "D"})
			},
		},
		{
			statement: `set(profile.attributes["test"], Split(profile.attributes["flags"], "|")) where profile.body == "operationA"`,
			want: func(td pprofile.Profiles) {
				putProfileAttribute(t, td, 0, "test", []string{"A", "B", "C"})
			},
		},
		{
			statement: `set(profile.attributes["test"], Split(profile.attributes["not_exist"], "|"))`,
			want:      func(_ pprofile.Profiles) {},
		},
		{
			statement: `set(profile.attributes["test"], Substring(profile.attributes["total.string"], 3, 3))`,
			want: func(td pprofile.Profiles) {
				putProfileAttribute(t, td, 0, "test", "456")
				putProfileAttribute(t, td, 1, "test", "678")
			},
		},
		{
			statement: `set(profile.attributes["test"], Substring(profile.attributes["total.string"], 3, 3)) where profile.body == "operationA"`,
			want: func(td pprofile.Profiles) {
				putProfileAttribute(t, td, 0, "test", "456")
			},
		},
		{
			statement: `set(profile.attributes["test"], Substring(profile.attributes["not_exist"], 3, 3))`,
			want:      func(_ pprofile.Profiles) {},
		},
		{
			statement: `set(profile.attributes["test"], ["A", "B", "C"]) where profile.body == "operationA"`,
			want: func(td pprofile.Profiles) {
				putProfileAttribute(t, td, 0, "test", []string{"A", "B", "C"})
			},
		},
		{
			statement: `set(profile.attributes["test"], ConvertCase(profile.body, "lower")) where profile.body == "operationA"`,
			want: func(td pprofile.Profiles) {
				putProfileAttribute(t, td, 0, "test", "operationa")
			},
		},
		{
			statement: `set(profile.attributes["test"], ConvertCase(profile.body, "upper")) where profile.body == "operationA"`,
			want: func(td pprofile.Profiles) {
				putProfileAttribute(t, td, 0, "test", "OPERATIONA")
			},
		},
		{
			statement: `set(profile.attributes["test"], ConvertCase(profile.body, "snake")) where profile.body == "operationA"`,
			want: func(td pprofile.Profiles) {
				putProfileAttribute(t, td, 0, "test", "operation_a")
			},
		},
		{
			statement: `set(profile.attributes["test"], ConvertCase(profile.body, "camel")) where profile.body == "operationA"`,
			want: func(td pprofile.Profiles) {
				putProfileAttribute(t, td, 0, "test", "OperationA")
			},
		},
		{
			statement: `merge_maps(profile.attributes, ParseJSON("{\"json_test\":\"pass\"}"), "insert") where profile.body == "operationA"`,
			want: func(td pprofile.Profiles) {
				putProfileAttribute(t, td, 0, "json_test", "pass")
			},
		},
		/*		{
				statement: `limit(profile.attributes, 0, []) where profile.body == "operationA"`,
				want: func(td pprofile.Profiles) {
					td.ResourceProfiles().At(0).ScopeProfiles().At(0).Profiles().At(0).Attributes().RemoveIf(func(_ string, _ pcommon.Value) bool { return true })
				},
			}, */
		{
			statement: `set(profile.attributes["test"], Profile(1)) where profile.body == "operationA"`,
			want: func(td pprofile.Profiles) {
				putProfileAttribute(t, td, 0, "test", float64(0.0))
			},
		},
		{
			statement: `replace_match(profile.body["metadata"]["uid"], "*", "12345")`,
			want:      func(_ pprofile.Profiles) {},
		},
	}

	for _, tt := range tests {
		t.Run(tt.statement, func(t *testing.T) {
			td := constructProfiles()
			processor, err := NewProcessor([]common.ContextStatements{{Context: "", Statements: []string{tt.statement}}}, ottl.IgnoreError, componenttest.NewNopTelemetrySettings())
			require.NoError(t, err)

			_, err = processor.ProcessProfiles(context.Background(), td)
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
						`set(attributes["test"], "pass") where resource.attributes["test"] == "pass"`,
					},
				},
			},
			want: func(td pprofile.Profiles) {
				td.ResourceProfiles().At(0).Resource().Attributes().PutStr("test", "pass")
				putProfileAttribute(t, td, 0, "test", "pass")
				putProfileAttribute(t, td, 1, "test", "pass")
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
						`set(attributes["test"], "pass") where instrumentation_scope.attributes["test"] == "pass"`,
					},
				},
			},
			want: func(td pprofile.Profiles) {
				td.ResourceProfiles().At(0).ScopeProfiles().At(0).Scope().Attributes().PutStr("test", "pass")
				putProfileAttribute(t, td, 0, "test", "pass")
				putProfileAttribute(t, td, 1, "test", "pass")
			},
		},
		{
			name: "order matters",
			contextStatements: []common.ContextStatements{
				{
					Context: "profile",
					Statements: []string{
						`set(attributes["test"], "pass") where instrumentation_scope.attributes["test"] == "pass"`,
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
						`set(attributes["test"], "pass") where instrumentation_scope.attributes["test"] == "pass"`,
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
				putProfileAttribute(t, td, 0, "test", "pass")
				putProfileAttribute(t, td, 1, "test", "pass")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			td := constructProfiles()
			processor, err := NewProcessor(tt.contextStatements, ottl.IgnoreError, componenttest.NewNopTelemetrySettings())
			require.NoError(t, err)

			_, err = processor.ProcessProfiles(context.Background(), td)
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
					Statements: []string{`set(profile.attributes["test"], "pass") where resource.attributes["test"] == "pass"`},
				},
			},
			want: func(td pprofile.Profiles) {
				td.ResourceProfiles().At(0).Resource().Attributes().PutStr("test", "pass")
				putProfileAttribute(t, td, 0, "test", "pass")
				putProfileAttribute(t, td, 1, "test", "pass")
			},
		},
		{
			name: "set scope and then use",
			contextStatements: []common.ContextStatements{
				{
					Statements: []string{`set(scope.attributes["test"], "pass")`},
				},
				{
					Statements: []string{`set(profile.attributes["test"], "pass") where instrumentation_scope.attributes["test"] == "pass"`},
				},
			},
			want: func(td pprofile.Profiles) {
				td.ResourceProfiles().At(0).ScopeProfiles().At(0).Scope().Attributes().PutStr("test", "pass")
				putProfileAttribute(t, td, 0, "test", "pass")
				putProfileAttribute(t, td, 1, "test", "pass")
			},
		},
		{
			name: "order matters",
			contextStatements: []common.ContextStatements{
				{
					Statements: []string{`set(profile.attributes["test"], "pass") where instrumentation_scope.attributes["test"] == "pass"`},
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
					Statements: []string{`set(profile.attributes["test"], "pass") where instrumentation_scope.attributes["test"] == "pass"`},
				},
				{
					Statements: []string{`set(scope.attributes["test"], "fail")`},
				},
			},
			want: func(td pprofile.Profiles) {
				td.ResourceProfiles().At(0).ScopeProfiles().At(0).Scope().Attributes().PutStr("test", "fail")
				putProfileAttribute(t, td, 0, "test", "pass")
				putProfileAttribute(t, td, 1, "test", "pass")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			td := constructProfiles()
			processor, err := NewProcessor(tt.contextStatements, ottl.IgnoreError, componenttest.NewNopTelemetrySettings())
			require.NoError(t, err)

			_, err = processor.ProcessProfiles(context.Background(), td)
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
			context: "resource",
		},
		{
			context: "scope",
		},
		{
			context: "profile",
		},
	}

	for _, tt := range tests {
		t.Run(string(tt.context), func(t *testing.T) {
			td := constructProfiles()
			processor, err := NewProcessor([]common.ContextStatements{{Context: tt.context, Statements: []string{`set(attributes["test"], ParseJSON(1))`}}}, ottl.PropagateError, componenttest.NewNopTelemetrySettings())
			require.NoError(t, err)

			_, err = processor.ProcessProfiles(context.Background(), td)
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
				{Statements: []string{`set(profile.attributes["pass"], ParseJSON(1))`}, ErrorMode: ottl.IgnoreError},
				{Statements: []string{`set(profile.attributes["test"], "pass") where profile.body == "operationA"`}},
			},
			want: func(td pprofile.Profiles) {
				putProfileAttribute(t, td, 0, "test", "pass")
			},
		},
		{
			name:      "profile: statements group error mode does not affect default",
			errorMode: ottl.PropagateError,
			statements: []common.ContextStatements{
				{Statements: []string{`set(profile.attributes["pass"], ParseJSON(1))`}, ErrorMode: ottl.IgnoreError},
				{Statements: []string{`set(profile.attributes["pass"], ParseJSON(true))`}},
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
			processor, err := NewProcessor(tt.statements, tt.errorMode, componenttest.NewNopTelemetrySettings())
			require.NoError(t, err)

			_, err = processor.ProcessProfiles(context.Background(), td)
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
					`set(profile.attributes["test"], profile.cache["test"])`,
				}},
			},
			want: func(td pprofile.Profiles) {
				putProfileAttribute(t, td, 0, "test", "pass")
				putProfileAttribute(t, td, 1, "test", "pass")
			},
		},
		{
			name: "profile:cache",
			statements: []common.ContextStatements{{
				Context: common.Profile,
				Statements: []string{
					`set(cache["test"], "pass")`,
					`set(attributes["test"], cache["test"])`,
				},
			}},
			want: func(td pprofile.Profiles) {
				putProfileAttribute(t, td, 0, "test", "pass")
				putProfileAttribute(t, td, 1, "test", "pass")
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
						`set(profile.attributes["test"], profile.cache["test"])`,
						`set(profile.attributes["test"], profile.cache["shared"])`,
					},
				},
				{
					Context: common.Profile,
					Statements: []string{
						`set(cache["test"], "pass")`,
						`set(attributes["test"], cache["test"])`,
						`set(attributes["test"], cache["shared"])`,
						`set(attributes["test"], profile.cache["shared"])`,
					},
				},
				{
					Statements: []string{`set(profile.attributes["test"], "pass") where profile.cache["shared"] == "fail"`},
				},
			},
			want: func(td pprofile.Profiles) {
				putProfileAttribute(t, td, 0, "test", "pass")
				putProfileAttribute(t, td, 1, "test", "pass")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			td := constructProfiles()
			processor, err := NewProcessor(tt.statements, ottl.IgnoreError, componenttest.NewNopTelemetrySettings())
			require.NoError(t, err)

			_, err = processor.ProcessProfiles(context.Background(), td)
			assert.NoError(t, err)

			exTd := constructProfiles()
			tt.want(exTd)

			assert.Equal(t, exTd, td)
		})
	}
}

func Test_NewProcessor_ConditionsParse(t *testing.T) {
	type testCase struct {
		name          string
		statements    []common.ContextStatements
		wantErrorWith string
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
			},
		}
	}

	for ctx, tests := range contextsTests {
		t.Run(ctx, func(t *testing.T) {
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					_, err := NewProcessor(tt.statements, ottl.PropagateError, componenttest.NewNopTelemetrySettings())
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

func constructProfiles() pprofile.Profiles {
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
									{Key: "htp.method", Value: "get"},
									{Key: "http.path", Value: "/health"},
									{Key: "http.url", Value: "http://localhost/health"},
									{Key: "flags", Value: "A|B|C"},
									{Key: "total.string", Value: "123456789"},
								},
								ProfileID:              profileID,
								TimeNanos:              1111,
								DurationNanos:          222,
								DroppedAttributesCount: 1,
							},
							{
								Attributes: []pprofiletest.Attribute{
									{Key: "htp.method", Value: "get"},
									{Key: "http.path", Value: "/health"},
									{Key: "http.url", Value: "http://localhost/health"},
									{Key: "flags", Value: "C|D"},
									{Key: "total.string", Value: "345678"},
								},
								ProfileID:              profileID,
								TimeNanos:              1111,
								DurationNanos:          222,
								DroppedAttributesCount: 1,
							},
						},
					},
				},
			},
		},
	}.Transform()
}

func putProfileAttribute(t *testing.T, pp pprofile.Profiles, idx int, key string, value any) {
	pv := pcommon.NewValueEmpty()
	assert.NoError(t, pv.FromRaw(value))
	profile := pp.ResourceProfiles().At(0).ScopeProfiles().At(0).Profiles().At(idx)
	assert.NoError(t, pprofile.AddAttribute(profile.AttributeTable(), profile, key, pv))
}

func clearProfileAttribute(pp pprofile.Profiles, idx int) {
	profile := pp.ResourceProfiles().At(0).ScopeProfiles().At(0).Profiles().At(idx)
	profile.AttributeIndices().FromRaw([]int32{})
}
