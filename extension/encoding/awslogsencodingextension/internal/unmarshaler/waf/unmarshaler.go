// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package waf // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension/internal/unmarshaler/waf"

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	gojson "github.com/goccy/go-json"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	conventions "go.opentelemetry.io/otel/semconv/v1.38.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension/internal/constants"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension/internal/metadata"
)

var _ encoding.StreamDecoder[plog.Logs] = (*wafLogUnmarshaler)(nil)

type wafLogUnmarshaler struct {
	buildInfo component.BuildInfo
	reader    io.Reader
	scanner   *bufio.Scanner
	opts      encoding.StreamDecoderOptions
	offset    encoding.StreamOffset

	// Flush tracking
	lastFlushTime time.Time
	bytesRead     int64
	itemsRead     int64

	// State for resource attributes (webACLID is consistent across batch)
	webACLID string
}

type wafLogUnmarshalerFactory struct {
	buildInfo component.BuildInfo
}

func NewWAFLogUnmarshalerFactory(buildInfo component.BuildInfo) func(reader io.Reader, opts encoding.StreamDecoderOptions) (encoding.StreamDecoder[plog.Logs], error) {
	return func(reader io.Reader, opts encoding.StreamDecoderOptions) (encoding.StreamDecoder[plog.Logs], error) {
		scanner := bufio.NewScanner(reader)
		// Skip to initial offset
		for i := encoding.StreamOffset(0); i < opts.InitialOffset; i++ {
			if !scanner.Scan() {
				break
			}
		}
		return &wafLogUnmarshaler{
			buildInfo:     buildInfo,
			reader:        reader,
			scanner:       scanner,
			opts:          opts,
			offset:        opts.InitialOffset,
			lastFlushTime: time.Now(),
		}, nil
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

func (w *wafLogUnmarshaler) Decode(ctx context.Context, to plog.Logs) error {
	logs := plog.NewLogs()
	resourceLogs := logs.ResourceLogs().AppendEmpty()
	resourceLogs.Resource().Attributes().PutStr(
		string(conventions.CloudProviderKey),
		conventions.CloudProviderAWS.Value.AsString(),
	)

	scopeLogs := resourceLogs.ScopeLogs().AppendEmpty()
	scopeLogs.Scope().SetName(metadata.ScopeName)
	scopeLogs.Scope().SetVersion(w.buildInfo.Version)
	scopeLogs.Scope().Attributes().PutStr(constants.FormatIdentificationTag, "aws."+constants.FormatWAFLog)

	batchWebACLID := w.webACLID
	hasRecords := false

	for {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Check flush conditions
		shouldFlush := false
		if w.opts.FlushItems > 0 && w.itemsRead >= w.opts.FlushItems {
			shouldFlush = true
		}
		if w.opts.FlushBytes > 0 && w.bytesRead >= w.opts.FlushBytes {
			shouldFlush = true
		}
		if w.opts.FlushTimeout > 0 && time.Since(w.lastFlushTime) >= w.opts.FlushTimeout {
			shouldFlush = true
		}

		// If we have records and should flush, break
		if hasRecords && shouldFlush {
			break
		}

		// Try to read the next line
		if !w.scanner.Scan() {
			if w.scanner.Err() != nil {
				return fmt.Errorf("failed to scan line: %w", w.scanner.Err())
			}
			// EOF
			if !hasRecords {
				return io.EOF
			}
			break
		}

		logLine := w.scanner.Bytes()
		lineSize := int64(len(logLine))

		var log wafLog
		if err := gojson.Unmarshal(logLine, &log); err != nil {
			return fmt.Errorf("failed to unmarshal WAF log: %w", err)
		}
		if log.WebACLID == "" {
			return errors.New("invalid WAF log: empty webaclId field")
		}
		if batchWebACLID != "" && log.WebACLID != batchWebACLID {
			return fmt.Errorf(
				"unexpected: new webaclId %q is different than previous one %q",
				log.WebACLID,
				batchWebACLID,
			)
		}
		batchWebACLID = log.WebACLID

		record := scopeLogs.LogRecords().AppendEmpty()
		if err := w.addWAFLog(log, record); err != nil {
			return err
		}

		hasRecords = true
		w.itemsRead++
		w.bytesRead += lineSize
	}

	if !hasRecords {
		return io.EOF
	}

	if err := setResourceAttributes(resourceLogs, batchWebACLID); err != nil {
		return fmt.Errorf("failed to get resource attributes: %w", err)
	}

	// Update state
	w.webACLID = batchWebACLID

	// Copy to output
	logs.CopyTo(to)

	// Update offset (count log records)
	recordCount := int64(to.LogRecordCount())
	w.offset += encoding.StreamOffset(recordCount)

	// Reset flush tracking
	w.lastFlushTime = time.Now()
	w.bytesRead = 0
	w.itemsRead = 0

	return nil
}

func (w *wafLogUnmarshaler) Offset() encoding.StreamOffset {
	return w.offset
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

func (*wafLogUnmarshaler) addWAFLog(log wafLog, record plog.LogRecord) error {
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

	putStr := func(name, value string) {
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
