// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package googlecloudlogentryencodingextension

import (
	"bytes"
	"fmt"
	"os"
	"testing"

	gojson "github.com/goccy/go-json"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/googlecloudlogentryencodingextension/internal/auditlog"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/googlecloudlogentryencodingextension/internal/constants"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/googlecloudlogentryencodingextension/internal/passthroughnlb"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/googlecloudlogentryencodingextension/internal/proxynlb"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/googlecloudlogentryencodingextension/internal/vpcflowlog"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/plogtest"
)

func newTestExtension(t *testing.T, cfg Config) *ext {
	extension := newExtension(&cfg)
	err := extension.Start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)
	t.Cleanup(func() {
		err = extension.Shutdown(t.Context())
		require.NoError(t, err)
	})
	return extension
}

func TestHandleLogLine(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		logLine     []byte
		expectedErr string
	}{
		{
			name:        "invalid log entry",
			logLine:     []byte("invalid"),
			expectedErr: `failed to unmarshal log entry`,
		},
		{
			name:        "invalid log entry fields",
			logLine:     []byte(`{"logName": "invalid"}`),
			expectedErr: `failed to handle log entry`,
		},
		{
			name: "valid",
			logLine: []byte(`{
	"logName": "projects/open-telemetry/logs/log-test", 
	"timestamp": "2024-05-05T10:31:19.45570687Z"
}`),
		},
	}

	extension := newTestExtension(t, Config{})
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			logs := plog.NewLogs()
			err := extension.handleLogLine(logs, tt.logLine)
			if tt.expectedErr != "" {
				require.ErrorContains(t, err, tt.expectedErr)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestUnmarshalLogs(t *testing.T) {
	t.Parallel()

	// this test will test all common log fields at once
	data, err := os.ReadFile("testdata/log_entry.json")
	require.NoError(t, err)

	compacted := bytes.NewBuffer([]byte{})
	err = gojson.Compact(compacted, data)
	require.NoError(t, err)

	tests := []struct {
		name  string
		nLogs int
	}{
		{
			name:  "1 log",
			nLogs: 1,
		},
		{
			name:  "4 logs",
			nLogs: 4,
		},
	}

	extension := newTestExtension(t, Config{})

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create payload with as many logs as defined in nLogs.
			// Each log takes up one line. A new line means a new log.
			buff := bytes.NewBuffer([]byte{})
			for i := 0; i < tt.nLogs; i++ {
				buff.Write(compacted.Bytes())
				buff.Write([]byte{'\n'})
			}

			logs, err := extension.UnmarshalLogs(buff.Bytes())
			require.NoError(t, err)

			expected, err := golden.ReadLogs("testdata/log_entry_expected.yaml")
			require.NoError(t, err)
			require.Equal(t, 1, expected.ResourceLogs().Len())
			require.Equal(t, 1, expected.ResourceLogs().At(0).ScopeLogs().Len())

			// expected logs is only for one log entry, so multiply by as
			// mine as input logs
			expectedLogs := plog.NewLogs()
			for i := 0; i < tt.nLogs; i++ {
				rl := expectedLogs.ResourceLogs().AppendEmpty()
				expected.ResourceLogs().At(0).Resource().CopyTo(rl.Resource())
				sl := rl.ScopeLogs()
				expected.ResourceLogs().At(0).ScopeLogs().CopyTo(sl)
			}

			require.NoError(t, plogtest.CompareLogs(expectedLogs, logs))
		})
	}
}

func TestPayloads(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		logFilename      string
		expectedFilename string
		expectedErr      string
	}{
		{
			name:             "audit log - activity",
			logFilename:      "testdata/auditlog/activity.json",
			expectedFilename: "testdata/auditlog/activity_expected.yaml",
		},
		{
			name:             "audit log - data access",
			logFilename:      "testdata/auditlog/data_access.json",
			expectedFilename: "testdata/auditlog/data_access_expected.yaml",
		},
		{
			name:             "audit log - policy",
			logFilename:      "testdata/auditlog/policy.json",
			expectedFilename: "testdata/auditlog/policy_expected.yaml",
		},
		{
			name:             "audit log - system event",
			logFilename:      "testdata/auditlog/system_event.json",
			expectedFilename: "testdata/auditlog/system_event_expected.yaml",
		},
		{
			name:             "vpc flow log - 0 bytes sent",
			logFilename:      "testdata/vpc-flow-log/vpc-flow-log-0-bytes-sent.json",
			expectedFilename: "testdata/vpc-flow-log/vpc-flow-log-0-bytes-sent_expected.yaml",
		},
		{
			name:             "vpc flow log - 19kb sent",
			logFilename:      "testdata/vpc-flow-log/vpc-flow-log-19kb-sent.json",
			expectedFilename: "testdata/vpc-flow-log/vpc-flow-log-19kb-sent_expected.yaml",
		},
		{
			name:             "vpc flow log - 800 bytes sent",
			logFilename:      "testdata/vpc-flow-log/vpc-flow-log-800-bytes-sent.json",
			expectedFilename: "testdata/vpc-flow-log/vpc-flow-log-800-bytes-sent_expected.yaml",
		},
		{
			name:             "vpc flow log - from compute engine",
			logFilename:      "testdata/vpc-flow-log/vpc-flow-log-from-computeengine.json",
			expectedFilename: "testdata/vpc-flow-log/vpc-flow-log-from-computeengine_expected.yaml",
		},
		{
			name:             "vpc flow log - with dest vpc",
			logFilename:      "testdata/vpc-flow-log/vpc-flow-log-w-dest-vpc.json",
			expectedFilename: "testdata/vpc-flow-log/vpc-flow-log-w-dest-vpc_expected.yaml",
		},
		{
			name:             "vpc flow log - with internet routing details",
			logFilename:      "testdata/vpc-flow-log/vpc-flow-log-w-internet-routing-details.json",
			expectedFilename: "testdata/vpc-flow-log/vpc-flow-log-w-internet-routing-details_expected.yaml",
		},
		{
			name:             "vpc flow log - google services",
			logFilename:      "testdata/vpc-flow-log/vpc-flow-log-google-service.json",
			expectedFilename: "testdata/vpc-flow-log/vpc-flow-log-google-service_expected.yaml",
		},
		{
			name:             "vpc flow log - managed instance mig regions",
			logFilename:      "testdata/vpc-flow-log/vpc-flow-log-managed-instance.json",
			expectedFilename: "testdata/vpc-flow-log/vpc-flow-log-managed-instance_expected.yaml",
		},
		{
			name:             "armor log - enforced security policy",
			logFilename:      "testdata/armorlog/enforced_security_policy.json",
			expectedFilename: "testdata/armorlog/enforced_security_policy_expected.yaml",
		},
		{
			name:             "armor log - enforced edge security policy",
			logFilename:      "testdata/armorlog/enforced_edge_security_policy.json",
			expectedFilename: "testdata/armorlog/enforced_edge_security_policy_expected.yaml",
		},
		{
			name:             "armor log - two security policies",
			logFilename:      "testdata/armorlog/two_security_policies.json",
			expectedFilename: "testdata/armorlog/two_security_policies_expected.yaml",
		},
		{
			name:             "application load balancer log - regional",
			logFilename:      "testdata/apploadbalancer/regional_external_application_load_balancer.json",
			expectedFilename: "testdata/apploadbalancer/regional_external_application_load_balancer_expected.yaml",
		},
		{
			name:             "application load balancer log - global",
			logFilename:      "testdata/apploadbalancer/global_external_application_load_balancer.json",
			expectedFilename: "testdata/apploadbalancer/global_external_application_load_balancer_expected.yaml",
		},
		{
			name:             "proxy nlb log - basic connection",
			logFilename:      "testdata/proxynlb/proxynlb-basic.json",
			expectedFilename: "testdata/proxynlb/proxynlb-basic_expected.yaml",
		},
		{
			name:             "passthrough nlb log - external",
			logFilename:      "testdata/passthroughnlb/passthroughnlb-external.json",
			expectedFilename: "testdata/passthroughnlb/passthroughnlb-external_expected.yaml",
		},
		{
			name:             "passthrough nlb log - internal",
			logFilename:      "testdata/passthroughnlb/passthroughnlb-internal.json",
			expectedFilename: "testdata/passthroughnlb/passthroughnlb-internal_expected.yaml",
		},
		{
			name:             "dns query log - no error",
			logFilename:      "testdata/dnslog/dns_query_no_error.json",
			expectedFilename: "testdata/dnslog/dns_query_no_error_expected.yaml",
		},
		{
			name:             "dns query log - nxdomain error",
			logFilename:      "testdata/dnslog/dns_query_no_domain_error.json",
			expectedFilename: "testdata/dnslog/dns_query_no_domain_error_expected.yaml",
		},
	}

	extension := newTestExtension(t, Config{})
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			data, err := os.ReadFile(tt.logFilename)
			require.NoError(t, err)

			// Logs are expected to be one JSON object per line, so we compact them.
			content := bytes.NewBuffer(nil)
			compactionErr := gojson.Compact(content, data)
			require.NoError(t, compactionErr)

			logs, err := extension.UnmarshalLogs(content.Bytes())
			require.NoError(t, err)

			// write expected log with:
			// golden.WriteLogs(t, tt.expectedFilename, logs)

			expectedLogs, err := golden.ReadLogs(tt.expectedFilename)
			require.NoError(t, err)
			require.NoError(t, plogtest.CompareLogs(expectedLogs, logs))
		})
	}
}

func TestEncodingFormatScopeAttributeUnknownLogs(t *testing.T) {
	// note that testing for known log types is already done in TestPayloads,
	// so we don't have tests here for known log types.
	tests := []struct {
		name           string
		logName        string
		expectedFormat string
	}{
		{
			name:           "unknown log type",
			logName:        "projects/test-project/logs/unknown-log-type",
			expectedFormat: "",
		},
		{
			name:           "generic log type",
			logName:        "projects/test-project/logs/generic-log",
			expectedFormat: "",
		},
	}

	extension := newTestExtension(t, Config{})

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Create a minimal log entry that won't trigger payload processing errors
			logLine := fmt.Appendf(nil, `{"logName": "%s", "timestamp": "2024-05-05T10:31:19.45570687Z", "textPayload": "test message"}`, tt.logName)

			logs, err := extension.UnmarshalLogs(logLine)
			require.NoError(t, err)

			require.Equal(t, 1, logs.ResourceLogs().Len())
			resourceLogs := logs.ResourceLogs().At(0)
			require.Equal(t, 1, resourceLogs.ScopeLogs().Len())
			scopeLogs := resourceLogs.ScopeLogs().At(0)

			scopeAttrs := scopeLogs.Scope().Attributes()

			_, exists := scopeAttrs.Get(constants.FormatIdentificationTag)
			require.False(t, exists, "encoding.format attribute should not exist for unknown log types")
		})
	}
}

func TestGetEncodingFormatFunction(t *testing.T) {
	tests := []struct {
		name           string
		logType        string
		expectedFormat string
	}{
		{
			name:           "audit log activity",
			logType:        auditlog.ActivityLogNameSuffix,
			expectedFormat: constants.GCPFormatAuditLog,
		},
		{
			name:           "audit log data access",
			logType:        auditlog.DataAccessLogNameSuffix,
			expectedFormat: constants.GCPFormatAuditLog,
		},
		{
			name:           "audit log system event",
			logType:        auditlog.SystemEventLogNameSuffix,
			expectedFormat: constants.GCPFormatAuditLog,
		},
		{
			name:           "audit log policy",
			logType:        auditlog.PolicyLogNameSuffix,
			expectedFormat: constants.GCPFormatAuditLog,
		},
		{
			name:           "vpc flow log network management",
			logType:        vpcflowlog.NetworkManagementNameSuffix,
			expectedFormat: constants.GCPFormatVPCFlowLog,
		},
		{
			name:           "vpc flow log compute",
			logType:        vpcflowlog.ComputeNameSuffix,
			expectedFormat: constants.GCPFormatVPCFlowLog,
		},
		{
			name:           "proxy nlb log connections",
			logType:        proxynlb.ConnectionsLogNameSuffix,
			expectedFormat: constants.GCPFormatProxyNLBLog,
		},
		{
			name:           "passthrough nlb log connections",
			logType:        passthroughnlb.ConnectionsLogNameSuffix,
			expectedFormat: constants.GCPFormatPassthroughNLBLog,
		},
		{
			name:           "unknown log type",
			logType:        "unknown-log-type",
			expectedFormat: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := getEncodingFormat(tt.logType)
			require.Equal(t, tt.expectedFormat, result)
		})
	}
}
