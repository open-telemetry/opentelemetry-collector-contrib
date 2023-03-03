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

package faroreceiver

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"go.opentelemetry.io/collector/pdata/ptrace"
)

type Kind string

const (
	KindException   Kind = "exception"
	KindMeasurement Kind = "measurement"
	KindLog              = "log"
	KindEvent            = "event"
)

// Payload is the body of the receiver request
type Payload struct {
	Exceptions   []Exception   `json:"exceptions,omitempty"`
	Logs         []Log         `json:"logs,omitempty"`
	Measurements []Measurement `json:"measurements,omitempty"`
	Events       []Event       `json:"events,omitempty"`
	Meta         Meta          `json:"meta,omitempty"`
	Traces       *Traces       `json:"traces,omitempty"`
}

// Frame struct represents a single stacktrace frame
type Frame struct {
	Function string `json:"function,omitempty"`
	Module   string `json:"module,omitempty"`
	Filename string `json:"filename,omitempty"`
	Lineno   int    `json:"lineno,omitempty"`
	Colno    int    `json:"colno,omitempty"`
}

// String function converts a Frame into a human readable string
func (frame Frame) String() string {
	module := ""
	if len(frame.Module) > 0 {
		module = frame.Module + "|"
	}
	return fmt.Sprintf("\n  at %s (%s%s:%v:%v)", frame.Function, module, frame.Filename, frame.Lineno, frame.Colno)
}

// Stacktrace is a collection of Frames
type Stacktrace struct {
	Frames []Frame `json:"frames,omitempty"`
}

// Exception struct controls all the data regarding an exception
type Exception struct {
	Type       string       `json:"type,omitempty"`
	Value      string       `json:"value,omitempty"`
	Stacktrace *Stacktrace  `json:"stacktrace,omitempty"`
	Timestamp  time.Time    `json:"timestamp"`
	Trace      TraceContext `json:"trace,omitempty"`
}

// Message string is concatenating of the Exception.Type and Exception.Value
func (e Exception) Message() string {
	return fmt.Sprintf("%s: %s", e.Type, e.Value)
}

// String is the string representation of an Exception
func (e Exception) String() string {
	var stacktrace = e.Message()
	if e.Stacktrace != nil {
		for _, frame := range e.Stacktrace.Frames {
			stacktrace += frame.String()
		}
	}
	return stacktrace
}

// KeyVal representation of the exception object
func (e Exception) KeyVal() *KeyVal {
	kv := NewKeyVal()
	KeyValAdd(kv, "timestamp", e.Timestamp.String())
	KeyValAdd(kv, "kind", string(KindException))
	KeyValAdd(kv, "type", e.Type)
	KeyValAdd(kv, "value", e.Value)
	KeyValAdd(kv, "stacktrace", e.String())
	MergeKeyVal(kv, e.Trace.KeyVal())
	return kv
}

// TraceContext holds trace id and span id associated to an entity (log, exception, measurement...).
type TraceContext struct {
	TraceID string `json:"trace_id"`
	SpanID  string `json:"span_id"`
}

// KeyVal representation of the trace context object.
func (tc TraceContext) KeyVal() *KeyVal {
	retv := NewKeyVal()
	KeyValAdd(retv, "traceID", tc.TraceID)
	KeyValAdd(retv, "spanID", tc.SpanID)
	return retv
}

// Traces wraps the otel traces model.
type Traces struct {
	ptrace.Traces
}

// UnmarshalJSON unmarshals Traces model.
func (t *Traces) UnmarshalJSON(b []byte) error {
	unmarshaler := &ptrace.JSONUnmarshaler{}
	td, err := unmarshaler.UnmarshalTraces(b)
	if err != nil {
		return err
	}
	*t = Traces{td}
	return nil
}

// MarshalJSON marshals Traces model to json.
func (t Traces) MarshalJSON() ([]byte, error) {
	marshaler := &ptrace.JSONMarshaler{}
	return marshaler.MarshalTraces(t.Traces)
}

// LogLevel is log level enum for incoming app logs
type LogLevel string

const (
	// LogLevelTrace is "trace"
	LogLevelTrace LogLevel = "trace"
	// LogLevelDebug is "debug"
	LogLevelDebug LogLevel = "debug"
	// LogLevelInfo is "info"
	LogLevelInfo LogLevel = "info"
	// LogLevelWarning is "warning"
	LogLevelWarning LogLevel = "warning"
	// LogLevelError is "error"
	LogLevelError LogLevel = "error"
)

// LogContext is a string to string map structure that
// represents the context of a log message
type LogContext map[string]string

// Log struct controls the data that come into a Log message
type Log struct {
	Message   string       `json:"message,omitempty"`
	LogLevel  LogLevel     `json:"level,omitempty"`
	Context   LogContext   `json:"context,omitempty"`
	Timestamp time.Time    `json:"timestamp"`
	Trace     TraceContext `json:"trace,omitempty"`
}

// KeyVal representation of a Log object
func (l Log) KeyVal() *KeyVal {
	kv := NewKeyVal()
	KeyValAdd(kv, "timestamp", l.Timestamp.String())
	KeyValAdd(kv, "kind", string(KindLog))
	KeyValAdd(kv, "message", l.Message)
	KeyValAdd(kv, "level", string(l.LogLevel))
	MergeKeyValWithPrefix(kv, KeyValFromMap(l.Context), "context_")
	MergeKeyVal(kv, l.Trace.KeyVal())
	return kv
}

// Measurement holds the data for user provided measurements
type Measurement struct {
	Values    map[string]float64 `json:"values,omitempty"`
	Timestamp time.Time          `json:"timestamp,omitempty"`
	Trace     TraceContext       `json:"trace,omitempty"`
}

// KeyVal representation of the exception object
func (m Measurement) KeyVal() *KeyVal {
	kv := NewKeyVal()

	KeyValAdd(kv, "timestamp", m.Timestamp.String())
	KeyValAdd(kv, "kind", string(KindMeasurement))

	keys := make([]string, 0, len(m.Values))
	for k := range m.Values {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		KeyValAdd(kv, k, fmt.Sprintf("%f", m.Values[k]))
	}
	MergeKeyVal(kv, m.Trace.KeyVal())
	return kv
}

// SDK holds metadata about the app agent that produced the event
type SDK struct {
	Name         string           `json:"name,omitempty"`
	Version      string           `json:"version,omitempty"`
	Integrations []SDKIntegration `json:"integrations,omitempty"`
}

// KeyVal produces key->value representation of Sdk metadata
func (sdk SDK) KeyVal() *KeyVal {
	kv := NewKeyVal()
	KeyValAdd(kv, "name", sdk.Name)
	KeyValAdd(kv, "version", sdk.Version)

	if len(sdk.Integrations) > 0 {
		integrations := make([]string, len(sdk.Integrations))

		for i, integration := range sdk.Integrations {
			integrations[i] = integration.String()
		}

		KeyValAdd(kv, "integrations", strings.Join(integrations, ","))
	}

	return kv
}

// SDKIntegration holds metadata about a plugin/integration on the app agent that collected and sent the event
type SDKIntegration struct {
	Name    string `json:"name,omitempty"`
	Version string `json:"version,omitempty"`
}

func (i SDKIntegration) String() string {
	return fmt.Sprintf("%s:%s", i.Name, i.Version)
}

// User holds metadata about the user related to an app event
type User struct {
	Email      string            `json:"email,omitempty"`
	ID         string            `json:"id,omitempty"`
	Username   string            `json:"username,omitempty"`
	Attributes map[string]string `json:"attributes,omitempty"`
}

// KeyVal produces a key->value representation User metadata
func (u User) KeyVal() *KeyVal {
	kv := NewKeyVal()
	KeyValAdd(kv, "email", u.Email)
	KeyValAdd(kv, "id", u.ID)
	KeyValAdd(kv, "username", u.Username)
	MergeKeyValWithPrefix(kv, KeyValFromMap(u.Attributes), "attr_")
	return kv
}

// Meta holds metadata about an app event
type Meta struct {
	SDK     SDK     `json:"sdk,omitempty"`
	App     App     `json:"app,omitempty"`
	User    User    `json:"user,omitempty"`
	Session Session `json:"session,omitempty"`
	Page    Page    `json:"page,omitempty"`
	Browser Browser `json:"browser,omitempty"`
	View    View    `json:"view,omitempty"`
}

// KeyVal produces key->value representation of the app event metadata
func (m Meta) KeyVal() *KeyVal {
	kv := NewKeyVal()
	MergeKeyValWithPrefix(kv, m.SDK.KeyVal(), "sdk_")
	MergeKeyValWithPrefix(kv, m.App.KeyVal(), "app_")
	MergeKeyValWithPrefix(kv, m.User.KeyVal(), "user_")
	MergeKeyValWithPrefix(kv, m.Session.KeyVal(), "session_")
	MergeKeyValWithPrefix(kv, m.Page.KeyVal(), "page_")
	MergeKeyValWithPrefix(kv, m.Browser.KeyVal(), "browser_")
	MergeKeyValWithPrefix(kv, m.View.KeyVal(), "view_")
	return kv
}

// Session holds metadata about the browser session the event originates from
type Session struct {
	ID         string            `json:"id,omitempty"`
	Attributes map[string]string `json:"attributes,omitempty"`
}

// KeyVal produces key->value representation of the Session metadata
func (s Session) KeyVal() *KeyVal {
	kv := NewKeyVal()
	KeyValAdd(kv, "id", s.ID)
	MergeKeyValWithPrefix(kv, KeyValFromMap(s.Attributes), "attr_")
	return kv
}

// Page holds metadata about the web page event originates from
type Page struct {
	ID         string            `json:"id,omitempty"`
	URL        string            `json:"url,omitempty"`
	Attributes map[string]string `json:"attributes,omitempty"`
}

// KeyVal produces key->val representation of Page metadata
func (p Page) KeyVal() *KeyVal {
	kv := NewKeyVal()
	KeyValAdd(kv, "id", p.ID)
	KeyValAdd(kv, "url", p.URL)
	MergeKeyValWithPrefix(kv, KeyValFromMap(p.Attributes), "attr_")
	return kv
}

// App holds metadata about the application event originates from
type App struct {
	Name        string `json:"name,omitempty"`
	Release     string `json:"release,omitempty"`
	Version     string `json:"version,omitempty"`
	Environment string `json:"environment,omitempty"`
}

// Event holds RUM event data
type Event struct {
	Name       string            `json:"name"`
	Domain     string            `json:"domain,omitempty"`
	Attributes map[string]string `json:"attributes,omitempty"`
	Timestamp  time.Time         `json:"timestamp,omitempty"`
	Trace      TraceContext      `json:"trace,omitempty"`
}

// KeyVal produces key -> value representation of Event metadata
func (e Event) KeyVal() *KeyVal {
	kv := NewKeyVal()
	KeyValAdd(kv, "timestamp", e.Timestamp.String())
	KeyValAdd(kv, "kind", string(KindEvent))
	KeyValAdd(kv, "event_name", e.Name)
	KeyValAdd(kv, "event_domain", e.Domain)
	if e.Attributes != nil {
		MergeKeyValWithPrefix(kv, KeyValFromMap(e.Attributes), "event_data_")
	}
	MergeKeyVal(kv, e.Trace.KeyVal())
	return kv
}

// KeyVal produces key-> value representation of App metadata
func (a App) KeyVal() *KeyVal {
	kv := NewKeyVal()
	KeyValAdd(kv, "name", a.Name)
	KeyValAdd(kv, "release", a.Release)
	KeyValAdd(kv, "version", a.Version)
	KeyValAdd(kv, "environment", a.Environment)
	return kv
}

// Browser holds metadata about a client's browser
type Browser struct {
	Name    string `json:"name,omitempty"`
	Version string `json:"version,omitempty"`
	OS      string `json:"os,omitempty"`
	Mobile  bool   `json:"mobile,omitempty"`
}

// KeyVal produces key->value representation of the Browser metadata
func (b Browser) KeyVal() *KeyVal {
	kv := NewKeyVal()
	KeyValAdd(kv, "name", b.Name)
	KeyValAdd(kv, "version", b.Version)
	KeyValAdd(kv, "os", b.OS)
	KeyValAdd(kv, "mobile", fmt.Sprintf("%v", b.Mobile))
	return kv
}

// View holds metadata about a view
type View struct {
	Name string `json:"name,omitempty"`
}

func (v View) KeyVal() *KeyVal {
	kv := NewKeyVal()
	KeyValAdd(kv, "name", v.Name)
	return kv
}
