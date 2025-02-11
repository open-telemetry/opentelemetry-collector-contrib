// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package webhookeventreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/webhookeventreceiver"

import (
	"bufio"
	"net/http"
	"net/textproto"
	"net/url"
	"strings"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/receiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/webhookeventreceiver/internal/metadata"
)

func reqToLog(sc *bufio.Scanner,
	headers http.Header,
	query url.Values,
	cfg *Config,
	settings receiver.Settings,
) (plog.Logs, int) {
	if cfg.SplitLogsAtNewLine {
		sc.Split(bufio.ScanLines)
	} else {
		// we simply dont split the data passed into scan (i.e. scan the whole thing)
		// the downside to this approach is that only 1 log per request can be handled.
		// NOTE: logs will contain these newline characters which could have formatting
		// consequences downstream.
		split := func(data []byte, atEOF bool) (advance int, token []byte, err error) {
			if !atEOF {
				return 0, nil, nil
			}
			return 0, data, bufio.ErrFinalToken
		}
		sc.Split(split)
	}

	log := plog.NewLogs()
	resourceLog := log.ResourceLogs().AppendEmpty()
	appendMetadata(resourceLog, query)
	scopeLog := resourceLog.ScopeLogs().AppendEmpty()

	scopeLog.Scope().SetName(scopeLogName)
	scopeLog.Scope().SetVersion(settings.BuildInfo.Version)
	scopeLog.Scope().Attributes().PutStr("source", settings.ID.String())
	scopeLog.Scope().Attributes().PutStr("receiver", metadata.Type.String())
	if cfg.ConvertHeadersToAttributes {
		appendHeaders(cfg, scopeLog, headers)
	}

	for sc.Scan() {
		logRecord := scopeLog.LogRecords().AppendEmpty()
		logRecord.SetObservedTimestamp(pcommon.NewTimestampFromTime(time.Now()))
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

// append headers as attributes
func appendHeaders(config *Config, scopeLog plog.ScopeLogs, headers http.Header) {
	for k := range headers {
		// Skip the required header used for authentication
		if k == textproto.CanonicalMIMEHeaderKey(config.RequiredHeader.Key) {
			continue
		}
		scopeLog.Scope().Attributes().PutStr(headerAttributeKey(k), strings.Join(headers.Values(k), ";"))
	}
}

// convert given header to snake_case and add "header" as a namespace prefix
func headerAttributeKey(header string) string {
	snakeCaseHeader := strings.ReplaceAll(header, "-", "_")
	snakeCaseHeader = strings.ToLower(snakeCaseHeader)
	return "header." + snakeCaseHeader
}
