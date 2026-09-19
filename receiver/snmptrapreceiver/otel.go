// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package snmptrapreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/snmptrapreceiver"

import (
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/snmptrapreceiver/internal/metadata"
)

// logsFromRecord builds one OTel log record. Identity fields are both
// attributes (for backends that promote attributes) and JSON body.
func logsFromRecord(rec Record, extra map[string]string) plog.Logs {
	ld := plog.NewLogs()
	rl := ld.ResourceLogs().AppendEmpty()
	sl := rl.ScopeLogs().AppendEmpty()
	sl.Scope().SetName(metadata.ScopeName)

	lr := sl.LogRecords().AppendEmpty()
	if !rec.Time.IsZero() {
		lr.SetTimestamp(pcommon.NewTimestampFromTime(rec.Time))
	}
	lr.SetObservedTimestamp(pcommon.NewTimestampFromTime(time.Now()))

	if body, err := rec.MarshalJSONLine(); err == nil {
		lr.Body().SetStr(string(body))
	}

	attrs := lr.Attributes()
	putStr(attrs, "source", rec.Source)
	putStr(attrs, "trap_oid", rec.TrapOID)
	putStr(attrs, "trap_name", rec.TrapName)
	putStr(attrs, "trap_mib", rec.MIB)
	putStr(attrs, "snmp_version", rec.Version)
	putStr(attrs, "pdu_type", rec.PDUType)
	putStr(attrs, "agent_address", rec.AgentAddress)
	putStr(attrs, "engine_id", rec.EngineID)
	putStr(attrs, "context_name", rec.ContextName)
	putStr(attrs, "community", rec.Community)
	for k, v := range extra {
		putStr(attrs, k, v)
	}
	return ld
}

func putStr(attrs pcommon.Map, key, val string) {
	if val == "" || key == "" {
		return
	}
	attrs.PutStr(key, val)
}
