// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package serializeprofiles // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/serializer/otelserializer/serializeprofiles"

import (
	"encoding/json"
	"strings"
	"time"

	conventions "go.opentelemetry.io/otel/semconv/v1.40.0"
)

// StackPayload represents a single [StackTraceEvent], with a [StackTrace], a
// map of [StackFrames] and a map of [ExeMetadata] that have been serialized,
// and need to be ingested into ES.
type StackPayload struct {
	StackTraceEvent StackTraceEvent
	StackTrace      StackTrace
	StackFrames     []StackFrame
	Executables     []ExeMetadata

	ResourceAttrs ResourceData
}

// StackTraceEvent represents a stacktrace event serializable into ES.
// The json field names need to be case-sensitively equal to the fields defined
// in the schema mapping.
type StackTraceEvent struct {
	// Event-specific metadata
	TimeStamp    unixTime64 `json:"@timestamp"`
	StackTraceID string     `json:"stacktrace.id"` // 128-bit hash in binary form
	Frequency    int64      `json:"sampling_frequency"`
	Count        uint16     `json:"count"`
	HostID       string     `json:"resource.attribute.host.id"`

	// Additional known resource attributes
	PodName          string `json:"resource.attribute.k8s.pod.name,omitempty"`
	ContainerID      string `json:"resource.attribute.container.id,omitempty"`
	ContainerName    string `json:"resource.attribute.container.name,omitempty"`
	K8sNamespaceName string `json:"resource.attribute.k8s.namespace.name,omitempty"`
	ThreadName       string `json:"resource.attribute.process.thread.name"`
	ExecutableName   string `json:"resource.attribute.process.executable.name"`
	ServiceName      string `json:"resource.attribute.service.name,omitempty"`
	HostName         string `json:"resource.attribute.host.name,omitempty"`
}

// StackTrace represents a stacktrace serializable into the stacktraces index.
// DocID should be the base64-encoded Stacktrace ID.
type StackTrace struct {
	DocID    string `json:"-"`
	FrameIDs string `json:"frame.ids"`
	Types    string `json:"frame.types"`
}

// StackFrame represents a stacktrace serializable into the stackframes index.
// DocID should be the base64-encoded FileID+Address (24 bytes).
// To simplify the unmarshalling for readers, we use arrays here, even though host agent
// doesn't send inline information yet. The symbolizer already stores arrays, which requires
// the reader to handle both formats if we don't use arrays here.
type StackFrame struct {
	DocID          string   `json:"-"`
	FileName       []string `json:"function.filename,omitempty"`
	FunctionName   []string `json:"function.name,omitempty"`
	LineNumber     []int32  `json:"line.number,omitempty"`
	FunctionOffset []int32  `json:"function.offset,omitempty"`
}

// ResourceData represents the resources metadata related to a sample for the
// profiling-hosts index.
type ResourceData struct {
	HostID string `json:"host.id"`
	Data   map[string]string
}

// MarshalJSON customizes the JSON marshaling for HostResourceData.
func (h ResourceData) MarshalJSON() ([]byte, error) {
	// Create a temporary map to hold the combined data
	combinedData := make(map[string]any)

	combinedData[string(conventions.HostIDKey)] = h.HostID
	// The ES index profiling-hosts expects a second-precise timestamp
	combinedData["@timestamp"] = time.Now().UTC().Unix()

	// Iterate over the Data map and add the key-value pairs with lowercase keys and values
	for key, value := range h.Data {
		if value == "" {
			// Do not populate keys without value
			continue
		}
		combinedData[strings.ToLower(key)] = strings.ToLower(value)
	}

	// Marshal the combined map into JSON
	return json.Marshal(combinedData)
}

// Script written in Painless that will both create a new document (if DocID does not exist),
// and update timestamp of an existing document. Named parameters are used to improve performance
// re: script compilation (since the script does not change across executions, it can be compiled
// once and cached).
const ExeMetadataUpsertScript = `
if (ctx.op == 'create') {
		ctx._source['@timestamp']            = params.timestamp;
		ctx._source['Executable.build.id']   = params.buildid;
		ctx._source['Executable.file.name']  = params.filename;
} else {
		if (ctx._source['@timestamp'] == params.timestamp) {
				ctx.op = 'noop'
		} else {
				ctx._source['@timestamp'] = params.timestamp
		}
}
`

type ExeMetadataScript struct {
	Source string            `json:"source"`
	Params ExeMetadataParams `json:"params"`
}

type ExeMetadataParams struct {
	LastSeen uint32 `json:"timestamp"`
	BuildID  string `json:"buildid"`
	FileName string `json:"filename"`
}

// ExeMetadata represents executable metadata serializable into the executables index.
// DocID should be the base64-encoded FileID.
type ExeMetadata struct {
	DocID string `json:"-"`
	// ScriptedUpsert needs to be 'true' for the script to execute regardless of the
	// document existing or not.
	ScriptedUpsert bool              `json:"scripted_upsert"`
	Script         ExeMetadataScript `json:"script"`
	// This needs to exist for document creation to succeed (if document does not exist),
	// but can be empty as the script implements both document creation and updating.
	Upsert struct{} `json:"upsert"`
}

func NewExeMetadata(docID string, lastSeen uint32, buildID, fileName string) ExeMetadata {
	return ExeMetadata{
		DocID:          docID,
		ScriptedUpsert: true,
		Script: ExeMetadataScript{
			Source: ExeMetadataUpsertScript,
			Params: ExeMetadataParams{
				LastSeen: lastSeen,
				BuildID:  buildID,
				FileName: fileName,
			},
		},
	}
}
