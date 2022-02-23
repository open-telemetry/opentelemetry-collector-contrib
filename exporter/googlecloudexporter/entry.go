// Copyright  The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package googlecloudexporter

import (
	"fmt"
	"net/url"
	"strings"

	"go.opentelemetry.io/collector/model/pdata"
	"google.golang.org/genproto/googleapis/logging/v2"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const defaultMaxEntrySize = 256000

// EntryBuilder is an interface for building google cloud logging entries.
type EntryBuilder interface {
	Build(oLogRecord *pdata.LogRecord) (*logging.LogEntry, error)
}

// GoogleEntryBuilder is used to build google cloud logging entries.
// GoogleEntryBuilder implements the EntryBuilder interface.
type GoogleEntryBuilder struct {
	MaxEntrySize int
	ProjectID    string
	NameFields   []string
}

// Build builds a google cloud logging entry from an OTEL log record.
// Build will throw an error if the entry size is larger than the max entry size.
func (g *GoogleEntryBuilder) Build(oLogRecord *pdata.LogRecord) (*logging.LogEntry, error) {
	logEntry := &logging.LogEntry{
		Timestamp: timestamppb.New(oLogRecord.Timestamp().AsTime()),
		Severity:  convertSeverity(oLogRecord.SeverityNumber()),
	}

	if err := g.setLogName(oLogRecord, logEntry); err != nil {
		return nil, fmt.Errorf("failed to set log name: %w", err)
	}

	g.setTrace(oLogRecord, logEntry)
	g.setSpanID(oLogRecord, logEntry)
	g.setLabels(oLogRecord, logEntry)

	if err := g.setPayload(oLogRecord, logEntry); err != nil {
		return nil, fmt.Errorf("failed to set payload: %w", err)
	}

	protoSize := proto.Size(logEntry)
	if protoSize > g.MaxEntrySize {
		return nil, fmt.Errorf("exceeds max entry size: %d", protoSize)
	}

	return logEntry, nil
}

// setLogName sets the log name of the google log entry using the indicated attribute on the OTEL log record.
// setLogName will return an error if the provided nameFields are impoperly formatted or don't point to a string.
func (g *GoogleEntryBuilder) setLogName(oLogRecord *pdata.LogRecord, logEntry *logging.LogEntry) error {
	if g.NameFields == nil {
		return nil
	}

	name, err := findLogName(oLogRecord, g.NameFields)
	if err != nil {
		return fmt.Errorf("failed to read log name field: %w", err)
	}

	logEntry.LogName = createLogName(g.ProjectID, name)

	return nil
}

// setTrace sets the trace of the google log entry using the equivalent field on the OTEL log record.
func (g *GoogleEntryBuilder) setTrace(oLogRecord *pdata.LogRecord, logEntry *logging.LogEntry) {
	if oLogRecord.TraceID().HexString() == "" {
		return
	}

	logEntry.Trace = oLogRecord.TraceID().HexString()
}

// setSpanID sets the span id of the google log entry using the equivalent field on the OTEL log record.
func (g *GoogleEntryBuilder) setSpanID(oLogRecord *pdata.LogRecord, logEntry *logging.LogEntry) {
	if oLogRecord.SpanID().HexString() == "" {
		return
	}

	logEntry.SpanId = oLogRecord.SpanID().HexString()
}

// setLabels sets the labels of the google log entry based on the supplied OTEL log record's attributes.
func (g *GoogleEntryBuilder) setLabels(oLogRecord *pdata.LogRecord, logEntry *logging.LogEntry) {
	attrs := oLogRecord.Attributes()
	if attrs.Len() == 0 {
		return
	}

	labels := make(map[string]string)

	findAttrsForAttrMap := func(key string, value pdata.AttributeValue) bool {
		for k, v := range retrieveNestedAttributes(key, value) {
			labels[k] = v
		}
		return true
	}

	attrs.Range(findAttrsForAttrMap)

	logEntry.Labels = labels
}

// setPayload sets the payload of the google log entry based on the supplied OTEL log record.
// setPayload will throw an error if there is a problem converting the OTEL log record's body.
func (g *GoogleEntryBuilder) setPayload(oLogRecord *pdata.LogRecord, logEntry *logging.LogEntry) error {
	switch oLogRecord.Body().Type() {
	case pdata.AttributeValueTypeString:
		logEntry.Payload = &logging.LogEntry_TextPayload{TextPayload: oLogRecord.Body().AsString()}
		return nil
	case pdata.AttributeValueTypeBytes:
		logEntry.Payload = &logging.LogEntry_TextPayload{TextPayload: string(oLogRecord.Body().BytesVal())}
		return nil
	case pdata.AttributeValueTypeMap:
		structValue, err := convertToProto(oLogRecord.Body().MapVal().AsRaw())
		if err != nil {
			return fmt.Errorf("failed to convert record of type map[string]interface: %w", err)
		}

		logEntry.Payload = &logging.LogEntry_JsonPayload{JsonPayload: structValue.GetStructValue()}
		return nil
	default:
		return fmt.Errorf("cannot convert record of type %s", oLogRecord.Body().Type().String())
	}
}

// createLogName creates a log name from the supplied project id and name value.
func createLogName(projectID, name string) string {
	return fmt.Sprintf("projects/%s/logs/%s", projectID, url.PathEscape(name))
}

// findLogName traverses through the OTEL log record's attributes using the given name fields.
// It returns the first matching value that can be converted to a string.
// findLogName can throw an error if a given name field isn't in thet correct structure, or if it doesn't result in a string value.
func findLogName(oLogRecord *pdata.LogRecord, nameFields []string) (string, error) {
	currAttr := oLogRecord.Attributes()

	for _, nameField := range nameFields {
		nameFieldKeys := strings.Split(nameField, ".")
		len := len(nameFieldKeys)

		for i, nameFieldKey := range nameFieldKeys {
			nextAttr, ok := currAttr.Get(nameFieldKey)

			if !ok {
				continue
			}

			if i < len-1 {
				switch nextAttr.Type() {
				case pdata.AttributeValueTypeMap:
					currAttr = nextAttr.MapVal()
				default:
					return "", fmt.Errorf("key: %s value must be of type: %s but instead is: %s", nameFieldKey, pdata.AttributeValueTypeMap, nextAttr.Type())
				}
			} else {
				switch nextAttr.Type() {
				case pdata.AttributeValueTypeMap:
					return "", fmt.Errorf("name value can't be of type: %s", pdata.AttributeValueTypeMap)
				default:
					name := nextAttr.AsString()
					currAttr.Delete(nameFieldKey)
					return name, nil
				}
			}
		}
	}

	return "", nil
}

// retrieveNestedAttributes takes a given key and OTEL attribute value (from an AttributeMapValue)
// and returns a list of all nested values within the attribute.
func retrieveNestedAttributes(key string, value pdata.AttributeValue) map[string]string {
	nestedAttrs := make(map[string]string)

	switch value.Type() {
	case pdata.AttributeValueTypeMap:
		findAttrsForAttrMap := func(nestedKey string, nestedValue pdata.AttributeValue) bool {
			for k, v := range retrieveNestedAttributes(key+"."+nestedKey, nestedValue) {
				nestedAttrs[k] = v
			}
			return true
		}

		value.MapVal().Range(findAttrsForAttrMap)
	default:
		nestedAttrs[key] = value.AsString()
	}

	return nestedAttrs
}
