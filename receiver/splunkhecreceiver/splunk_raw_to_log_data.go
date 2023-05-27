// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package splunkhecreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/splunkhecreceiver"

import (
	"bufio"
	"net/url"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/splunk"
)

const (
	// splunk metadata
	index      = "index"
	source     = "source"
	sourcetype = "sourcetype"
	host       = "host"
)

// SplunkHecRawToLogData transforms raw splunk event into log
func splunkHecRawToLogData(sc *bufio.Scanner, query url.Values, resourceCustomizer func(pcommon.Resource), config *splunk.HecToOtelAttrs) (plog.Logs, int) {
	ld := plog.NewLogs()
	rl := ld.ResourceLogs().AppendEmpty()
	appendSplunkMetadata(rl, config, query.Get(host), query.Get(source), query.Get(sourcetype), query.Get(index))
	if resourceCustomizer != nil {
		resourceCustomizer(rl.Resource())
	}

	sl := rl.ScopeLogs().AppendEmpty()
	for sc.Scan() {
		logRecord := sl.LogRecords().AppendEmpty()
		logLine := sc.Text()
		logRecord.Body().SetStr(logLine)
	}

	return ld, sl.LogRecords().Len()
}

func appendSplunkMetadata(rl plog.ResourceLogs, attrs *splunk.HecToOtelAttrs, host, source, sourceType, index string) {
	if host != "" {
		rl.Resource().Attributes().PutStr(attrs.Host, host)
	}
	if source != "" {
		rl.Resource().Attributes().PutStr(attrs.Source, source)
	}
	if sourceType != "" {
		rl.Resource().Attributes().PutStr(attrs.SourceType, sourceType)
	}
	if index != "" {
		rl.Resource().Attributes().PutStr(attrs.Index, index)
	}
}
