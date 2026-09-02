// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package loghouseprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/loghouseprocessor"
import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	jsoniter "github.com/json-iterator/go"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"
)

type logProcessor struct {
	logger *zap.Logger

	nextConsumer consumer.Logs

	nonRoutedLogRecordsCounter metric.Int64Counter
}

func (p *logProcessor) Shutdown(context.Context) error {
	return nil
}

func (p *logProcessor) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func (p *logProcessor) Start(_ context.Context, _ component.Host) error {
	return nil
}

func (p *logProcessor) ConsumeLogs(ctx context.Context, l plog.Logs) error {
	// This processes logs one by one with no context between, so we couldn't group
	// stacktrace log lines together at this level.
	for i := 0; i < l.ResourceLogs().Len(); i++ {
		rlogs := l.ResourceLogs().At(i)
		for j := 0; j < rlogs.ScopeLogs().Len(); j++ {
			scopeLogs := rlogs.ScopeLogs().At(j)
			for k := 0; k < scopeLogs.LogRecords().Len(); k++ {
				logLine := scopeLogs.LogRecords().At(k)
				err := processOneLogLine(&logLine)
				if err != nil {
					p.logger.Debug("failed to parse log line", zap.Error(err))
				}
				// This does have a "last line wins" if we have somehow set the same key with different values.
				// We are just going to ignore this case for now though.
				promoteResourceAttrs(&logLine, &rlogs)
			}
		}
	}
	return p.nextConsumer.ConsumeLogs(ctx, l)
}

// K8s logs begin with "<timestamp> <stdout/err> F"
func trimK8sLogPreamble(s string) (string, bool) {
	if len(s) == 0 {
		return s, false
	}
	lastChar := s[len(s)-1:]
	if lastChar != "}" && !(lastChar == "\n" && len(s) > 1 && s[len(s)-2:] == "}\n") {
		// This isn't a json string as the last char is not a closing brace
		return s, false
	}

	index := strings.Index(s, "{")
	if index == -1 {
		// If there is no curly brace, return the original string
		return s, false
	}
	// Trim everything from the start until the first curly brace
	return s[index:], true
}

func extractJSONAttrs(body string, l *plog.LogRecord) error {
	j := jsoniter.ConfigFastest
	var parsedValue map[string]any
	err := j.UnmarshalFromString(body, &parsedValue)
	if err != nil {
		return fmt.Errorf("fail to unmarshal json | %w", err)
	}
	result := pcommon.NewMap()
	err = result.FromRaw(parsedValue)
	if err != nil {
		return fmt.Errorf("fail to read attrs | %w", err)
	}
	result.CopyTo(l.Attributes())
	return nil
}

func extractBody(l *plog.LogRecord) bool {
	copyBody := func(msgKey string) bool {
		message, ok := l.Attributes().Get(msgKey)
		if !ok {
			return false
		}
		l.Body().SetStr(message.Str())
		l.Attributes().Remove(msgKey)
		return true
	}

	return copyBody("msg") || copyBody("message") || copyBody("body")
}

func processOneLogLine(l *plog.LogRecord) error {
	logBody := l.Body().Str()
	jsonString, ok := trimK8sLogPreamble(logBody)
	if !ok {
		return processPlaintextLog(l)
	}
	err := extractJSONAttrs(jsonString, l)
	if err != nil {
		return err
	}
	processJSONLog(l)
	return nil
}

func parseSeverity(l *plog.LogRecord) error {
	logLevel, ok := l.Attributes().Get("level")
	if !ok {
		return fmt.Errorf("no level specified")
	}
	updateSeverity(strings.ToUpper(logLevel.Str()), l)
	return nil
}

func processJSONLog(l *plog.LogRecord) {
	_ = parseSeverity(l)
	extractBody(l)
	promoteTraceAndSpan(l)
	isCH := parseCHTimestamp(l)
	if isCH {
		parseCHSeverity(l)
	}
}

func promoteResourceAttrs(l *plog.LogRecord, rlogs *plog.ResourceLogs) {
	attributes, ok := l.Attributes().Get("resource")
	if !ok {
		return
	}

	if attributes.Type() != pcommon.ValueTypeMap {
		return
	}

	origAttr := rlogs.Resource().Attributes()
	merged := MergeRawMaps(origAttr.AsRaw(), attributes.Map().AsRaw())
	_ = origAttr.FromRaw(merged)
}

// MergeRawMaps merges n maps with a later map's keys overriding earlier maps. (copied to avoid dep hell)
func MergeRawMaps(maps ...map[string]any) map[string]any {
	ret := map[string]any{}

	for _, m := range maps {
		for k, v := range m {
			ret[k] = v
		}
	}

	return ret
}

// Both funcs copied from: https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/pkg/ottl/contexts/internal/ids.go#L25

func ParseSpanID(spanIDStr string) (pcommon.SpanID, error) {
	var id pcommon.SpanID
	if hex.DecodedLen(len(spanIDStr)) != len(id) {
		return pcommon.SpanID{}, errors.New("span ids must be 16 hex characters")
	}
	_, err := hex.Decode(id[:], []byte(spanIDStr))
	if err != nil {
		return pcommon.SpanID{}, err
	}
	return id, nil
}

func ParseTraceID(traceIDStr string) (pcommon.TraceID, error) {
	var id pcommon.TraceID
	if hex.DecodedLen(len(traceIDStr)) != len(id) {
		return pcommon.TraceID{}, errors.New("trace ids must be 32 hex characters")
	}
	_, err := hex.Decode(id[:], []byte(traceIDStr))
	if err != nil {
		return pcommon.TraceID{}, err
	}
	return id, nil
}

func promoteTraceAndSpan(l *plog.LogRecord) {
	var (
		traceID pcommon.TraceID
		spanID  pcommon.SpanID
	)
	if tid, ok := l.Attributes().Get("traceId"); ok {
		parsed, err := ParseTraceID(tid.Str())
		if err != nil {
			return
		}
		traceID = parsed
	}
	if sid, ok := l.Attributes().Get("spanId"); ok {
		parsed, err := ParseSpanID(sid.Str())
		if err != nil {
			return
		}
		spanID = parsed
	}
	// No point having one or the other, we must always do both
	if !traceID.IsEmpty() && !spanID.IsEmpty() {
		l.SetTraceID(traceID)
		l.SetSpanID(spanID)
	}
}

var (
	k8sTimestampRe = regexp.MustCompile(`\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}.\d*Z`)
	chTimestampRe  = regexp.MustCompile(`\d{4}.\d{2}.\d{2} \d{2}:\d{2}:\d{2}.\d*`)
	chTimeFormat   = "2006.01.02 15:04:05.999999999"
	logLevelRe     = regexp.MustCompile(`(?i)<(trace|debug|info|information|warn|warning|error|fatal)>`)
	FATAL          = "FATAL"
	ERROR          = "ERROR"
	WARN           = "WARN"
	INFO           = "INFO"
	DEBUG          = "DEBUG"
	TRACE          = "TRACE"
)

func parsePlaintextSeverity(l *plog.LogRecord) {
	matches := logLevelRe.FindStringSubmatch(l.Body().Str())
	if len(matches) < 2 {
		return
	}
	level := strings.ToUpper(matches[1])
	switch level {
	case "INFORMATION":
		level = INFO
	case "WARNING":
		level = WARN
	}
	updateSeverity(level, l)
}

func parsePlaintextTimestamp(l *plog.LogRecord) error {
	// Priority - try and match a ClickHouse timestamp, if not - then fall back to a Kubenetes timestamp.
	var chErr, k8sErr error
	var t time.Time
	chMatch := chTimestampRe.FindString(l.Body().Str())
	t, chErr = time.Parse(chTimeFormat, chMatch)
	if chErr != nil {
		match := k8sTimestampRe.FindString(l.Body().Str())
		t, k8sErr = time.Parse(time.RFC3339Nano, match)
		if k8sErr != nil {
			return fmt.Errorf("fail to parse CH time and K8s time, CH err: %w, k8s err: %w", chErr, k8sErr)
		}
	}
	l.SetTimestamp(pcommon.NewTimestampFromTime(t))
	return nil
}

func processPlaintextLog(l *plog.LogRecord) error {
	parsePlaintextSeverity(l)
	err := parsePlaintextTimestamp(l)
	if err != nil {
		return err
	}
	return nil
}

func updateSeverity(sev string, l *plog.LogRecord) {
	l.SetSeverityText(strings.ToUpper(sev))
	switch sev {
	case TRACE:
		l.SetSeverityNumber(plog.SeverityNumberTrace)
	case DEBUG:
		l.SetSeverityNumber(plog.SeverityNumberDebug)
	case INFO:
		l.SetSeverityNumber(plog.SeverityNumberInfo)
	case WARN:
		l.SetSeverityNumber(plog.SeverityNumberWarn)
	case ERROR:
		l.SetSeverityNumber(plog.SeverityNumberError)
	case FATAL:
		l.SetSeverityNumber(plog.SeverityNumberFatal)
	}
}

func parseCHNumericSeverity(l *plog.LogRecord) bool {
	sevText, ok := l.Attributes().Get("level")
	if !ok {
		return false
	}
	switch sevText.Str() {
	case "0":
		updateSeverity(FATAL, l)
	case "1":
		updateSeverity(FATAL, l)
	case "2":
		updateSeverity(FATAL, l)
	case "3":
		updateSeverity(ERROR, l)
	case "4":
		updateSeverity(WARN, l)
	case "5":
		updateSeverity(INFO, l)
	case "6":
		updateSeverity(INFO, l)
	case "7":
		updateSeverity(DEBUG, l)
	case "8":
		updateSeverity(TRACE, l)
	}
	l.SetSeverityText(strings.ToUpper(l.SeverityNumber().String()))
	return true
}

func parseCHSeverity(l *plog.LogRecord) bool {
	levelVal, ok := l.Attributes().Get("level")
	if !ok {
		return false
	}

	if unicode.IsDigit([]rune(levelVal.Str())[0]) {
		return parseCHNumericSeverity(l)
	}

	level := strings.ToUpper(levelVal.Str())
	switch level {
	case "INFORMATION":
		level = INFO
	case "WARNING":
		level = WARN
	}
	updateSeverity(level, l)
	return true
}

func parseCHTimestamp(l *plog.LogRecord) bool {
	dateTime, ok := l.Attributes().Get("date_time")
	if !ok {
		return false
	}
	components := strings.Split(dateTime.Str(), ".")
	s, err := strconv.Atoi(components[0])
	if err != nil {
		fmt.Printf("Failed A: %v\n", err)
		return false
	}
	us, err := strconv.Atoi(components[1])
	if err != nil {
		fmt.Printf("Failed B: %v\n", err)
		return false
	}
	ts := time.Unix(int64(s), int64(us)*int64(1e3))
	l.SetTimestamp(pcommon.NewTimestampFromTime(ts))
	return true
}
