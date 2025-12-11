// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func Test_parseLEEF(t *testing.T) {
	tests := []struct {
		name     string
		target   ottl.StringGetter[any]
		expected map[string]any
	}{
		{
			name: "LEEF 1.0 simple",
			target: ottl.StandardStringGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return "LEEF:1.0|Microsoft|MSExchange|4.0 SP1|15345|src=10.50.1.1\tdst=2.10.20.20\tsev=5", nil
				},
			},
			expected: map[string]any{
				"version":         "1.0",
				"vendor":          "Microsoft",
				"product_name":    "MSExchange",
				"product_version": "4.0 SP1",
				"event_id":        "15345",
				"attributes": map[string]any{
					"src": "10.50.1.1",
					"dst": "2.10.20.20",
					"sev": "5",
				},
			},
		},
		{
			name: "LEEF 1.0 with many attributes",
			target: ottl.StandardStringGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return "LEEF:1.0|QRadar|QRM|1.0|NEW_PORT_DISCOVERED|src=7.5.6.6\tdst=172.50.123.1\tsev=5\tcat=anomaly\tsrcPort=3881\tdstPort=21\tusrName=joe.black", nil
				},
			},
			expected: map[string]any{
				"version":         "1.0",
				"vendor":          "QRadar",
				"product_name":    "QRM",
				"product_version": "1.0",
				"event_id":        "NEW_PORT_DISCOVERED",
				"attributes": map[string]any{
					"src":     "7.5.6.6",
					"dst":     "172.50.123.1",
					"sev":     "5",
					"cat":     "anomaly",
					"srcPort": "3881",
					"dstPort": "21",
					"usrName": "joe.black",
				},
			},
		},
		{
			name: "LEEF 1.0 header only no attributes",
			target: ottl.StandardStringGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return "LEEF:1.0|Vendor|Product|1.0|EventID|", nil
				},
			},
			expected: map[string]any{
				"version":         "1.0",
				"vendor":          "Vendor",
				"product_name":    "Product",
				"product_version": "1.0",
				"event_id":        "EventID",
				"attributes":      map[string]any{},
			},
		},
		{
			name: "LEEF 1.0 no trailing pipe",
			target: ottl.StandardStringGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return "LEEF:1.0|Vendor|Product|1.0|EventID", nil
				},
			},
			expected: map[string]any{
				"version":         "1.0",
				"vendor":          "Vendor",
				"product_name":    "Product",
				"product_version": "1.0",
				"event_id":        "EventID",
				"attributes":      map[string]any{},
			},
		},
		{
			name: "LEEF 2.0 with caret delimiter",
			target: ottl.StandardStringGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return "LEEF:2.0|Lancope|StealthWatch|1.0|41|^|src=10.0.1.8^dst=10.0.0.5^sev=5", nil
				},
			},
			expected: map[string]any{
				"version":         "2.0",
				"vendor":          "Lancope",
				"product_name":    "StealthWatch",
				"product_version": "1.0",
				"event_id":        "41",
				"attributes": map[string]any{
					"src": "10.0.1.8",
					"dst": "10.0.0.5",
					"sev": "5",
				},
			},
		},
		{
			name: "LEEF 2.0 with hex tab delimiter",
			target: ottl.StandardStringGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return "LEEF:2.0|Vendor|Product|1.0|100|0x09|key1=val1\tkey2=val2", nil
				},
			},
			expected: map[string]any{
				"version":         "2.0",
				"vendor":          "Vendor",
				"product_name":    "Product",
				"product_version": "1.0",
				"event_id":        "100",
				"attributes": map[string]any{
					"key1": "val1",
					"key2": "val2",
				},
			},
		},
		{
			name: "LEEF 2.0 with hex caret delimiter",
			target: ottl.StandardStringGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return "LEEF:2.0|Vendor|Product|1.0|100|0x5e|key1=val1^key2=val2", nil
				},
			},
			expected: map[string]any{
				"version":         "2.0",
				"vendor":          "Vendor",
				"product_name":    "Product",
				"product_version": "1.0",
				"event_id":        "100",
				"attributes": map[string]any{
					"key1": "val1",
					"key2": "val2",
				},
			},
		},
		{
			name: "LEEF 2.0 with empty delimiter defaults to tab",
			target: ottl.StandardStringGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return "LEEF:2.0|Vendor|Product|1.0|100||key1=val1\tkey2=val2", nil
				},
			},
			expected: map[string]any{
				"version":         "2.0",
				"vendor":          "Vendor",
				"product_name":    "Product",
				"product_version": "1.0",
				"event_id":        "100",
				"attributes": map[string]any{
					"key1": "val1",
					"key2": "val2",
				},
			},
		},
		{
			name: "LEEF 2.0 header only",
			target: ottl.StandardStringGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return "LEEF:2.0|Vendor|Product|1.0|EventID|^|", nil
				},
			},
			expected: map[string]any{
				"version":         "2.0",
				"vendor":          "Vendor",
				"product_name":    "Product",
				"product_version": "1.0",
				"event_id":        "EventID",
				"attributes":      map[string]any{},
			},
		},
		{
			name: "LEEF 2.0 no trailing pipe after delimiter",
			target: ottl.StandardStringGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return "LEEF:2.0|Vendor|Product|1.0|EventID|^", nil
				},
			},
			expected: map[string]any{
				"version":         "2.0",
				"vendor":          "Vendor",
				"product_name":    "Product",
				"product_version": "1.0",
				"event_id":        "EventID",
				"attributes":      map[string]any{},
			},
		},
		{
			name: "attribute value with spaces",
			target: ottl.StandardStringGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return "LEEF:1.0|Vendor|Product|1.0|Event|msg=This is a message with spaces\tsrc=1.2.3.4", nil
				},
			},
			expected: map[string]any{
				"version":         "1.0",
				"vendor":          "Vendor",
				"product_name":    "Product",
				"product_version": "1.0",
				"event_id":        "Event",
				"attributes": map[string]any{
					"msg": "This is a message with spaces",
					"src": "1.2.3.4",
				},
			},
		},
		{
			name: "attribute value with equals sign",
			target: ottl.StandardStringGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return "LEEF:1.0|Vendor|Product|1.0|Event|url=http://example.com?foo=bar\tsrc=1.2.3.4", nil
				},
			},
			expected: map[string]any{
				"version":         "1.0",
				"vendor":          "Vendor",
				"product_name":    "Product",
				"product_version": "1.0",
				"event_id":        "Event",
				"attributes": map[string]any{
					"url": "http://example.com?foo=bar",
					"src": "1.2.3.4",
				},
			},
		},
		{
			name: "attribute with empty value",
			target: ottl.StandardStringGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return "LEEF:1.0|Vendor|Product|1.0|Event|key1=\tkey2=value2", nil
				},
			},
			expected: map[string]any{
				"version":         "1.0",
				"vendor":          "Vendor",
				"product_name":    "Product",
				"product_version": "1.0",
				"event_id":        "Event",
				"attributes": map[string]any{
					"key1": "",
					"key2": "value2",
				},
			},
		},
		{
			name: "LEEF 2.0 uppercase hex",
			target: ottl.StandardStringGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return "LEEF:2.0|Vendor|Product|1.0|100|0X5E|key1=val1^key2=val2", nil
				},
			},
			expected: map[string]any{
				"version":         "2.0",
				"vendor":          "Vendor",
				"product_name":    "Product",
				"product_version": "1.0",
				"event_id":        "100",
				"attributes": map[string]any{
					"key1": "val1",
					"key2": "val2",
				},
			},
		},
		{
			name: "header fields with special characters",
			target: ottl.StandardStringGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return "LEEF:1.0|Vendor-Name_123|Product.Name|1.0-beta|Event_ID_123|key=value", nil
				},
			},
			expected: map[string]any{
				"version":         "1.0",
				"vendor":          "Vendor-Name_123",
				"product_name":    "Product.Name",
				"product_version": "1.0-beta",
				"event_id":        "Event_ID_123",
				"attributes": map[string]any{
					"key": "value",
				},
			},
		},
		{
			name: "real world QRadar example",
			target: ottl.StandardStringGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return "LEEF:2.0|IBM|QRadar|7.3.2|Authentication|^|src=192.168.1.100^dst=10.0.0.1^usrName=admin^cat=auth^sev=3^devTime=Jan 15 2024 10:30:45^devTimeFormat=MMM dd yyyy HH:mm:ss", nil
				},
			},
			expected: map[string]any{
				"version":         "2.0",
				"vendor":          "IBM",
				"product_name":    "QRadar",
				"product_version": "7.3.2",
				"event_id":        "Authentication",
				"attributes": map[string]any{
					"src":           "192.168.1.100",
					"dst":           "10.0.0.1",
					"usrName":       "admin",
					"cat":           "auth",
					"sev":           "3",
					"devTime":       "Jan 15 2024 10:30:45",
					"devTimeFormat": "MMM dd yyyy HH:mm:ss",
				},
			},
		},
		{
			name: "network security event",
			target: ottl.StandardStringGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return "LEEF:1.0|Cisco|ASA|9.8|FirewallDeny|src=10.1.1.1\tdst=192.168.1.1\tsrcPort=12345\tdstPort=443\tproto=TCP\tsev=7", nil
				},
			},
			expected: map[string]any{
				"version":         "1.0",
				"vendor":          "Cisco",
				"product_name":    "ASA",
				"product_version": "9.8",
				"event_id":        "FirewallDeny",
				"attributes": map[string]any{
					"src":     "10.1.1.1",
					"dst":     "192.168.1.1",
					"srcPort": "12345",
					"dstPort": "443",
					"proto":   "TCP",
					"sev":     "7",
				},
			},
		},
		{
			name: "duplicate delimiter in attributes section",
			target: ottl.StandardStringGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return "LEEF:2.0|Vendor|Product|1.0|Event|^|key1=val1^^key2=val2", nil
				},
			},
			expected: map[string]any{
				"version":         "2.0",
				"vendor":          "Vendor",
				"product_name":    "Product",
				"product_version": "1.0",
				"event_id":        "Event",
				"attributes": map[string]any{
					"key1": "val1",
					"key2": "val2",
				},
			},
		},
		{
			name: "trailing delimiter in attributes",
			target: ottl.StandardStringGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return "LEEF:1.0|Vendor|Product|1.0|Event|key1=val1\tkey2=val2\t", nil
				},
			},
			expected: map[string]any{
				"version":         "1.0",
				"vendor":          "Vendor",
				"product_name":    "Product",
				"product_version": "1.0",
				"event_id":        "Event",
				"attributes": map[string]any{
					"key1": "val1",
					"key2": "val2",
				},
			},
		},
		{
			name: "leading delimiter in attributes",
			target: ottl.StandardStringGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return "LEEF:1.0|Vendor|Product|1.0|Event|\tkey1=val1\tkey2=val2", nil
				},
			},
			expected: map[string]any{
				"version":         "1.0",
				"vendor":          "Vendor",
				"product_name":    "Product",
				"product_version": "1.0",
				"event_id":        "Event",
				"attributes": map[string]any{
					"key1": "val1",
					"key2": "val2",
				},
			},
		},
		{
			name: "IBM Guardium login failure event with syslog header",
			target: ottl.StandardStringGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					// Full sample from https://www.ibm.com/docs/en/dsm?topic=guardium-sample-event-messages
					// Includes syslog header (RFC 3164 format)
					// Note: LEEF 1.0 uses tab delimiter for attributes per spec at
					// https://www.ibm.com/docs/en/dsm?topic=overview-leef-event-components
					return "<30>Aug 19 12:33:31 ibm.guardium.test guard_sender[4486]: LEEF:1.0|IBM|Guardium|8.0|Login failures|ruleID=20026\truleDesc=Login failures\tseverity=INFO\tdevTime=2013-8-19 6:34:41\tserverType=DB2\tclassification=\tcategory=\tdbProtocolVersion=3.0\tusrName=\tsourceProgram=DB2JCC_APPLICATION\tstart=1376908481000\tdbUser=user\tdst=10.30.2.124\tdstPort=50000\tsrc=10.30.5.152\tsrcPort=38754\tprotocol=TCP\ttype=LOGIN_FAILED\tviolationID=15\tsql=\terror=08001-XXXX:30082-01", nil
				},
			},
			expected: map[string]any{
				"version":         "1.0",
				"vendor":          "IBM",
				"product_name":    "Guardium",
				"product_version": "8.0",
				"event_id":        "Login failures",
				"attributes": map[string]any{
					"ruleID":            "20026",
					"ruleDesc":          "Login failures",
					"severity":          "INFO",
					"devTime":           "2013-8-19 6:34:41",
					"serverType":        "DB2",
					"classification":    "",
					"category":          "",
					"dbProtocolVersion": "3.0",
					"usrName":           "",
					"sourceProgram":     "DB2JCC_APPLICATION",
					"start":             "1376908481000",
					"dbUser":            "user",
					"dst":               "10.30.2.124",
					"dstPort":           "50000",
					"src":               "10.30.5.152",
					"srcPort":           "38754",
					"protocol":          "TCP",
					"type":              "LOGIN_FAILED",
					"violationID":       "15",
					"sql":               "",
					"error":             "08001-XXXX:30082-01",
				},
			},
		},
		{
			name: "IBM Guardium unauthorized access event with syslog header",
			target: ottl.StandardStringGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					// Full sample from https://www.ibm.com/docs/en/dsm?topic=guardium-sample-event-messages
					// Includes syslog header (RFC 3164 format)
					// Note: LEEF 1.0 uses tab delimiter for attributes per spec at
					// https://www.ibm.com/docs/en/dsm?topic=overview-leef-event-components
					return "<25>Jun 11 13:47:19 ibm.guardium.test guard_sender[3432]: LEEF:1.0|IBM|Guardium|8.0|Unauthorized Users on Cardholder Objects - Alert|ruleID=159\truleDesc=Unauthorized Users on Cardholder Objects - Alert\tseverity=MED\tdevTime=2013-6-11 12:46:21\tserverType=MS SQL SERVER\tclassification=Violation\tcategory=PCI\tdbProtocolVersion=8.0\tusrName=\tsourceProgram=ABCDEF.EXE\tstart=1370965581000\tdbUser=SYSTEM\tdst=172.16.107.92\tdstPort=1433\tsrc=172.16.107.92\tsrcPort=60621\tprotocol=TCP\ttype=SQL_LANG\tviolationID=0\tsql=SELECT * FROM EPOAgentHandlerAssignment INNER JOIN EPOAgentHandlerAssignmentPriority ON (EPOAgentHandlerAssignment.AutoID = EPOAgentHandlerAssignmentPriority.AssignmentID) ORDER BY EPOAgentHandlerAssignmentPriority.Priority ASC\terror=TDS_MS-", nil
				},
			},
			expected: map[string]any{
				"version":         "1.0",
				"vendor":          "IBM",
				"product_name":    "Guardium",
				"product_version": "8.0",
				"event_id":        "Unauthorized Users on Cardholder Objects - Alert",
				"attributes": map[string]any{
					"ruleID":            "159",
					"ruleDesc":          "Unauthorized Users on Cardholder Objects - Alert",
					"severity":          "MED",
					"devTime":           "2013-6-11 12:46:21",
					"serverType":        "MS SQL SERVER",
					"classification":    "Violation",
					"category":          "PCI",
					"dbProtocolVersion": "8.0",
					"usrName":           "",
					"sourceProgram":     "ABCDEF.EXE",
					"start":             "1370965581000",
					"dbUser":            "SYSTEM",
					"dst":               "172.16.107.92",
					"dstPort":           "1433",
					"src":               "172.16.107.92",
					"srcPort":           "60621",
					"protocol":          "TCP",
					"type":              "SQL_LANG",
					"violationID":       "0",
					"sql":               "SELECT * FROM EPOAgentHandlerAssignment INNER JOIN EPOAgentHandlerAssignmentPriority ON (EPOAgentHandlerAssignment.AutoID = EPOAgentHandlerAssignmentPriority.AssignmentID) ORDER BY EPOAgentHandlerAssignmentPriority.Priority ASC",
					"error":             "TDS_MS-",
				},
			},
		},
		{
			name: "syslog header RFC 5424 format",
			target: ottl.StandardStringGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					// RFC 5424 syslog format with structured data
					return "<113>1 2019-01-18T11:07:53.520+07:00 hostname LEEF:2.0|Lancope|StealthWatch|1.0|41|^|src=10.0.1.8^dst=10.0.0.5^sev=5", nil
				},
			},
			expected: map[string]any{
				"version":         "2.0",
				"vendor":          "Lancope",
				"product_name":    "StealthWatch",
				"product_version": "1.0",
				"event_id":        "41",
				"attributes": map[string]any{
					"src": "10.0.1.8",
					"dst": "10.0.0.5",
					"sev": "5",
				},
			},
		},
		{
			name: "syslog header simple",
			target: ottl.StandardStringGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return "<13>Jan 18 11:07:53 192.168.1.1 LEEF:1.0|Microsoft|MSExchange|4.0 SP1|15345|src=192.0.2.0\tdst=172.50.123.1", nil
				},
			},
			expected: map[string]any{
				"version":         "1.0",
				"vendor":          "Microsoft",
				"product_name":    "MSExchange",
				"product_version": "4.0 SP1",
				"event_id":        "15345",
				"attributes": map[string]any{
					"src": "192.0.2.0",
					"dst": "172.50.123.1",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc := parseLEEF(tt.target)
			result, err := exprFunc(t.Context(), nil)
			require.NoError(t, err)

			resultMap, ok := result.(pcommon.Map)
			require.True(t, ok, "result should be pcommon.Map")

			// Check top-level fields
			assertMapValue(t, resultMap, "version", tt.expected["version"])
			assertMapValue(t, resultMap, "vendor", tt.expected["vendor"])
			assertMapValue(t, resultMap, "product_name", tt.expected["product_name"])
			assertMapValue(t, resultMap, "product_version", tt.expected["product_version"])
			assertMapValue(t, resultMap, "event_id", tt.expected["event_id"])

			// Check attributes
			expectedAttrs := tt.expected["attributes"].(map[string]any)
			attrsVal, ok := resultMap.Get("attributes")
			require.True(t, ok, "attributes field should exist")
			attrsMap := attrsVal.Map()
			assert.Equal(t, len(expectedAttrs), attrsMap.Len(), "attributes count mismatch")

			for k, v := range expectedAttrs {
				attrVal, ok := attrsMap.Get(k)
				assert.True(t, ok, "attribute %q should exist", k)
				assert.Equal(t, v, attrVal.Str(), "attribute %q value mismatch", k)
			}
		})
	}
}

func Test_parseLEEF_error(t *testing.T) {
	tests := []struct {
		name          string
		target        ottl.StringGetter[any]
		expectedError string
	}{
		{
			name: "empty input",
			target: ottl.StandardStringGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return "", nil
				},
			},
			expectedError: "cannot parse empty LEEF message",
		},
		{
			name: "not a LEEF message",
			target: ottl.StandardStringGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return "CEF:0|Vendor|Product|1.0|100|Event Name|5|src=1.2.3.4", nil
				},
			},
			expectedError: "'LEEF:' not found",
		},
		{
			name: "unsupported LEEF version",
			target: ottl.StandardStringGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return "LEEF:3.0|Vendor|Product|1.0|EventID|key=value", nil
				},
			},
			expectedError: "unsupported LEEF version: 3.0",
		},
		{
			name: "invalid LEEF version format",
			target: ottl.StandardStringGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return "LEEF:abc|Vendor|Product|1.0|EventID|key=value", nil
				},
			},
			expectedError: "unsupported LEEF version: abc",
		},
		{
			name: "missing pipes in header",
			target: ottl.StandardStringGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return "LEEF:1.0|OnlyVendor", nil
				},
			},
			expectedError: "invalid LEEF 1.0 header: expected at least 4 fields",
		},
		{
			name: "LEEF 1.0 too few header fields",
			target: ottl.StandardStringGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return "LEEF:1.0|Vendor|Product", nil
				},
			},
			expectedError: "invalid LEEF 1.0 header: expected at least 4 fields",
		},
		{
			name: "LEEF 2.0 too few header fields",
			target: ottl.StandardStringGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return "LEEF:2.0|Vendor|Product|1.0", nil
				},
			},
			expectedError: "invalid LEEF 2.0 header: expected at least 5 fields",
		},
		{
			name: "no pipe delimiter at all",
			target: ottl.StandardStringGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return "LEEF:1.0", nil
				},
			},
			expectedError: "missing pipe delimiter in header",
		},
		{
			name: "invalid hex delimiter - odd length",
			target: ottl.StandardStringGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return "LEEF:2.0|Vendor|Product|1.0|EventID|0x9|key=value", nil
				},
			},
			expectedError: "invalid hex delimiter",
		},
		{
			name: "invalid hex delimiter - not hex",
			target: ottl.StandardStringGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return "LEEF:2.0|Vendor|Product|1.0|EventID|0xGG|key=value", nil
				},
			},
			expectedError: "invalid hex delimiter",
		},
		{
			name: "invalid hex delimiter - too many bytes",
			target: ottl.StandardStringGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return "LEEF:2.0|Vendor|Product|1.0|EventID|0x0909|key=value", nil
				},
			},
			expectedError: "hex delimiter must decode to a single byte",
		},
		{
			name: "invalid hex delimiter - empty hex",
			target: ottl.StandardStringGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return "LEEF:2.0|Vendor|Product|1.0|EventID|0x|key=value", nil
				},
			},
			expectedError: "empty hex value",
		},
		{
			name: "plain text not LEEF",
			target: ottl.StandardStringGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return "This is just plain text log message", nil
				},
			},
			expectedError: "'LEEF:' not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc := parseLEEF(tt.target)
			_, err := exprFunc(t.Context(), nil)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedError)
		})
	}
}

func Test_parseLEEF_target_error(t *testing.T) {
	target := ottl.StandardStringGetter[any]{
		Getter: func(context.Context, any) (any, error) {
			return nil, assert.AnError
		},
	}
	exprFunc := parseLEEF(target)
	_, err := exprFunc(t.Context(), nil)
	require.Error(t, err)
}

func Test_createParseLEEFFunction(t *testing.T) {
	factory := NewParseLEEFFactory[any]()
	assert.Equal(t, "ParseLEEF", factory.Name())

	args := &ParseLEEFArguments[any]{
		Target: ottl.StandardStringGetter[any]{
			Getter: func(context.Context, any) (any, error) {
				return "LEEF:1.0|Vendor|Product|1.0|Event|key=value", nil
			},
		},
	}

	exprFunc, err := factory.CreateFunction(ottl.FunctionContext{}, args)
	require.NoError(t, err)

	result, err := exprFunc(t.Context(), nil)
	require.NoError(t, err)
	assert.NotNil(t, result)
}

func Test_createParseLEEFFunction_wrongArgs(t *testing.T) {
	factory := NewParseLEEFFactory[any]()

	_, err := factory.CreateFunction(ottl.FunctionContext{}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ParseLEEFFactory args must be of type *ParseLEEFArguments[K]")
}

func Test_parseDelimiter(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
		hasError bool
	}{
		{
			name:     "empty defaults to tab",
			input:    "",
			expected: "\t",
		},
		{
			name:     "single character",
			input:    "^",
			expected: "^",
		},
		{
			name:     "pipe character",
			input:    "|",
			expected: "|",
		},
		{
			name:     "hex tab",
			input:    "0x09",
			expected: "\t",
		},
		{
			name:     "hex caret lowercase",
			input:    "0x5e",
			expected: "^",
		},
		{
			name:     "hex caret uppercase",
			input:    "0x5E",
			expected: "^",
		},
		{
			name:     "hex with uppercase prefix",
			input:    "0X5e",
			expected: "^",
		},
		{
			name:     "hex space",
			input:    "0x20",
			expected: " ",
		},
		{
			name:     "multi-character delimiter",
			input:    "||",
			expected: "||",
		},
		{
			name:     "invalid hex - odd length",
			input:    "0x9",
			hasError: true,
		},
		{
			name:     "invalid hex - not hex chars",
			input:    "0xZZ",
			hasError: true,
		},
		{
			name:     "invalid hex - too many bytes",
			input:    "0x0909",
			hasError: true,
		},
		{
			name:     "invalid hex - empty",
			input:    "0x",
			hasError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseDelimiter(tt.input)
			if tt.hasError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func Test_parseLEEFAttributes(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		delimiter string
		expected  map[string]any
	}{
		{
			name:      "simple tab delimited",
			input:     "key1=val1\tkey2=val2",
			delimiter: "\t",
			expected: map[string]any{
				"key1": "val1",
				"key2": "val2",
			},
		},
		{
			name:      "caret delimited",
			input:     "key1=val1^key2=val2^key3=val3",
			delimiter: "^",
			expected: map[string]any{
				"key1": "val1",
				"key2": "val2",
				"key3": "val3",
			},
		},
		{
			name:      "empty attributes",
			input:     "",
			delimiter: "\t",
			expected:  map[string]any{},
		},
		{
			name:      "value with equals",
			input:     "url=http://example.com?a=b",
			delimiter: "\t",
			expected: map[string]any{
				"url": "http://example.com?a=b",
			},
		},
		{
			name:      "key without value skipped",
			input:     "key1=val1\tkeyonly\tkey2=val2",
			delimiter: "\t",
			expected: map[string]any{
				"key1": "val1",
				"key2": "val2",
			},
		},
		{
			name:      "empty value",
			input:     "key1=\tkey2=val2",
			delimiter: "\t",
			expected: map[string]any{
				"key1": "",
				"key2": "val2",
			},
		},
		{
			name:      "whitespace handling",
			input:     "  key1=val1  \t  key2=val2  ",
			delimiter: "\t",
			expected: map[string]any{
				"key1": "val1",
				"key2": "val2",
			},
		},
		{
			name:      "duplicate delimiters",
			input:     "key1=val1^^key2=val2",
			delimiter: "^",
			expected: map[string]any{
				"key1": "val1",
				"key2": "val2",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseLEEFAttributes(tt.input, tt.delimiter)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func assertMapValue(t *testing.T, m pcommon.Map, key string, expected any) {
	t.Helper()
	val, ok := m.Get(key)
	require.True(t, ok, "key %q should exist", key)
	assert.Equal(t, expected, val.Str(), "value for key %q mismatch", key)
}
