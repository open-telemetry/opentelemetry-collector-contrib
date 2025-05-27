package waf

import (
	"bufio"
	"bytes"
	"fmt"
	gojson "github.com/goccy/go-json"
	"github.com/klauspost/compress/gzip"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension/internal/metadata"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	conventions "go.opentelemetry.io/otel/semconv/v1.28.0"
	"io"
	"sync"
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
		URI         string `json:"URI"`
		Args        string `json:"args"`
		HTTPVersion string `json:"httpVersion"`
		HTTPMethod  string `json:"httpMethod"`
		RequestId   string `json:"requestId"`
		Fragment    string `json:"fragment"`
		Scheme      string `json:"scheme"`
		Host        string `json:"host"`
	} `json:"httpRequest"`
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
	resourceLogs.Resource().Attributes().PutStr(string(conventions.CloudProviderKey), conventions.CloudProviderAWS.Value.AsString())

	scopeLogs := resourceLogs.ScopeLogs().AppendEmpty()
	scopeLogs.Scope().SetName(metadata.ScopeName)
	scopeLogs.Scope().SetVersion(w.buildInfo.Version)

	scanner := bufio.NewScanner(reader)

	if scanner.Scan() {
		logLine := scanner.Bytes()
		var log wafLog
		if err := gojson.Unmarshal(logLine, &log); err != nil {
			return plog.Logs{}, fmt.Errorf("failed to unmarshal WAF log: %w", err)
		}

		record := scopeLogs.LogRecords().AppendEmpty()
		if err := w.addWAFLog(log, record); err != nil {
			return plog.Logs{}, err
		}
	}

	return logs, nil
}

func (w *wafLogUnmarshaler) addWAFLog(log wafLog, record plog.LogRecord) error {
	// timestamp is in milliseconds, so we need to convert it to ns first
	nanos := log.Timestamp * 1_000_000
	ts := pcommon.Timestamp(nanos)
	record.SetTimestamp(ts)

	record.Attributes().PutStr(string(conventions.CloudResourceIDKey), log.WebACLID)

	record.Attributes().PutStr("aws.waf.action", log.Action)
	record.Attributes().PutStr("aws.waf.terminating_rule.type", log.TerminatingRuleType)
	record.Attributes().PutStr("aws.waf.terminating_rule.id", log.TerminatingRuleID)
	record.Attributes().PutStr("aws.waf.source.id", log.HTTPSourceID)
	record.Attributes().PutStr("aws.waf.source.name", log.HTTPSourceName)

	for _, header := range log.HTTPRequest.Headers {
		record.Attributes().PutStr("http.request.header."+header.Name, header.Value)
	}
	record.Attributes().PutStr(string(conventions.ClientAddressKey), log.HTTPRequest.ClientIP)
	record.Attributes().PutStr(string("geo.country.iso_code"), log.HTTPRequest.Country)
	record.Attributes().PutStr(string(conventions.URLPathKey), log.HTTPRequest.URI)
	record.Attributes().PutStr(string(conventions.URLQueryKey), log.HTTPRequest.Args)
	record.Attributes().PutStr(string(conventions.ClientAddressKey), log.HTTPRequest.HTTPVersion)
	record.Attributes().PutStr(string(conventions.HTTPRequestMethodKey), log.HTTPRequest.HTTPMethod)
	record.Attributes().PutStr(string(conventions.AWSRequestIDKey), log.HTTPRequest.RequestId)
	record.Attributes().PutStr(string(conventions.URLFragmentKey), log.HTTPRequest.Fragment)
	record.Attributes().PutStr(string(conventions.URLSchemeKey), log.HTTPRequest.Scheme)
	// record.Attributes().PutStr(string(conventions.ClientAddressKey), log.HTTPRequest.Host)

	return nil
}
