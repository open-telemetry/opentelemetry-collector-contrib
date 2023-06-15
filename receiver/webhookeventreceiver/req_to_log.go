// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package webhookeventreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/webhookeventreceiver"

import (
	"bufio"
	"net/url"

	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/receiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/webhookeventreceiver/internal/metadata"
)

func reqToLog(sc *bufio.Scanner,
	query url.Values,
	_ *Config,
	settings receiver.CreateSettings) (plog.Logs, int) {
	log := plog.NewLogs()
	resourceLog := log.ResourceLogs().AppendEmpty()
	appendMetadata(resourceLog, query)
	scopeLog := resourceLog.ScopeLogs().AppendEmpty()

	scopeLog.Scope().SetName(scopeLogName)
	scopeLog.Scope().SetVersion(settings.BuildInfo.Version)
	scopeLog.Scope().Attributes().PutStr("source", settings.ID.String())
	scopeLog.Scope().Attributes().PutStr("receiver", metadata.Type)

	for sc.Scan() {
		logRecord := scopeLog.LogRecords().AppendEmpty()
		line := sc.Text()
		logRecord.Body().SetStr(line)
	}

	return log, scopeLog.LogRecords().Len()
}

// append query parameters and webhook source as resource attributes
func appendMetadata(resourceLog plog.ResourceLogs, query url.Values) {
	for k := range query {
		if query.Get(k) != "" {
			resourceLog.Resource().Attributes().PutStr(k, query.Get(k))
		}
	}

}
