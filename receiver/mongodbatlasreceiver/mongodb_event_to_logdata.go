// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package mongodbatlasreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbatlasreceiver"

import (
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/multierr"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbatlasreceiver/internal/model"
)

const (
	// Number of log attributes to add to the plog.LogRecordSlice for host logs.
	totalLogAttributes = 10
	// Number of log attributes to add to the plog.LogRecordSlice for audit logs.
	totalAuditLogAttributes = 16

	// Number of resource attributes to add to the plog.ResourceLogs.
	totalResourceAttributes = 4
)

// jsonTimestampLayout for the timestamp format in the plog.Logs structure
const (
	jsonTimestampLayout    = "2006-01-02T15:04:05.000-07:00"
	consoleTimestampLayout = "2006-01-02T15:04:05.000-0700"
)

// Severity mapping of the mongodb atlas logs
var severityMap = map[string]plog.SeverityNumber{
	"F":  plog.SeverityNumberFatal,
	"E":  plog.SeverityNumberError,
	"W":  plog.SeverityNumberWarn,
	"I":  plog.SeverityNumberInfo,
	"D":  plog.SeverityNumberDebug,
	"D1": plog.SeverityNumberDebug,
	"D2": plog.SeverityNumberDebug2,
	"D3": plog.SeverityNumberDebug3,
	"D4": plog.SeverityNumberDebug4,
	"D5": plog.SeverityNumberDebug4,
}

// mongoAuditEventToLogRecord converts model.AuditLog event to plog.LogRecordSlice and adds the resource attributes.
func mongodbAuditEventToLogData(logger *zap.Logger, logs []model.AuditLog, pc projectContext, hostname, logName string, clusterInfo clusterInfo) (plog.Logs, error) {
	ld := plog.NewLogs()
	rl := ld.ResourceLogs().AppendEmpty()
	sl := rl.ScopeLogs().AppendEmpty()

	resourceAttrs := rl.Resource().Attributes()
	resourceAttrs.EnsureCapacity(totalResourceAttributes)

	// Attributes related to the object causing the event.
	resourceAttrs.PutStr("mongodb_atlas.org", pc.orgName)
	resourceAttrs.PutStr("mongodb_atlas.project", pc.Project.Name)
	resourceAttrs.PutStr("mongodb_atlas.cluster", clusterInfo.ClusterName)
	resourceAttrs.PutStr("mongodb_atlas.region.name", clusterInfo.RegionName)
	resourceAttrs.PutStr("mongodb_atlas.provider.name", clusterInfo.ProviderName)
	resourceAttrs.PutStr("mongodb_atlas.host.name", hostname)

	var errs []error

	for _, log := range logs {
		lr := sl.LogRecords().AppendEmpty()

		logTsFormat := tsLayout(clusterInfo.MongoDBMajorVersion)
		t, err := time.Parse(logTsFormat, log.Timestamp.Date)
		if err != nil {
			logger.Warn("Time failed to parse correctly", zap.Error(err))
		}

		lr.SetTimestamp(pcommon.NewTimestampFromTime(t))
		lr.SetObservedTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		// Insert Raw Log message into Body of LogRecord
		lr.Body().SetStr(log.Raw)
		// Since Audit Logs don't have a severity/level
		// Set the "SeverityNumber" and "SeverityText" to INFO
		lr.SetSeverityNumber(plog.SeverityNumberInfo)
		lr.SetSeverityText("INFO")
		attrs := lr.Attributes()
		attrs.EnsureCapacity(totalAuditLogAttributes)

		attrs.PutStr("atype", log.Type)

		if log.Local.IP != nil {
			attrs.PutStr("local.ip", *log.Local.IP)
		}

		if log.Local.Port != nil {
			attrs.PutInt("local.port", int64(*log.Local.Port))
		}

		if log.Local.SystemUser != nil {
			attrs.PutBool("local.isSystemUser", *log.Local.SystemUser)
		}

		if log.Local.UnixSocket != nil {
			attrs.PutStr("local.unix", *log.Local.UnixSocket)
		}

		if log.Remote.IP != nil {
			attrs.PutStr("remote.ip", *log.Remote.IP)
		}

		if log.Remote.Port != nil {
			attrs.PutInt("remote.port", int64(*log.Remote.Port))
		}

		if log.Remote.SystemUser != nil {
			attrs.PutBool("remote.isSystemUser", *log.Remote.SystemUser)
		}

		if log.Remote.UnixSocket != nil {
			attrs.PutStr("remote.unix", *log.Remote.UnixSocket)
		}

		if log.ID != nil {
			attrs.PutStr("uuid.binary", log.ID.Binary)
			attrs.PutStr("uuid.type", log.ID.Type)
		}

		attrs.PutInt("result", int64(log.Result))

		if err = attrs.PutEmptyMap("param").FromRaw(log.Param); err != nil {
			errs = append(errs, err)
		}

		usersSlice := attrs.PutEmptySlice("users")
		usersSlice.EnsureCapacity(len(log.Users))
		for _, user := range log.Users {
			user.Pdata().CopyTo(usersSlice.AppendEmpty().SetEmptyMap())
		}

		rolesSlice := attrs.PutEmptySlice("roles")
		rolesSlice.EnsureCapacity(len(log.Roles))
		for _, roles := range log.Roles {
			roles.Pdata().CopyTo(rolesSlice.AppendEmpty().SetEmptyMap())
		}

		attrs.PutStr("log_name", logName)
	}

	return ld, multierr.Combine(errs...)
}

// mongoEventToLogRecord converts model.LogEntry event to plog.LogRecordSlice and adds the resource attributes.
func mongodbEventToLogData(logger *zap.Logger, logs []model.LogEntry, pc projectContext, hostname, logName string, clusterInfo clusterInfo) plog.Logs {
	ld := plog.NewLogs()
	rl := ld.ResourceLogs().AppendEmpty()
	sl := rl.ScopeLogs().AppendEmpty()

	resourceAttrs := rl.Resource().Attributes()
	resourceAttrs.EnsureCapacity(totalResourceAttributes)

	// Attributes related to the object causing the event.
	resourceAttrs.PutStr("mongodb_atlas.org", pc.orgName)
	resourceAttrs.PutStr("mongodb_atlas.project", pc.Project.Name)
	resourceAttrs.PutStr("mongodb_atlas.cluster", clusterInfo.ClusterName)
	resourceAttrs.PutStr("mongodb_atlas.region.name", clusterInfo.RegionName)
	resourceAttrs.PutStr("mongodb_atlas.provider.name", clusterInfo.ProviderName)
	resourceAttrs.PutStr("mongodb_atlas.host.name", hostname)

	logTsFormat := tsLayout(clusterInfo.MongoDBMajorVersion)

	for _, log := range logs {
		lr := sl.LogRecords().AppendEmpty()

		t, err := time.Parse(logTsFormat, log.Timestamp.Date)
		if err != nil {
			logger.Warn("Time failed to parse correctly", zap.Error(err))
		}

		lr.SetTimestamp(pcommon.NewTimestampFromTime(t))
		lr.SetObservedTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		// Insert Raw Log message into Body of LogRecord
		lr.Body().SetStr(log.Raw)
		// Set the "SeverityNumber" and "SeverityText" if a known type of
		// severity is found.
		if severityNumber, ok := severityMap[log.Severity]; ok {
			lr.SetSeverityNumber(severityNumber)
			lr.SetSeverityText(log.Severity)
		} else {
			logger.Debug("unknown severity type", zap.String("type", log.Severity))
		}
		attrs := lr.Attributes()
		attrs.EnsureCapacity(totalLogAttributes)
		//nolint:errcheck
		attrs.FromRaw(log.Attributes)
		attrs.PutStr("message", log.Message)
		attrs.PutStr("component", log.Component)
		attrs.PutStr("context", log.Context)
		// log ID is not present on MongoDB 4.2 systems
		if clusterInfo.MongoDBMajorVersion != mongoDBMajorVersion4_2 {
			attrs.PutInt("id", log.ID)
		}
		attrs.PutStr("log_name", logName)
	}

	return ld
}

func tsLayout(clusterVersion string) string {
	switch clusterVersion {
	case mongoDBMajorVersion4_2:
		return consoleTimestampLayout
	default:
		return jsonTimestampLayout
	}
}
