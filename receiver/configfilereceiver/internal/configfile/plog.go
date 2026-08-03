package configfile

import (
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
)

const ServiceName = "asama-configfile"

// SnapshotsToLogs converts snapshots into OTLP pdata logs for the collector pipeline.
func SnapshotsToLogs(snaps []*Snapshot) plog.Logs {
	ld := plog.NewLogs()
	if len(snaps) == 0 {
		return ld
	}

	rl := ld.ResourceLogs().AppendEmpty()
	rl.Resource().Attributes().PutStr("service.name", ServiceName)
	sl := rl.ScopeLogs().AppendEmpty()
	sl.Scope().SetName(ServiceName)
	lr := sl.LogRecords()

	for _, snap := range snaps {
		if snap == nil {
			continue
		}
		rec := lr.AppendEmpty()
		rec.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		rec.SetSeverityNumber(plog.SeverityNumberInfo)
		rec.SetSeverityText("INFO")
		rec.Body().SetStr("configfile snapshot")

		attrs := rec.Attributes()
		attrs.PutStr("config.file", snap.File)
		attrs.PutStr("config.format", snap.Format)
		attrs.PutStr("config.checksum", snap.Checksum)
		attrs.PutInt("config.keys_total", int64(snap.KeysTotal))
		attrs.PutStr("config.event", snap.Event)
		for key, value := range snap.Keys {
			attrs.PutStr("config.key."+key, value)
		}
	}
	return ld
}

// LogRecordAttributes returns attribute map from a plog record (for tests).
func LogRecordAttributes(rec plog.LogRecord) map[string]string {
	out := make(map[string]string)
	rec.Attributes().Range(func(k string, v pcommon.Value) bool {
		out[k] = v.AsString()
		return true
	})
	return out
}
