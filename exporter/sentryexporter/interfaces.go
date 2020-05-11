// Copyright 2020 OpenTelemetry Authors
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

package sentryexporter

import (
	"time"

	"github.com/getsentry/sentry-go"
)

// StatusCode defines status codes for a finished span
type StatusCode int

// Enum for possible StatusCodes
const (
	Ok StatusCode = iota
	Cancelled
	Unknown
	InvalidArgument
	DeadlineExceeded
	NotFound
	AlreadyExists
	PermissionDenied
	ResourceExhausted
	FailedPrecondition
	Aborted
	OutOfRange
	Unimplemented
	InternalError
	Unavailable
	DataLoss
	Unauthenticated
)

func (statusCode StatusCode) String() string {
	codes := [...]string{
		"Ok",
		"Cancelled",
		"Unknown",
		"InvalidArgument",
		"DeadlineExceeded",
		"NotFound",
		"AlreadyExists",
		"PermissionDenied",
		"ResourceExhausted",
		"FailedPrecondition",
		"Aborted",
		"OutOfRange",
		"Unimplemented",
		"InternalError",
		"Unavailable",
		"DataLoss",
		"Unauthenticated",
	}

	return codes[statusCode]
}

// SentrySpan describes a Span following the Sentry format
type SentrySpan struct {
	TraceID      string            `json:"trace_id"`
	SpanID       string            `json:"span_id"`
	ParentSpanID string            `json:"parent_span_id,omitempty"`
	Description  string            `json:"description,omitempty"`
	Op           string            `json:"op,omitempty"`
	Tags         map[string]string `json:"tags,omitempty"`
	EndTimestamp time.Time         `json:"end_timestamp"`
	Timestamp    time.Time         `json:"timestamp"`
	Status       StatusCode        `json:"status"`
}

// TraceContext describes the context of the trace
type TraceContext struct {
	TraceID      string `json:"trace_id"`
	SpanID       string `json:"span_id"`
	ParentSpanID string `json:"parent_span_id,omitempty"`
}

// SentryTransaction describes a Sentry Transaction
// TODO: We should create a seperate transaction for each resource
// TODO: generate extra fields when sending event EventID, Type, User, Platform, SDK
// Check event interface for all details https://github.com/getsentry/relay/blob/90387f541cec341f9bee857bf88d4cd896fe4553/relay-general/src/protocol/event.rs#L207
type SentryTransaction struct {
	StartTimestamp time.Time            `json:"start_timestamp,omitempty"`
	Timestamp      time.Time            `json:"timestamp"`
	Contexts       TraceContext         `json:"contexts,omitempty"`
	Transaction    string               `json:"transaction,omitempty"`
	Tags           map[string]string    `json:"tags,omitempty"`
	Spans          []*SentrySpan        `json:"spans,omitempty"`
	Breadcrumbs    []*sentry.Breadcrumb `json:"breadcrumbs,omitempty"`
}

/*
{
	start_timestamp: ...
	timestamp: ...
	contexts: {
		trace_id: ...
		span_id: ...
		parent_span_id: ...
	}
	transaction: ...
	tags: []
	spans: []
	breadcrumbs: []
}
*/

/*
An application (hereby referred to as an Resource) has a set of InstrumentationLibrarySpans associated with it.
InstrumentationLibrarySpans refers to a collection of Spans produced by an InstrumentationLibrary.

We can say for Sentry, an InstrumentationLibrary produces an individual transaction, but

Traces: {
	ResourceSpans: [
		{
			InstrumentationLibrarySpans: [
				{
					Spans: [],
				},
			],
		},
	],
},

                                Application
+--------------------------------------------------------------------------+
|                         TracerProvider(Resource)                         |
|                         MeterProvider(Resource)                          |
|                                                                          |
|      Instrumentation Library 1           Instrumentation Library 2       |
|  +--------------------------------+  +--------------------------------+  |
|  | Tracer(InstrumentationLibrary) |  | Tracer(InstrumentationLibrary) |  |
|  | Meter(InstrumentationLibrary)  |  | Meter(InstrumentationLibrary)  |  |
|  +--------------------------------+  +--------------------------------+  |
|                                                                          |
|      Instrumentation Library 3           Instrumentation Library 4       |
|  +--------------------------------+  +--------------------------------+  |
|  | Tracer(InstrumentationLibrary) |  | Tracer(InstrumentationLibrary) |  |
|  | Meter(InstrumentationLibrary)  |  | Meter(InstrumentationLibrary)  |  |
|  +--------------------------------+  +--------------------------------+  |
|                                                                          |
+--------------------------------------------------------------------------+

For the Sentry Exporter, each InstrumentationLibrarySpan -> a Sentry transaction.
*/
