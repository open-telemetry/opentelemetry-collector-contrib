// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package webhookeventreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/webhookeventreceiver"

import (
	"bufio"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/webhookeventreceiver/internal/metadata"
)

const (
	headerNamespace = "header"
)

func (er *eventReceiver) reqToLog(sc *bufio.Scanner,
	headers http.Header,
	query url.Values,
) (plog.Logs, int) {
	if er.cfg.SplitLogsAtNewLine {
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
	scopeLog.Scope().SetVersion(er.settings.BuildInfo.Version)
	scopeLog.Scope().Attributes().PutStr("source", er.settings.ID.String())
	scopeLog.Scope().Attributes().PutStr("receiver", metadata.Type.String())

	for sc.Scan() {
		logRecord := scopeLog.LogRecords().AppendEmpty()
		logRecord.SetObservedTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		line := sc.Text()
		logRecord.Body().SetStr(line)
		if er.includeHeadersRegex != nil {
			appendHeaders(headers, logRecord, er.includeHeadersRegex)
		}
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

// append headers as logRecord attributes if they match supplied regex
func appendHeaders(h http.Header, l plog.LogRecord, r *regexp.Regexp) {
	for k := range h {
		// Skip the required header used for authentication
		if r.MatchString(k) {
			slice := l.Attributes().PutEmptySlice(headerAttributeKey(k))
			for _, v := range h.Values(k) {
				slice.AppendEmpty().SetStr(v)
			}
		}
	}
}

// prepend the header key with the "header." namespace
func headerAttributeKey(header string) string {
	return strings.Join([]string{headerNamespace, header}, ".")
}
