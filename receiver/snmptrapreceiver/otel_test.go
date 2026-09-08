// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package snmptrapreceiver

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

func TestLogsFromRecord(t *testing.T) {
	rec := Record{
		Time:     time.Date(2026, 8, 18, 19, 0, 0, 0, time.UTC),
		Source:   "172.20.20.2",
		Version:  "2c",
		PDUType:  "trap",
		TrapOID:  "1.3.6.1.6.3.1.1.5.1",
		TrapName: "SNMPv2-MIB::coldStart",
		MIB:      "SNMPv2-MIB",
		Varbinds: []Varbind{{OID: "1.3.6.1.2.1.1.5.0", Type: "OctetString", Value: "spine1"}},
	}
	ld := logsFromRecord(rec, map[string]string{"job": "snmptrap"})
	require.Equal(t, 1, ld.LogRecordCount())
	lr := ld.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
	require.Equal(t, "snmptrap", mustAttr(t, lr.Attributes(), "job"))
	require.Equal(t, "1.3.6.1.6.3.1.1.5.1", mustAttr(t, lr.Attributes(), "trap_oid"))
	require.Equal(t, "2c", mustAttr(t, lr.Attributes(), "snmp_version"))

	var parsed map[string]any
	require.NoError(t, json.Unmarshal([]byte(lr.Body().AsString()), &parsed))
	require.Equal(t, "1.3.6.1.6.3.1.1.5.1", parsed["trap_oid"])
}

func TestCommunityAllowed(t *testing.T) {
	require.True(t, CommunityAllowed(nil, "anything"))
	require.True(t, CommunityAllowed([]string{"public"}, "public"))
	require.False(t, CommunityAllowed([]string{"public"}, "private"))
}

func mustAttr(t *testing.T, attrs pcommon.Map, key string) string {
	t.Helper()
	v, ok := attrs.Get(key)
	require.True(t, ok, "missing attribute %s", key)
	return v.AsString()
}
