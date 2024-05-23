// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cloudfoundryreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/cloudfoundryreceiver"

import (
	"fmt"
	"strings"
	"time"
	"unicode"

	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/otel/trace"
)

const (
	attributeNamePrefix        = "org.cloudfoundry."
	envelopeSourceTypeTag      = "org.cloudfoundry.source_type"
	envelopeSourceTypeValueRTR = "RTR"
	logLineRTRTraceIDKey       = "x_b3_traceid"
	logLineRTRSpanIDKey        = "x_b3_spanid"
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
	if _, isLog := envelope.Message.(*loggregator_v2.Envelope_Log); isLog {
		log := logSlice.AppendEmpty()
		log.SetTimestamp(pcommon.Timestamp(envelope.GetTimestamp()))
		log.SetObservedTimestamp(pcommon.NewTimestampFromTime(startTime))
		log.Body().SetStr(string(envelope.GetLog().GetPayload()))
		switch envelope.GetLog().GetType() {
		case loggregator_v2.Log_OUT:
			log.SetSeverityText(plog.SeverityNumberInfo.String())
			log.SetSeverityNumber(plog.SeverityNumberInfo)
		case loggregator_v2.Log_ERR:
			log.SetSeverityText(plog.SeverityNumberError.String())
			log.SetSeverityNumber(plog.SeverityNumberError)
		}
		copyEnvelopeAttributes(log.Attributes(), envelope)
		if value, found := log.Attributes().Get(envelopeSourceTypeTag); found && value.AsString() == envelopeSourceTypeValueRTR {
			_, wordsMap := parseLogLine(log.Body().AsString())
			traceIDStr, found := wordsMap[logLineRTRTraceIDKey]
			if !found {
				return fmt.Errorf("traceid key %s not found in log", logLineRTRTraceIDKey)
			}
			spanIDStr, found := wordsMap[logLineRTRSpanIDKey]
			if !found {
				return fmt.Errorf("spanid key %s not found in log", logLineRTRSpanIDKey)
			}
			traceID, err := trace.TraceIDFromHex(traceIDStr)
			if err != nil {
				return err
			}
			spanID, err := trace.SpanIDFromHex(spanIDStr)
			if err != nil {
				return err
			}
			log.SetTraceID([16]byte(traceID))
			log.SetSpanID([8]byte(spanID))
		}
		return nil
	}
	return fmt.Errorf("envelope is not a log")
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
		wordList = append(wordList, sb.String())
		if isMap {
			wordMap[mapKey] = mapValue.String()
		}
	}
	return wordList, wordMap
}
