// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package syslog // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/parser/syslog"

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"regexp"
	"time"

	sl "github.com/leodido/go-syslog/v4"
	"github.com/leodido/go-syslog/v4/nontransparent"
	"github.com/leodido/go-syslog/v4/octetcounting"
	"github.com/leodido/go-syslog/v4/rfc3164"
	"github.com/leodido/go-syslog/v4/rfc5424"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

var priRegex = regexp.MustCompile(`<\d{1,3}>`)

// rawSyslogMessage is a simple container for raw syslog data that doesn't conform
// to RFC3164 or RFC5424 formats. It implements sl.Message interface.
type rawSyslogMessage struct {
	message string
}

func (r *rawSyslogMessage) Valid() bool                 { return true }
func (r *rawSyslogMessage) FacilityMessage() *string    { return nil }
func (r *rawSyslogMessage) FacilityLevel() *string      { return nil }
func (r *rawSyslogMessage) SeverityMessage() *string    { return nil }
func (r *rawSyslogMessage) SeverityLevel() *string      { return nil }
func (r *rawSyslogMessage) SeverityShortLevel() *string { return nil }
func (r *rawSyslogMessage) ComputeFromPriority(_ uint8) {}
func (r *rawSyslogMessage) GetMessage() string          { return r.message }

// parseFunc a parseFunc determines how the raw input is to be parsed into a syslog message
type parseFunc func(input []byte) (sl.Message, error)

// Parser is an operator that parses syslog.
type Parser struct {
	helper.ParserOperator
	protocol                     string
	location                     *time.Location
	enableOctetCounting          bool
	allowSkipPriHeader           bool
	nonTransparentFramingTrailer *string
	maxOctets                    int
}

func (p *Parser) ProcessBatch(ctx context.Context, entries []*entry.Entry) error {
	return p.TransformerOperator.ProcessBatchWith(ctx, entries, p.Process)
}

// Process will parse an entry field as syslog.
func (p *Parser) Process(ctx context.Context, entry *entry.Entry) error {
	// For 'none' protocol, use raw postprocessing which handles optional timestamp
	if p.protocol == None {
		return p.ProcessWithCallback(ctx, entry, p.parse, postprocessRaw)
	}

	// if pri header is missing and this is an expected behavior then facility and severity values should be skipped.
	if !p.enableOctetCounting && p.allowSkipPriHeader {
		bytes, err := toBytes(entry.Body)
		if err != nil {
			return err
		}

		if p.shouldSkipPriorityValues(bytes) {
			return p.ProcessWithCallback(ctx, entry, p.parse, postprocessWithoutPriHeader)
		}
	}

	return p.ProcessWithCallback(ctx, entry, p.parse, postprocess)
}

// parse will parse a value as syslog.
func (p *Parser) parse(value any) (any, error) {
	bytes, err := toBytes(value)
	if err != nil {
		return nil, err
	}

	pFunc, err := p.buildParseFunc()
	if err != nil {
		return nil, err
	}

	slog, err := pFunc(bytes)
	if err != nil {
		return nil, err
	}

	skipPriHeaderValues := p.shouldSkipPriorityValues(bytes)

	switch message := slog.(type) {
	case *rfc3164.SyslogMessage:
		return p.parseRFC3164(message, skipPriHeaderValues)
	case *rfc5424.SyslogMessage:
		return p.parseRFC5424(message, skipPriHeaderValues)
	case *rawSyslogMessage:
		return p.parseRaw(message)
	default:
		return nil, errors.New("parsed value was not rfc3164 or rfc5424 compliant")
	}
}

func (p *Parser) buildParseFunc() (parseFunc, error) {
	switch p.protocol {
	case RFC3164:
		return func(input []byte) (sl.Message, error) {
			if p.allowSkipPriHeader && !priRegex.Match(input) {
				input = append([]byte("<0>"), input...)
			}
			return rfc3164.NewMachine(rfc3164.WithLocaleTimezone(p.location)).Parse(input)
		}, nil
	case RFC5424:
		switch {
		// Octet Counting Parsing RFC6587
		case p.enableOctetCounting:
			return newOctetCountingParseFunc(p.maxOctets), nil
		// Non-Transparent-Framing Parsing RFC6587
		case p.nonTransparentFramingTrailer != nil && *p.nonTransparentFramingTrailer == LFTrailer:
			return newNonTransparentFramingParseFunc(nontransparent.LF), nil
		case p.nonTransparentFramingTrailer != nil && *p.nonTransparentFramingTrailer == NULTrailer:
			return newNonTransparentFramingParseFunc(nontransparent.NUL), nil
		// Raw RFC5424 parsing
		default:
			return func(input []byte) (sl.Message, error) {
				if p.allowSkipPriHeader && !priRegex.Match(input) {
					input = append([]byte("<0>"), input...)
				}
				return rfc5424.NewMachine().Parse(input)
			}, nil
		}
	case None:
		return func(input []byte) (sl.Message, error) {
			return &rawSyslogMessage{message: string(input)}, nil
		}, nil

	default:
		return nil, fmt.Errorf("invalid protocol %s", p.protocol)
	}
}

func (p *Parser) shouldSkipPriorityValues(value []byte) bool {
	if !p.enableOctetCounting && p.allowSkipPriHeader {
		// check if entry starts with '<'.
		// if not it means that the pre header was missing from the body and hence we should skip it.
		if len(value) > 1 && value[0] != '<' {
			return true
		}
	}
	return false
}

// parseRFC3164 will parse an RFC3164 syslog message.
func (p *Parser) parseRFC3164(syslogMessage *rfc3164.SyslogMessage, skipPriHeaderValues bool) (map[string]any, error) {
	value := map[string]any{
		"timestamp": syslogMessage.Timestamp,
		"hostname":  syslogMessage.Hostname,
		"appname":   syslogMessage.Appname,
		"proc_id":   syslogMessage.ProcID,
		"msg_id":    syslogMessage.MsgID,
		"message":   syslogMessage.Message,
	}

	if !skipPriHeaderValues {
		value["priority"] = syslogMessage.Priority
		value["severity"] = syslogMessage.Severity
		value["facility"] = syslogMessage.Facility
	}

	return p.toSafeMap(value)
}

// parseRFC5424 will parse an RFC5424 syslog message.
func (p *Parser) parseRFC5424(syslogMessage *rfc5424.SyslogMessage, skipPriHeaderValues bool) (map[string]any, error) {
	value := map[string]any{
		"timestamp":       syslogMessage.Timestamp,
		"hostname":        syslogMessage.Hostname,
		"appname":         syslogMessage.Appname,
		"proc_id":         syslogMessage.ProcID,
		"msg_id":          syslogMessage.MsgID,
		"message":         syslogMessage.Message,
		"structured_data": syslogMessage.StructuredData,
		"version":         syslogMessage.Version,
	}

	if !skipPriHeaderValues {
		value["priority"] = syslogMessage.Priority
		value["severity"] = syslogMessage.Severity
		value["facility"] = syslogMessage.Facility
	}

	return p.toSafeMap(value)
}

// parseRaw will parse a raw syslog message that doesn't conform to RFC3164 or RFC5424.
// It attempts best-effort timestamp extraction and returns the original message.
func (p *Parser) parseRaw(syslogMessage *rawSyslogMessage) (map[string]any, error) {
	msg := syslogMessage.GetMessage()
	value := map[string]any{
		"message": msg,
	}

	// Attempt best-effort timestamp extraction
	if ts := p.extractTimestamp(msg); ts != nil {
		value["timestamp"] = *ts
	}

	return value, nil
}

// Common timestamp formats found in syslog messages
var timestampFormats = []string{
	// RFC3339 and variations (RFC5424 style)
	time.RFC3339Nano,
	time.RFC3339,
	"2006-01-02T15:04:05.999999Z07:00",
	"2006-01-02T15:04:05Z07:00",
	"2006-01-02T15:04:05.999999",
	"2006-01-02T15:04:05",
	// RFC3164 BSD style (month day time)
	"Jan _2 15:04:05",
	"Jan 02 15:04:05",
	// Common variations
	"2006-01-02 15:04:05.999999",
	"2006-01-02 15:04:05",
	"2006/01/02 15:04:05",
	"01/02/2006 15:04:05",
	"02/Jan/2006:15:04:05 -0700",
	"02/Jan/2006:15:04:05",
}

// Regex patterns to help extract potential timestamp substrings
var timestampPatterns = []*regexp.Regexp{
	// RFC5424 style: 2015-08-05T21:58:59.693Z or with timezone offset
	regexp.MustCompile(`\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:\d{2})?`),
	// RFC3164 BSD style: Jan  5 15:04:05 or Jan 05 15:04:05
	regexp.MustCompile(`[A-Z][a-z]{2}\s+\d{1,2}\s+\d{2}:\d{2}:\d{2}`),
	// ISO date with space: 2006-01-02 15:04:05
	regexp.MustCompile(`\d{4}-\d{2}-\d{2}\s+\d{2}:\d{2}:\d{2}(?:\.\d+)?`),
	// Common log format: 02/Jan/2006:15:04:05 -0700
	regexp.MustCompile(`\d{2}/[A-Z][a-z]{2}/\d{4}:\d{2}:\d{2}:\d{2}(?:\s+[+-]\d{4})?`),
}

// extractTimestamp attempts to extract a timestamp from the raw message using common formats.
func (p *Parser) extractTimestamp(msg string) *time.Time {
	for _, pattern := range timestampPatterns {
		if match := pattern.FindString(msg); match != "" {
			if ts := p.tryParseTimestamp(match); ts != nil {
				return ts
			}
		}
	}
	return nil
}

// tryParseTimestamp attempts to parse the given string as a timestamp using known formats.
func (p *Parser) tryParseTimestamp(s string) *time.Time {
	for _, format := range timestampFormats {
		if ts, err := time.Parse(format, s); err == nil {
			// For formats without year (like RFC3164), use current year
			if ts.Year() == 0 {
				now := time.Now()
				if p.location != nil {
					now = now.In(p.location)
				}
				ts = ts.AddDate(now.Year(), 0, 0)
			}
			// Apply location if available
			if p.location != nil && ts.Location() == time.UTC {
				ts = time.Date(ts.Year(), ts.Month(), ts.Day(), ts.Hour(), ts.Minute(), ts.Second(), ts.Nanosecond(), p.location)
			}
			return &ts
		}
	}
	return nil
}

// toSafeMap will dereference any pointers on the supplied map.
func (*Parser) toSafeMap(message map[string]any) (map[string]any, error) {
	for key, val := range message {
		switch v := val.(type) {
		case *string:
			if v == nil {
				delete(message, key)
				continue
			}
			message[key] = *v
		case *uint8:
			if v == nil {
				delete(message, key)
				continue
			}
			message[key] = int(*v)
		case uint16:
			message[key] = int(v)
		case *time.Time:
			if v == nil {
				delete(message, key)
				continue
			}
			message[key] = *v
		case *map[string]map[string]string:
			if v == nil {
				delete(message, key)
				continue
			}
			message[key] = convertMap(*v)
		default:
			return nil, fmt.Errorf("key %s has unknown field of type %T", key, v)
		}
	}

	return message, nil
}

// convertMap converts map[string]map[string]string to map[string]any
// which is expected by stanza converter
func convertMap(data map[string]map[string]string) map[string]any {
	ret := map[string]any{}
	for key, value := range data {
		ret[key] = map[string]any{}
		r := ret[key].(map[string]any)

		for k, v := range value {
			r[k] = v
		}
	}

	return ret
}

func toBytes(value any) ([]byte, error) {
	switch v := value.(type) {
	case string:
		return []byte(v), nil
	default:
		return nil, fmt.Errorf("unable to convert type '%T' to bytes", value)
	}
}

var severityMapping = [...]entry.Severity{
	0: entry.Fatal,
	1: entry.Error3,
	2: entry.Error2,
	3: entry.Error,
	4: entry.Warn,
	5: entry.Info2,
	6: entry.Info,
	7: entry.Debug,
}

var severityText = [...]string{
	0: "emerg",
	1: "alert",
	2: "crit",
	3: "err",
	4: "warning",
	5: "notice",
	6: "info",
	7: "debug",
}

var severityField = entry.NewAttributeField("severity")

func cleanupTimestamp(e *entry.Entry) error {
	_, ok := entry.NewAttributeField("timestamp").Delete(e)
	if !ok {
		return errors.New("failed to cleanup timestamp")
	}

	return nil
}

func postprocessWithoutPriHeader(e *entry.Entry) error {
	return cleanupTimestamp(e)
}

// postprocessRaw handles post-processing for raw syslog messages where timestamp may be absent.
func postprocessRaw(e *entry.Entry) error {
	timestampField := entry.NewAttributeField("timestamp")
	// Try to get and set the timestamp if it exists
	if ts, ok := timestampField.Delete(e); ok {
		if timestamp, ok := ts.(time.Time); ok {
			e.Timestamp = timestamp
		}
	}
	return nil
}

func postprocess(e *entry.Entry) error {
	sev, ok := severityField.Delete(e)
	if !ok {
		return errors.New("severity field does not exist")
	}

	sevInt, ok := sev.(int)
	if !ok {
		return errors.New("severity field is not an int")
	}

	if sevInt < 0 || sevInt > 7 {
		return fmt.Errorf("invalid severity '%d'", sevInt)
	}

	e.Severity = severityMapping[sevInt]
	e.SeverityText = severityText[sevInt]

	return cleanupTimestamp(e)
}

func newOctetCountingParseFunc(maxOctets int) parseFunc {
	return func(input []byte) (message sl.Message, err error) {
		listener := func(res *sl.Result) {
			message = res.Message
			err = res.Error
		}

		parserOpts := []sl.ParserOption{
			sl.WithBestEffort(),
			sl.WithListener(listener),
		}

		if maxOctets > 0 {
			parserOpts = append(parserOpts, sl.WithMaxMessageLength(maxOctets))
		}

		parser := octetcounting.NewParser(parserOpts...)
		reader := bytes.NewReader(input)
		parser.Parse(reader)
		return message, err
	}
}

func newNonTransparentFramingParseFunc(trailerType nontransparent.TrailerType) parseFunc {
	return func(input []byte) (message sl.Message, err error) {
		listener := func(res *sl.Result) {
			message = res.Message
			err = res.Error
		}

		parser := nontransparent.NewParser(sl.WithBestEffort(), nontransparent.WithTrailer(trailerType), sl.WithListener(listener))
		reader := bytes.NewReader(input)
		parser.Parse(reader)
		return message, err
	}
}
