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
// slice of [StackFrame] and a slice of [ExeMetadata] that have been serialized,
// and need to be ingested into ES.
type StackPayload struct {
	StackTraceEvent StackTraceEvent
	StackTrace      StackTrace
	StackFrames     []StackFrame
	Executables     []ExeMetadata

	ResourceAttrs ResourceData
}

// StackTraceEvent represents a stacktrace event serializable into ES.
type StackTraceEvent struct {
	TimeStamp        unixTime64
	StackTraceID     string
	Frequency        int64
	Count            uint16
	HostID           string
	PodName          string
	ContainerID      string
	ContainerName    string
	K8sNamespaceName string
	ThreadName       string
	ExecutableName   string
	ServiceName      string
	HostName         string
}

// MarshalJSON serializes StackTraceEvent with resource.attributes as a nested object
// to match the Elasticsearch passthrough mapping.
func (e StackTraceEvent) MarshalJSON() ([]byte, error) {
	attrs := map[string]any{
		"host.id":                 e.HostID,
		"thread.name":             e.ThreadName,
		"process.executable.name": e.ExecutableName,
	}
	if e.PodName != "" {
		attrs["k8s.pod.name"] = e.PodName
	}
	if e.ContainerID != "" {
		attrs["container.id"] = e.ContainerID
	}
	if e.ContainerName != "" {
		attrs["container.name"] = e.ContainerName
	}
	if e.K8sNamespaceName != "" {
		attrs["k8s.namespace.name"] = e.K8sNamespaceName
	}
	if e.ServiceName != "" {
		attrs["service.name"] = e.ServiceName
	}
	if e.HostName != "" {
		attrs["host.name"] = e.HostName
	}
	return json.Marshal(map[string]any{
		"@timestamp":         e.TimeStamp,
		"stacktrace.id":      e.StackTraceID,
		"sampling_frequency": e.Frequency,
		"count":              e.Count,
		"resource":           map[string]any{"attributes": attrs},
	})
}

// StackTrace represents a stacktrace serializable into the stacktraces index.
// DocID should be the base64-encoded Stacktrace ID.
type StackTrace struct {
	DocID     string     `json:"-"`
	Timestamp unixTime64 `json:"@timestamp"`
	FrameIDs  string     `json:"frame.ids"`
	Types     string     `json:"frame.types"`
}

// StackFrame represents a stacktrace serializable into the stackframes index.
// DocID should be the base64-encoded FileID+Address (24 bytes).
// To simplify the unmarshalling for readers, we use arrays here, even though host agent
// doesn't send inline information yet. The symbolizer already stores arrays, which requires
// the reader to handle both formats if we don't use arrays here.
type StackFrame struct {
	DocID          string     `json:"-"`
	Timestamp      unixTime64 `json:"@timestamp"`
	FileName       []string   `json:"function.filename,omitempty"`
	FunctionName   []string   `json:"function.name,omitempty"`
	LineNumber     []int32    `json:"line.number,omitempty"`
	FunctionOffset []int32    `json:"function.offset,omitempty"`
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
		combinedData["resource.attributes."+strings.ToLower(key)] = strings.ToLower(value)
	}

	// Marshal the combined map into JSON
	return json.Marshal(combinedData)
}

// ExeMetadata represents executable metadata serializable into the profiling-executables datastream.
// DocID should be the htlhash build ID string.
type ExeMetadata struct {
	DocID     string
	Timestamp uint32
	BuildID   string
	Name      string
}

// MarshalJSON serializes ExeMetadata with resource.attributes as a nested object
// to match the Elasticsearch passthrough mapping.
func (e ExeMetadata) MarshalJSON() ([]byte, error) {
	attrs := map[string]any{}
	if e.BuildID != "" {
		attrs["process.executable.build_id.htlhash"] = e.BuildID
	}
	if e.Name != "" {
		attrs["process.executable.name"] = e.Name
	}
	m := map[string]any{"@timestamp": e.Timestamp}
	if len(attrs) > 0 {
		m["resource"] = map[string]any{"attributes": attrs}
	}
	return json.Marshal(m)
}
