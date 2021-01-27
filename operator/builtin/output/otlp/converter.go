// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package otlp

import (
	"encoding/json"
	"fmt"

	"github.com/opentelemetry/opentelemetry-log-collection/entry"
	"github.com/opentelemetry/opentelemetry-log-collection/version"
	"go.opentelemetry.io/collector/consumer/pdata"
)

// Convert converts a slice of entries to pdata.Logs format
func Convert(entries []*entry.Entry) pdata.Logs {

	out := pdata.NewLogs()
	logs := out.ResourceLogs()

	entriesByResource := groupByResource(entries)

	logs.Resize(len(entriesByResource))

	for i, resourceEntries := range entriesByResource {
		rls := logs.At(i)
		resource := rls.Resource()
		resource.InitEmpty()

		resourceAtts := resource.Attributes()
		for k, v := range resourceEntries[0].Resource {
			resourceAtts.InsertString(k, v)
		}

		rls.InstrumentationLibraryLogs().Resize(1)
		ills := rls.InstrumentationLibraryLogs().At(0)
		ills.InitEmpty()

		il := ills.InstrumentationLibrary()
		il.InitEmpty()
		il.SetName("stanza")
		il.SetVersion(version.GetVersion())

		for _, entry := range resourceEntries {
			lr := pdata.NewLogRecord()
			lr.InitEmpty()
			lr.SetTimestamp(pdata.TimestampUnixNano(entry.Timestamp.UnixNano()))

			lr.SetSeverityNumber(convertSeverity(entry.Severity))
			lr.SetSeverityText(entry.SeverityText)

			if len(entry.Labels) > 0 {
				attributes := lr.Attributes()
				for k, v := range entry.Labels {
					attributes.InsertString(k, v)
				}
			}

			lr.Body().InitEmpty()
			insertToAttributeVal(entry.Record, lr.Body())

			ills.Logs().Append(lr)
		}
	}

	return out
}

func groupByResource(entries []*entry.Entry) [][]*entry.Entry {
	resourceMap := make(map[string][]*entry.Entry)

	for _, ent := range entries {
		resourceBytes, err := json.Marshal(ent.Resource)
		if err != nil {
			continue // not expected to ever happen
		}
		resourceHash := string(resourceBytes)

		if resourceEntries, ok := resourceMap[resourceHash]; ok {
			resourceEntries = append(resourceEntries, ent)
		} else {
			resourceEntries = make([]*entry.Entry, 0, 8)
			resourceEntries = append(resourceEntries, ent)
			resourceMap[resourceHash] = resourceEntries
		}
	}

	entriesByResource := make([][]*entry.Entry, 0, len(resourceMap))
	for _, v := range resourceMap {
		entriesByResource = append(entriesByResource, v)
	}
	return entriesByResource
}

func insertToAttributeVal(value interface{}, dest pdata.AttributeValue) {
	switch t := value.(type) {
	case bool:
		dest.SetBoolVal(t)
	case string:
		dest.SetStringVal(t)
	case []byte:
		dest.SetStringVal(string(t))
	case int64:
		dest.SetIntVal(t)
	case int32:
		dest.SetIntVal(int64(t))
	case int16:
		dest.SetIntVal(int64(t))
	case int8:
		dest.SetIntVal(int64(t))
	case int:
		dest.SetIntVal(int64(t))
	case uint64:
		dest.SetIntVal(int64(t))
	case uint32:
		dest.SetIntVal(int64(t))
	case uint16:
		dest.SetIntVal(int64(t))
	case uint8:
		dest.SetIntVal(int64(t))
	case uint:
		dest.SetIntVal(int64(t))
	case float64:
		dest.SetDoubleVal(t)
	case float32:
		dest.SetDoubleVal(float64(t))
	case map[string]string:
		attMap := pdata.NewAttributeMap()
		attMap.InitEmptyWithCapacity(len(t))
		for k, v := range t {
			attMap.InsertString(k, v)
		}
		dest.SetMapVal(attMap)
	case map[string]interface{}:
		dest.SetMapVal(toAttributeMap(t))
	case []string:
		arr := pdata.NewAnyValueArray()
		for _, v := range t {
			attVal := pdata.NewAttributeValue()
			attVal.SetStringVal(v)
			arr.Append(attVal)
		}
		dest.SetArrayVal(arr)
	case []interface{}:
		dest.SetArrayVal(toAttributeArray(t))
	default:
		dest.SetStringVal(fmt.Sprintf("%v", t))
	}
}

func toAttributeMap(obsMap map[string]interface{}) pdata.AttributeMap {
	attMap := pdata.NewAttributeMap()
	attMap.InitEmptyWithCapacity(len(obsMap))
	for k, v := range obsMap {
		switch t := v.(type) {
		case bool:
			attMap.InsertBool(k, t)
		case string:
			attMap.InsertString(k, t)
		case []byte:
			attMap.InsertString(k, string(t))
		case int64:
			attMap.InsertInt(k, t)
		case int32:
			attMap.InsertInt(k, int64(t))
		case int16:
			attMap.InsertInt(k, int64(t))
		case int8:
			attMap.InsertInt(k, int64(t))
		case int:
			attMap.InsertInt(k, int64(t))
		case uint64:
			attMap.InsertInt(k, int64(t))
		case uint32:
			attMap.InsertInt(k, int64(t))
		case uint16:
			attMap.InsertInt(k, int64(t))
		case uint8:
			attMap.InsertInt(k, int64(t))
		case uint:
			attMap.InsertInt(k, int64(t))
		case float64:
			attMap.InsertDouble(k, t)
		case float32:
			attMap.InsertDouble(k, float64(t))
		case map[string]string:
			subMap := pdata.NewAttributeMap()
			subMap.InitEmptyWithCapacity(len(t))
			for k, v := range t {
				subMap.InsertString(k, v)
			}
			subMapVal := pdata.NewAttributeValueMap()
			subMapVal.SetMapVal(subMap)
			attMap.Insert(k, subMapVal)
		case map[string]interface{}:
			subMap := toAttributeMap(t)
			subMapVal := pdata.NewAttributeValueMap()
			subMapVal.SetMapVal(subMap)
			attMap.Insert(k, subMapVal)
		case []string:
			arr := pdata.NewAnyValueArray()
			for _, v := range t {
				attVal := pdata.NewAttributeValue()
				insertToAttributeVal(v, attVal)
				arr.Append(attVal)
			}
			arrVal := pdata.NewAttributeValueArray()
			arrVal.SetArrayVal(arr)
			attMap.Insert(k, arrVal)
		case []interface{}:
			arr := toAttributeArray(t)
			arrVal := pdata.NewAttributeValueArray()
			arrVal.SetArrayVal(arr)
			attMap.Insert(k, arrVal)
		default:
			attMap.InsertString(k, fmt.Sprintf("%v", t))
		}
	}
	return attMap
}

func toAttributeArray(obsArr []interface{}) pdata.AnyValueArray {
	arr := pdata.NewAnyValueArray()
	for _, v := range obsArr {
		attVal := pdata.NewAttributeValue()
		insertToAttributeVal(v, attVal)
		arr.Append(attVal)
	}
	return arr
}

var namedLevels = map[entry.Severity]pdata.SeverityNumber{
	entry.Default:     pdata.SeverityNumberUNDEFINED,
	entry.Trace:       pdata.SeverityNumberTRACE,
	entry.Trace2:      pdata.SeverityNumberTRACE2,
	entry.Trace3:      pdata.SeverityNumberTRACE3,
	entry.Trace4:      pdata.SeverityNumberTRACE4,
	entry.Debug:       pdata.SeverityNumberDEBUG,
	entry.Debug2:      pdata.SeverityNumberDEBUG2,
	entry.Debug3:      pdata.SeverityNumberDEBUG3,
	entry.Debug4:      pdata.SeverityNumberDEBUG4,
	entry.Info:        pdata.SeverityNumberINFO,
	entry.Info2:       pdata.SeverityNumberINFO2,
	entry.Info3:       pdata.SeverityNumberINFO3,
	entry.Info4:       pdata.SeverityNumberINFO4,
	entry.Notice:      pdata.SeverityNumberINFO4,
	entry.Warning:     pdata.SeverityNumberWARN,
	entry.Warning2:    pdata.SeverityNumberWARN2,
	entry.Warning3:    pdata.SeverityNumberWARN3,
	entry.Warning4:    pdata.SeverityNumberWARN4,
	entry.Error:       pdata.SeverityNumberERROR,
	entry.Error2:      pdata.SeverityNumberERROR2,
	entry.Error3:      pdata.SeverityNumberERROR3,
	entry.Error4:      pdata.SeverityNumberERROR4,
	entry.Critical:    pdata.SeverityNumberERROR4,
	entry.Alert:       pdata.SeverityNumberERROR4,
	entry.Emergency:   pdata.SeverityNumberFATAL,
	entry.Emergency2:  pdata.SeverityNumberFATAL2,
	entry.Emergency3:  pdata.SeverityNumberFATAL3,
	entry.Emergency4:  pdata.SeverityNumberFATAL4,
	entry.Catastrophe: pdata.SeverityNumberFATAL4,
}

func convertSeverity(s entry.Severity) pdata.SeverityNumber {
	// Handle named severity levels
	if sev, ok := namedLevels[s]; ok {
		return sev
	}

	// Handle custom severity levels
	switch {
	case s > entry.Emergency4:
		return pdata.SeverityNumberFATAL4
	case s > entry.Emergency:
		return pdata.SeverityNumberFATAL
	case s > entry.Error4:
		return pdata.SeverityNumberERROR4
	case s > entry.Error:
		return pdata.SeverityNumberERROR
	case s > entry.Warning4:
		return pdata.SeverityNumberWARN4
	case s > entry.Warning:
		return pdata.SeverityNumberWARN
	case s > entry.Info4:
		return pdata.SeverityNumberINFO4
	case s > entry.Info:
		return pdata.SeverityNumberINFO
	case s > entry.Debug4:
		return pdata.SeverityNumberDEBUG4
	case s > entry.Debug:
		return pdata.SeverityNumberDEBUG
	case s > entry.Trace4:
		return pdata.SeverityNumberTRACE4
	case s > entry.Default:
		return pdata.SeverityNumberTRACE
	default:
		return pdata.SeverityNumberUNDEFINED
	}
}
