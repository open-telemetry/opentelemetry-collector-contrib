// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cloudfoundryreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/cloudfoundryreceiver"

import (
	"encoding/hex"
	"fmt"
	"strings"
	"time"
	"unicode"

	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

const (
	attributeNamePrefix        = "org.cloudfoundry."
	envelopeSourceTypeKey      = "source_type"
	envelopeSourceTypeValueRTR = "RTR"
	logLineRTRZipkinKey        = "b3"
	logLineRTRW3CKey           = "traceparent"
	traceW3CVersion            = "00"
)

func convertEnvelopeToMetrics(envelope *loggregator_v2.Envelope, metricSlice pmetric.MetricSlice, startTime time.Time) {
	namePrefix := envelope.Tags["origin"] + "."

	switch message := envelope.Message.(type) {
	case *loggregator_v2.Envelope_Counter:
		metric := metricSlice.AppendEmpty()
		metric.SetName(namePrefix + message.Counter.GetName())
		dataPoint := metric.SetEmptySum().DataPoints().AppendEmpty()
		dataPoint.SetDoubleValue(float64(message.Counter.GetTotal()))
		dataPoint.SetTimestamp(pcommon.Timestamp(envelope.GetTimestamp()))
		dataPoint.SetStartTimestamp(pcommon.NewTimestampFromTime(startTime))
		copyEnvelopeAttributes(dataPoint.Attributes(), envelope)
	case *loggregator_v2.Envelope_Gauge:
		for name, value := range message.Gauge.GetMetrics() {
			metric := metricSlice.AppendEmpty()
			metric.SetName(namePrefix + name)
			dataPoint := metric.SetEmptyGauge().DataPoints().AppendEmpty()
			dataPoint.SetDoubleValue(value.Value)
			dataPoint.SetTimestamp(pcommon.Timestamp(envelope.GetTimestamp()))
			dataPoint.SetStartTimestamp(pcommon.NewTimestampFromTime(startTime))
			copyEnvelopeAttributes(dataPoint.Attributes(), envelope)
		}
	}
}

func convertEnvelopeToLogs(envelope *loggregator_v2.Envelope, logSlice plog.LogRecordSlice, startTime time.Time) error {
	log := logSlice.AppendEmpty()
	log.SetTimestamp(pcommon.Timestamp(envelope.GetTimestamp()))
	log.SetObservedTimestamp(pcommon.NewTimestampFromTime(startTime))
	logLine := string(envelope.GetLog().GetPayload())
	log.Body().SetStr(logLine)
	switch envelope.GetLog().GetType() {
	case loggregator_v2.Log_OUT:
		log.SetSeverityText(plog.SeverityNumberInfo.String())
		log.SetSeverityNumber(plog.SeverityNumberInfo)
	case loggregator_v2.Log_ERR:
		log.SetSeverityText(plog.SeverityNumberError.String())
		log.SetSeverityNumber(plog.SeverityNumberError)
	}
	copyEnvelopeAttributes(log.Attributes(), envelope)
	if t, found := envelope.Tags[envelopeSourceTypeKey]; found && t == envelopeSourceTypeValueRTR {
		traceID, spanID, flags, err := getTracingIDs(logLine)
		if err != nil {
			return err
		}
		if !pcommon.TraceID(traceID).IsEmpty() {
			log.SetTraceID(traceID)
			log.SetSpanID(spanID)
			log.SetFlags(plog.LogRecordFlags(flags))
		}
	}
	return nil
}

func getTracingIDs(logLine string) (traceID [16]byte, spanID [8]byte, flags byte, err error) {
	var trace []byte
	var span []byte
	var hexFlags []byte
	_, wordsMap := parseLogLine(logLine)
	traceIDStr, foundW3C := wordsMap[logLineRTRW3CKey]
	if foundW3C {
		// Use W3C headers
		traceW3C := strings.Split(traceIDStr, "-")
		if len(traceW3C) != 4 || traceW3C[0] != traceW3CVersion {
			err = fmt.Errorf(
				"traceId W3C key %s with format %s not valid in log",
				logLineRTRW3CKey, traceW3C[0])
			return
		}
		trace = []byte(traceW3C[1])
		span = []byte(traceW3C[2])
		hexFlags = []byte(traceW3C[3])
	} else {
		// try Zipkin headers
		traceIDStr, foundZk := wordsMap[logLineRTRZipkinKey]
		if !foundZk {
			// log line has no tracing headers
			return
		}
		traceZk := strings.Split(traceIDStr, "-")
		if len(traceZk) != 2 {
			err = fmt.Errorf(
				"traceId Zipkin key %s not valid in log",
				logLineRTRZipkinKey)
			return
		}
		trace = []byte(traceZk[0])
		span = []byte(traceZk[1])
		hexFlags = []byte("00")
	}
	traceDecoded := make([]byte, 16)
	spanDecoded := make([]byte, 8)
	flagsDecoded := make([]byte, 1)
	if _, err = hex.Decode(traceDecoded, trace); err != nil {
		return
	}
	if _, err = hex.Decode(spanDecoded, span); err != nil {
		return
	}
	if _, err = hex.Decode(flagsDecoded, hexFlags); err != nil {
		return
	}
	copy(traceID[:], traceDecoded)
	copy(spanID[:], spanDecoded)
	flags = flagsDecoded[0]
	return
}

func copyEnvelopeAttributes(attributes pcommon.Map, envelope *loggregator_v2.Envelope) {
	for key, value := range envelope.Tags {
		attributes.PutStr(attributeNamePrefix+key, value)
	}
	if envelope.SourceId != "" {
		attributes.PutStr(attributeNamePrefix+"source_id", envelope.SourceId)
	}
	if envelope.InstanceId != "" {
		attributes.PutStr(attributeNamePrefix+"instance_id", envelope.InstanceId)
	}
}

func parseLogLine(s string) ([]string, map[string]string) {
	wordList := make([]string, 0, 20)
	sb := &strings.Builder{}
	mapValue := &strings.Builder{}
	timestamp := &strings.Builder{}
	isTimeStamp := false
	mapKey := ""
	isMap := false
	isQuoted := false
	wordMap := make(map[string]string)
	for _, ch := range s {
		if ch == '"' {
			isQuoted = !isQuoted
			sb.WriteRune(ch)
			continue
		}
		if isQuoted {
			sb.WriteRune(ch)
			if isMap {
				mapValue.WriteRune(ch)
			}
			continue
		}
		if ch == '[' && sb.Len() == 0 {
			// first char after space
			isTimeStamp = true
			continue
		}
		if ch == ']' && isTimeStamp {
			wordList = append(wordList, timestamp.String())
			timestamp.Reset()
			isTimeStamp = false
			continue
		}
		if isTimeStamp {
			timestamp.WriteRune(ch)
			continue
		}
		if unicode.IsSpace(ch) {
			if sb.Len() > 0 {
				word := sb.String()
				if isMap {
					wordMap[mapKey] = mapValue.String()
				} else if strings.HasPrefix(word, `"`) && strings.HasSuffix(word, `"`) {
					// remove " if the item is not a keyMap and starts and ends with it
					word = strings.Trim(word, `"`)
				}
				wordList = append(wordList, word)
			}
			isMap = false
			mapValue.Reset()
			sb.Reset()
			continue
		}
		if isMap {
			mapValue.WriteRune(ch)
		} else if ch == ':' {
			mapKey = sb.String()
			isMap = true
		}
		sb.WriteRune(ch)
	}
	if sb.Len() > 0 {
		word := sb.String()
		if isMap {
			wordMap[mapKey] = mapValue.String()
		} else if strings.HasPrefix(word, `"`) && strings.HasSuffix(word, `"`) {
			// remove " if the item is not a keyMap and starts and ends with it
			word = strings.Trim(word, `"`)
		}
		wordList = append(wordList, word)
	}
	return wordList, wordMap
}
