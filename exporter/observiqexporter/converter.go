// Copyright  OpenTelemetry Authors
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

package observiqexporter

import (
	"encoding/hex"
	"encoding/json"
	"hash/fnv"
	"strings"

	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/model/pdata"
)

// Type preEncodedJSON aliases []byte, represents JSON that has already been encoded
// Does not support unmarshalling
type preEncodedJSON []byte

type observIQLogBatch struct {
	Logs []*observIQLog `json:"logs"`
}

type observIQLog struct {
	ID    string         `json:"id"`
	Size  int            `json:"size"`
	Entry preEncodedJSON `json:"entry"`
}

type observIQAgentInfo struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Version string `json:"version"`
}
type observIQLogEntry struct {
	Timestamp string                 `json:"@timestamp"`
	Severity  string                 `json:"severity,omitempty"`
	EntryType string                 `json:"type,omitempty"`
	Message   string                 `json:"message,omitempty"`
	Resource  map[string]interface{} `json:"resource,omitempty"`
	Agent     *observIQAgentInfo     `json:"agent,omitempty"`
	Data      map[string]interface{} `json:"data,omitempty"`
	Body      interface{}            `json:"body,omitempty"`
}

// Hash related variables, re-used to avoid multiple allocations
var fnvHash = fnv.New128a()
var fnvHashOut = make([]byte, 0, 16)

// Convert pdata.Logs to observIQLogBatch
func logdataToObservIQFormat(ld pdata.Logs, agentID string, agentName string, buildVersion string) (*observIQLogBatch, []error) {
	var rls = ld.ResourceLogs()
	var sliceOut = make([]*observIQLog, 0, ld.LogRecordCount())
	var errorsOut = make([]error, 0)

	for i := 0; i < rls.Len(); i++ {
		rl := rls.At(i)
		res := rl.Resource()
		resMap := attributeMapToBaseType(res.Attributes())
		ills := rl.InstrumentationLibraryLogs()
		for j := 0; j < ills.Len(); j++ {
			ill := ills.At(j)
			logs := ill.Logs()
			for k := 0; k < logs.Len(); k++ {
				oiqLogEntry := resourceAndInstrumentationLogToEntry(resMap, logs.At(k), agentID, agentName, buildVersion)

				jsonOIQLogEntry, err := json.Marshal(oiqLogEntry)

				if err != nil {
					//Skip this log, keep record of error
					errorsOut = append(errorsOut, consumererror.NewPermanent(err))
					continue
				}

				//fnv sum of the message is ID
				fnvHash.Reset()
				_, err = fnvHash.Write(jsonOIQLogEntry)
				if err != nil {
					errorsOut = append(errorsOut, consumererror.NewPermanent(err))
					continue
				}

				fnvHashOut = fnvHashOut[:0]
				fnvHashOut = fnvHash.Sum(fnvHashOut)

				fnvHashAsHex := hex.EncodeToString(fnvHashOut)

				sliceOut = append(sliceOut, &observIQLog{
					ID:    fnvHashAsHex,
					Size:  len(jsonOIQLogEntry),
					Entry: preEncodedJSON(jsonOIQLogEntry),
				})
			}
		}
	}

	return &observIQLogBatch{Logs: sliceOut}, errorsOut
}

// Output timestamp format, an ISO8601 compliant timestamp with millisecond precision
const timestampFieldOutputLayout = "2006-01-02T15:04:05.000Z07:00"

func resourceAndInstrumentationLogToEntry(resMap map[string]interface{}, log pdata.LogRecord, agentID string, agentName string, buildVersion string) *observIQLogEntry {
	return &observIQLogEntry{
		Timestamp: timestampFromRecord(log),
		Severity:  severityFromRecord(log),
		Resource:  resMap,
		Message:   messageFromRecord(log),
		Data:      attributeMapToBaseType(log.Attributes()),
		Body:      bodyFromRecord(log),
		Agent:     &observIQAgentInfo{Name: agentName, ID: agentID, Version: buildVersion},
	}
}

func timestampFromRecord(log pdata.LogRecord) string {
	if log.Timestamp() == 0 {
		return timeNow().UTC().Format(timestampFieldOutputLayout)
	}
	return log.Timestamp().AsTime().UTC().Format(timestampFieldOutputLayout)
}

func messageFromRecord(log pdata.LogRecord) string {
	if log.Body().Type() == pdata.AttributeValueTypeString {
		return log.Body().StringVal()
	}

	return ""
}

// bodyFromRecord returns what the "body" field should be on the observiq entry from the given LogRecord.
func bodyFromRecord(log pdata.LogRecord) interface{} {
	if log.Body().Type() != pdata.AttributeValueTypeString {
		return attributeValueToBaseType(log.Body())
	}
	return nil
}

//Mappings from opentelemetry severity number to observIQ severity string
var severityNumberToObservIQName = map[int32]string{
	0:  "default",
	1:  "trace",
	2:  "trace",
	3:  "trace",
	4:  "trace",
	5:  "debug",
	6:  "debug",
	7:  "debug",
	8:  "debug",
	9:  "info",
	10: "notice",
	11: "notice",
	12: "notice",
	13: "warning",
	14: "warning",
	15: "warning",
	16: "warning",
	17: "error",
	18: "critical",
	19: "alert",
	20: "alert",
	21: "emergency",
	22: "emergency",
	23: "catastrophe",
	24: "catastrophe",
}

/*
	Get severity from the a log record.
	We prefer the severity number, and map it to a string
	representing the opentelemetry defined severity.
	If there is no severity number, we use "default"
*/
func severityFromRecord(log pdata.LogRecord) string {
	var sevAsInt32 = int32(log.SeverityNumber())
	if sevAsInt32 < int32(len(severityNumberToObservIQName)) && sevAsInt32 >= 0 {
		return severityNumberToObservIQName[sevAsInt32]
	}
	return "default"
}

/*
	Transform AttributeMap to native Go map, skipping keys with nil values, and replacing dots in keys with _
*/
func attributeMapToBaseType(m pdata.AttributeMap) map[string]interface{} {
	mapOut := make(map[string]interface{}, m.Len())
	m.Range(func(k string, v pdata.AttributeValue) bool {
		val := attributeValueToBaseType(v)
		if val != nil {
			dedotedKey := strings.ReplaceAll(k, ".", "_")
			mapOut[dedotedKey] = val
		}
		return true
	})
	return mapOut
}

/*
	attrib is the attribute value to convert to it's native Go type - skips nils in arrays/maps
*/
func attributeValueToBaseType(attrib pdata.AttributeValue) interface{} {
	switch attrib.Type() {
	case pdata.AttributeValueTypeString:
		return attrib.StringVal()
	case pdata.AttributeValueTypeBool:
		return attrib.BoolVal()
	case pdata.AttributeValueTypeInt:
		return attrib.IntVal()
	case pdata.AttributeValueTypeDouble:
		return attrib.DoubleVal()
	case pdata.AttributeValueTypeMap:
		attribMap := attrib.MapVal()
		return attributeMapToBaseType(attribMap)
	case pdata.AttributeValueTypeArray:
		arrayVal := attrib.ArrayVal()
		slice := make([]interface{}, 0, arrayVal.Len())
		for i := 0; i < arrayVal.Len(); i++ {
			val := attributeValueToBaseType(arrayVal.At(i))
			if val != nil {
				slice = append(slice, val)
			}
		}
		return slice
	case pdata.AttributeValueTypeEmpty:
		return nil
	}
	return nil
}

// The marshaled JSON is just the []byte that this type aliases.
func (p preEncodedJSON) MarshalJSON() ([]byte, error) {
	return p, nil
}
