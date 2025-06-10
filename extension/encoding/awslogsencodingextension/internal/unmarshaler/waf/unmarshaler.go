// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package waf // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension/internal/unmarshaler/waf"

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"

	gojson "github.com/goccy/go-json"
	"github.com/klauspost/compress/gzip"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	conventions "go.opentelemetry.io/otel/semconv/v1.28.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension/internal/metadata"
)

type wafLogUnmarshaler struct {
	buildInfo component.BuildInfo
	gzipPool  sync.Pool
}

var _ plog.Unmarshaler = (*wafLogUnmarshaler)(nil)

func NewWAFLogUnmarshaler(buildInfo component.BuildInfo) plog.Unmarshaler {
	return &wafLogUnmarshaler{
		buildInfo: buildInfo,
	}
}

// See log fields: https://docs.aws.amazon.com/waf/latest/developerguide/logging-fields.html.
type wafLog struct {
	Timestamp           int64  `json:"timestamp"`
	WebACLID            string `json:"webaclId"`
	TerminatingRuleID   string `json:"terminatingRuleId"`
	TerminatingRuleType string `json:"terminatingRuleType"`
	Action              string `json:"action"`
	HTTPSourceName      string `json:"httpSourceName"`
	HTTPSourceID        string `json:"httpSourceId"`
	HTTPRequest         struct {
		ClientIP string `json:"clientIp"`
		Country  string `json:"country"`
		Headers  []struct {
			Name  string `json:"name"`
			Value string `json:"value"`
		} `json:"headers"`
		URI         string `json:"uri"`
		Args        string `json:"args"`
		HTTPVersion string `json:"httpVersion"`
		HTTPMethod  string `json:"httpMethod"`
		RequestID   string `json:"requestID"`
		Fragment    string `json:"fragment"`
		Scheme      string `json:"scheme"`
		Host        string `json:"host"`
	} `json:"httpRequest"`
	ResponseCodeSent *int64 `json:"responseCodeSent"`
	Ja3Fingerprint   string `json:"ja3Fingerprint"`
	Ja4Fingerprint   string `json:"ja4Fingerprint"`
}

func (w *wafLogUnmarshaler) UnmarshalLogs(content []byte) (plog.Logs, error) {
	var errGzipReader error
	gzipReader, ok := w.gzipPool.Get().(*gzip.Reader)
	if !ok {
		gzipReader, errGzipReader = gzip.NewReader(bytes.NewReader(content))
	} else {
		errGzipReader = gzipReader.Reset(bytes.NewReader(content))
	}
	if errGzipReader != nil {
		if gzipReader != nil {
			w.gzipPool.Put(gzipReader)
		}
		return plog.Logs{}, fmt.Errorf("failed to decompress content: %w", errGzipReader)
	}
	defer func() {
		_ = gzipReader.Close()
		w.gzipPool.Put(gzipReader)
	}()

	return w.unmarshalWAFLogs(gzipReader)
}

func (w *wafLogUnmarshaler) unmarshalWAFLogs(reader io.Reader) (plog.Logs, error) {
	logs := plog.NewLogs()

	resourceLogs := logs.ResourceLogs().AppendEmpty()
	resourceLogs.Resource().Attributes().PutStr(
		string(conventions.CloudProviderKey),
		conventions.CloudProviderAWS.Value.AsString(),
	)

	scopeLogs := resourceLogs.ScopeLogs().AppendEmpty()
	scopeLogs.Scope().SetName(metadata.ScopeName)
	scopeLogs.Scope().SetVersion(w.buildInfo.Version)

	scanner := bufio.NewScanner(reader)
	webACLID := ""
	for scanner.Scan() {
		logLine := scanner.Bytes()

		var log wafLog
		if err := gojson.Unmarshal(logLine, &log); err != nil {
			return plog.Logs{}, fmt.Errorf("failed to unmarshal WAF log: %w", err)
		}
		if log.WebACLID == "" {
			return plog.Logs{}, errors.New("invalid WAF log: empty webaclId field")
		}
		if webACLID != "" && log.WebACLID != webACLID {
			return plog.Logs{}, fmt.Errorf(
				"unexpected: new webaclId %q is different than previous one %q",
				webACLID,
				log.WebACLID,
			)
		}
		webACLID = log.WebACLID

		record := scopeLogs.LogRecords().AppendEmpty()
		if err := w.addWAFLog(log, record); err != nil {
			return plog.Logs{}, err
		}
	}

	if err := setResourceAttributes(resourceLogs, webACLID); err != nil {
		return plog.Logs{}, fmt.Errorf("failed to get resource attributes: %w", err)
	}

	return logs, nil
}

// setResourceAttributes based on the web ACL ID
func setResourceAttributes(resourceLogs plog.ResourceLogs, webACLID string) error {
	expectedFormat := "arn:aws:wafv2:<region>:<account>:<scope>/webacl/<name>/<id>"
	value, remaining, _ := strings.Cut(webACLID, "arn:aws:wafv2:")
	if value != "" {
		return fmt.Errorf("webaclId %q does not have expected prefix %q", webACLID, "arn:aws:wafv2:")
	}
	if remaining == "" {
		return fmt.Errorf("webaclId %q contains no data after expected prefix %q", webACLID, "arn:aws:wafv2:")
	}

	value, remaining, _ = strings.Cut(remaining, ":")
	if value == "" {
		return fmt.Errorf("could not find region in webaclId %q", webACLID)
	}
	resourceLogs.Resource().Attributes().PutStr(string(conventions.CloudRegionKey), value)

	value, remaining, _ = strings.Cut(remaining, ":")
	if value == "" {
		return fmt.Errorf("could not find account in webaclId %q", webACLID)
	}
	resourceLogs.Resource().Attributes().PutStr(string(conventions.CloudAccountIDKey), value)

	if remaining == "" {
		return fmt.Errorf("webaclId %q does not have expected format %q", webACLID, expectedFormat)
	}

	resourceLogs.Resource().Attributes().PutStr(string(conventions.CloudResourceIDKey), webACLID)
	return nil
}

func (w *wafLogUnmarshaler) addWAFLog(log wafLog, record plog.LogRecord) error {
	// timestamp is in milliseconds, so we need to convert it to ns first
	nanos := log.Timestamp * 1_000_000
	ts := pcommon.Timestamp(nanos)
	record.SetTimestamp(ts)

	if log.HTTPRequest.HTTPVersion != "" {
		_, version, found := strings.Cut(log.HTTPRequest.HTTPVersion, "HTTP/")
		if !found || version == "" {
			return fmt.Errorf(
				`httpRequest.httpVersion %q does not have expected format "HTTP/<version"`,
				log.HTTPRequest.HTTPVersion,
			)
		}
		record.Attributes().PutStr(string(conventions.NetworkProtocolNameKey), "http")
		record.Attributes().PutStr(string(conventions.NetworkProtocolVersionKey), version)
	}

	if log.ResponseCodeSent != nil {
		record.Attributes().PutInt(string(conventions.HTTPResponseStatusCodeKey), *log.ResponseCodeSent)
	}

	putStr := func(name string, value string) {
		if value != "" {
			record.Attributes().PutStr(name, value)
		}
	}

	putStr("aws.waf.terminating_rule.type", log.TerminatingRuleType)
	putStr("aws.waf.terminating_rule.id", log.TerminatingRuleID)
	putStr("aws.waf.action", log.Action)
	putStr("aws.waf.source.id", log.HTTPSourceID)
	putStr("aws.waf.source.name", log.HTTPSourceName)

	for _, header := range log.HTTPRequest.Headers {
		putStr("http.request.header."+header.Name, header.Value)
	}

	putStr(string(conventions.ClientAddressKey), log.HTTPRequest.ClientIP)
	putStr(string(conventions.ServerAddressKey), log.HTTPRequest.Host)
	putStr(string(conventions.URLPathKey), log.HTTPRequest.URI)
	putStr(string(conventions.URLQueryKey), log.HTTPRequest.Args)
	putStr(string(conventions.HTTPRequestMethodKey), log.HTTPRequest.HTTPMethod)
	putStr(string(conventions.AWSRequestIDKey), log.HTTPRequest.RequestID)
	putStr(string(conventions.URLFragmentKey), log.HTTPRequest.Fragment)
	putStr(string(conventions.URLSchemeKey), log.HTTPRequest.Scheme)
	putStr("geo.country.iso_code", log.HTTPRequest.Country)

	putStr(string(conventions.TLSClientJa3Key), log.Ja3Fingerprint)
	putStr("tls.client.ja4", log.Ja4Fingerprint)

	return nil
}
