// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package serializeprofiles // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/serializer/otelserializer/serializeprofiles"

import (
	"time"
)

// EcsVersionString is the value for the `ecs.version` metrics field.
// It is relatively arbitrary and currently has no consumer.
// APM server is using 1.12.0. We stick with it as well.
const EcsVersionString = "1.12.0"

// EcsVersion is a struct to hold the `ecs.version` metrics field.
// Used as a helper in ES index struct types.
type EcsVersion struct {
	V string `json:"ecs.version"`
}

// StackPayload represents a single [StackTraceEvent], with a [StackTrace], a
// map of [StackFrames] and a map of [ExeMetadata] that have been serialized,
// and need to be ingested into ES.
type StackPayload struct {
	StackTraceEvent StackTraceEvent
	StackTrace      StackTrace
	StackFrames     []StackFrame
	Executables     []ExeMetadata

	UnsymbolizedLeafFrames  []UnsymbolizedLeafFrame
	UnsymbolizedExecutables []UnsymbolizedExecutable
}

// StackTraceEvent represents a stacktrace event serializable into ES.
// The json field names need to be case-sensitively equal to the fields defined
// in the schema mapping.
type StackTraceEvent struct {
	EcsVersion
	TimeStamp    unixTime64 `json:"@timestamp"`
	HostID       string     `json:"host.id"`
	StackTraceID string     `json:"Stacktrace.id"` // 128-bit hash in binary form

	// Event-specific metadata
	PodName          string `json:"orchestrator.resource.name,omitempty"`
	ContainerID      string `json:"container.id,omitempty"`
	ContainerName    string `json:"container.name,omitempty"`
	K8sNamespaceName string `json:"k8s.namespace.name,omitempty"`
	ThreadName       string `json:"process.thread.name"`
	Count            uint16 `json:"Stacktrace.count"`
}

// StackTrace represents a stacktrace serializable into the stacktraces index.
// DocID should be the base64-encoded Stacktrace ID.
type StackTrace struct {
	EcsVersion
	DocID    string `json:"-"`
	FrameIDs string `json:"Stacktrace.frame.ids"`
	Types    string `json:"Stacktrace.frame.types"`
}

// StackFrame represents a stacktrace serializable into the stackframes index.
// DocID should be the base64-encoded FileID+Address (24 bytes).
// To simplify the unmarshalling for readers, we use arrays here, even though host agent
// doesn't send inline information yet. The symbolizer already stores arrays, which requires
// the reader to handle both formats if we don't use arrays here.
type StackFrame struct {
	EcsVersion
	DocID          string   `json:"-"`
	FileName       []string `json:"Stackframe.file.name,omitempty"`
	FunctionName   []string `json:"Stackframe.function.name,omitempty"`
	LineNumber     []int32  `json:"Stackframe.line.number,omitempty"`
	FunctionOffset []int32  `json:"Stackframe.function.offset,omitempty"`
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
		ctx._source['ecs.version']           = params.ecsversion;
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
	LastSeen   uint32 `json:"timestamp"`
	BuildID    string `json:"buildid"`
	FileName   string `json:"filename"`
	EcsVersion string `json:"ecsversion"`
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
				LastSeen:   lastSeen,
				BuildID:    buildID,
				FileName:   fileName,
				EcsVersion: EcsVersionString,
			},
		},
	}
}

// UnsymbolizedExecutable represents an array of executable FileIDs written into the
// executable symbolization queue index.
type UnsymbolizedExecutable struct {
	EcsVersion
	DocID   string    `json:"-"`
	FileID  []string  `json:"Executable.file.id"`
	Created time.Time `json:"Time.created"`
	Next    time.Time `json:"Symbolization.time.next"`
	Retries int       `json:"Symbolization.retries"`
}

// UnsymbolizedLeafFrame represents an array of frame IDs written into the
// leaf frame symbolization queue index.
type UnsymbolizedLeafFrame struct {
	EcsVersion
	DocID   string    `json:"-"`
	FrameID []string  `json:"Stacktrace.frame.id"`
	Created time.Time `json:"Time.created"`
	Next    time.Time `json:"Symbolization.time.next"`
	Retries int       `json:"Symbolization.retries"`
}
