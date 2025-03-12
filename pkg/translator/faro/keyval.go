// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package faro // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/faro"

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	faroTypes "github.com/grafana/faro/pkg/go"
	om "github.com/wk8/go-ordered-map"
)

// KeyVal is an ordered map of string to interface
type KeyVal = om.OrderedMap

// NewKeyVal creates new empty KeyVal
func NewKeyVal() *KeyVal {
	return om.New()
}

// KeyValFromMap will instantiate KeyVal from a map[string]string
func KeyValFromMap(m map[string]string) *KeyVal {
	kv := NewKeyVal()
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		KeyValAdd(kv, k, m[k])
	}
	return kv
}

// KeyValFromMap will instantiate KeyVal from a map[string]float64
func KeyValFromFloatMap(m map[string]float64) *KeyVal {
	kv := NewKeyVal()
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		kv.Set(k, m[k])
	}
	return kv
}

// MergeKeyVal will merge source in target
func MergeKeyVal(target *KeyVal, source *KeyVal) {
	for el := source.Oldest(); el != nil; el = el.Next() {
		target.Set(el.Key, el.Value)
	}
}

// MergeKeyValWithPrefix will merge source in target, adding a prefix to each key being merged in
func MergeKeyValWithPrefix(target *KeyVal, source *KeyVal, prefix string) {
	for el := source.Oldest(); el != nil; el = el.Next() {
		target.Set(fmt.Sprintf("%s%s", prefix, el.Key), el.Value)
	}
}

// KeyValAdd adds a key + value string pair to kv
func KeyValAdd(kv *KeyVal, key string, value string) {
	if len(value) > 0 {
		kv.Set(key, value)
	}
}

// KeyValToInterfaceSlice converts KeyVal to []interface{}, typically used for logging
func KeyValToInterfaceSlice(kv *KeyVal) []any {
	slice := make([]any, kv.Len()*2)
	idx := 0
	for el := kv.Oldest(); el != nil; el = el.Next() {
		slice[idx] = el.Key
		idx++
		slice[idx] = el.Value
		idx++
	}
	return slice
}

// KeyValToInterfaceMap converts KeyVal to map[string]interface
func KeyValToInterfaceMap(kv *KeyVal) map[string]any {
	retv := make(map[string]any)
	for el := kv.Oldest(); el != nil; el = el.Next() {
		retv[fmt.Sprint(el.Key)] = el.Value
	}
	return retv
}

// LogToKeyVal represents a Log object as KeyVal
func LogToKeyVal(l faroTypes.Log) *KeyVal {
	kv := NewKeyVal()
	KeyValAdd(kv, "timestamp", l.Timestamp.Format(string(faroTypes.TimeFormatRFC3339Milli)))
	KeyValAdd(kv, "kind", string(faroTypes.KindLog))
	KeyValAdd(kv, "message", l.Message)
	KeyValAdd(kv, "level", string(l.LogLevel))
	MergeKeyValWithPrefix(kv, KeyValFromMap(l.Context), "context_")
	MergeKeyVal(kv, TraceToKeyVal(l.Trace))
	return kv
}

// ExceptionToKeyVal represents an Exception object as KeyVal
func ExceptionToKeyVal(e faroTypes.Exception) *KeyVal {
	kv := NewKeyVal()
	KeyValAdd(kv, "timestamp", e.Timestamp.Format(string(faroTypes.TimeFormatRFC3339Milli)))
	KeyValAdd(kv, "kind", string(faroTypes.KindException))
	KeyValAdd(kv, "type", e.Type)
	KeyValAdd(kv, "value", e.Value)
	KeyValAdd(kv, "stacktrace", ExceptionToString(e))
	MergeKeyVal(kv, TraceToKeyVal(e.Trace))
	MergeKeyValWithPrefix(kv, KeyValFromMap(e.Context), "context_")
	return kv
}

// ExceptionMessage string is concatenating of the Exception.Type and Exception.Value
func ExceptionMessage(e faroTypes.Exception) string {
	return fmt.Sprintf("%s: %s", e.Type, e.Value)
}

// ExceptionToString is the string representation of an Exception
func ExceptionToString(e faroTypes.Exception) string {
	stacktrace := ExceptionMessage(e)
	if e.Stacktrace != nil {
		for _, frame := range e.Stacktrace.Frames {
			stacktrace += FrameToString(frame)
		}
	}
	return stacktrace
}

// FrameToString function converts a Frame into a human readable string
func FrameToString(frame faroTypes.Frame) string {
	module := ""
	if len(frame.Module) > 0 {
		module = frame.Module + "|"
	}
	return fmt.Sprintf("\n  at %s (%s%s:%v:%v)", frame.Function, module, frame.Filename, frame.Lineno, frame.Colno)
}

// MeasurementToKeyVal representation of the measurement object
func MeasurementToKeyVal(m faroTypes.Measurement) *KeyVal {
	kv := NewKeyVal()

	KeyValAdd(kv, "timestamp", m.Timestamp.Format(string(faroTypes.TimeFormatRFC3339Milli)))
	KeyValAdd(kv, "kind", string(faroTypes.KindMeasurement))
	KeyValAdd(kv, "type", m.Type)
	MergeKeyValWithPrefix(kv, KeyValFromMap(m.Context), "context_")

	keys := make([]string, 0, len(m.Values))
	for k := range m.Values {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		KeyValAdd(kv, k, fmt.Sprintf("%f", m.Values[k]))
	}
	MergeKeyVal(kv, TraceToKeyVal(m.Trace))

	values := make(map[string]float64, len(m.Values))
	for key, value := range m.Values {
		values[key] = value
	}

	MergeKeyValWithPrefix(kv, KeyValFromFloatMap(values), "value_")

	return kv
}

// EventToKeyVal produces key -> value representation of Event metadata
func EventToKeyVal(e faroTypes.Event) *KeyVal {
	kv := NewKeyVal()
	KeyValAdd(kv, "timestamp", e.Timestamp.Format(string(faroTypes.TimeFormatRFC3339Milli)))
	KeyValAdd(kv, "kind", string(faroTypes.KindEvent))
	KeyValAdd(kv, "event_name", e.Name)
	KeyValAdd(kv, "event_domain", e.Domain)
	if e.Attributes != nil {
		MergeKeyValWithPrefix(kv, KeyValFromMap(e.Attributes), "event_data_")
	}
	MergeKeyVal(kv, TraceToKeyVal(e.Trace))
	return kv
}

// MetaToKeyVal produces key->value representation of the metadata
func MetaToKeyVal(m faroTypes.Meta) *KeyVal {
	kv := NewKeyVal()
	MergeKeyValWithPrefix(kv, SDKToKeyVal(m.SDK), "sdk_")
	MergeKeyValWithPrefix(kv, AppToKeyVal(m.App), "app_")
	MergeKeyValWithPrefix(kv, UserToKeyVal(m.User), "user_")
	MergeKeyValWithPrefix(kv, SessionToKeyVal(m.Session), "session_")
	MergeKeyValWithPrefix(kv, PageToKeyVal(m.Page), "page_")
	MergeKeyValWithPrefix(kv, BrowserToKeyVal(m.Browser), "browser_")
	MergeKeyValWithPrefix(kv, K6ToKeyVal(m.K6), "k6_")
	MergeKeyValWithPrefix(kv, ViewToKeyVal(m.View), "view_")
	MergeKeyValWithPrefix(kv, GeoToKeyVal(m.Geo), "geo_")
	return kv
}

// SDKToKeyVal produces key->value representation of Sdk metadata
func SDKToKeyVal(sdk faroTypes.SDK) *KeyVal {
	kv := NewKeyVal()
	KeyValAdd(kv, "name", sdk.Name)
	KeyValAdd(kv, "version", sdk.Version)

	if len(sdk.Integrations) > 0 {
		integrations := make([]string, len(sdk.Integrations))

		for i, integration := range sdk.Integrations {
			integrations[i] = SDKIntegrationToString(integration)
		}

		KeyValAdd(kv, "integrations", strings.Join(integrations, ","))
	}

	return kv
}

// SDKIntegrationToString is the string representation of an SDKIntegration
func SDKIntegrationToString(i faroTypes.SDKIntegration) string {
	return fmt.Sprintf("%s:%s", i.Name, i.Version)
}

// AppToKeyVal produces key-> value representation of App metadata
func AppToKeyVal(a faroTypes.App) *KeyVal {
	kv := NewKeyVal()
	KeyValAdd(kv, "name", a.Name)
	KeyValAdd(kv, "namespace", a.Namespace)
	KeyValAdd(kv, "release", a.Release)
	KeyValAdd(kv, "version", a.Version)
	KeyValAdd(kv, "environment", a.Environment)
	return kv
}

// UserToKeyVal produces a key->value representation User metadata
func UserToKeyVal(u faroTypes.User) *KeyVal {
	kv := NewKeyVal()
	KeyValAdd(kv, "email", u.Email)
	KeyValAdd(kv, "id", u.ID)
	KeyValAdd(kv, "username", u.Username)
	MergeKeyValWithPrefix(kv, KeyValFromMap(u.Attributes), "attr_")
	return kv
}

// SessionToKeyVal produces key->value representation of the Session metadata
func SessionToKeyVal(s faroTypes.Session) *KeyVal {
	kv := NewKeyVal()
	KeyValAdd(kv, "id", s.ID)
	MergeKeyValWithPrefix(kv, KeyValFromMap(s.Attributes), "attr_")
	return kv
}

// PageToKeyVal produces key->val representation of Page metadata
func PageToKeyVal(p faroTypes.Page) *KeyVal {
	kv := NewKeyVal()
	KeyValAdd(kv, "id", p.ID)
	KeyValAdd(kv, "url", p.URL)
	MergeKeyValWithPrefix(kv, KeyValFromMap(p.Attributes), "attr_")

	return kv
}

// BrowserToKeyVal produces key->value representation of the Browser metadata
func BrowserToKeyVal(b faroTypes.Browser) *KeyVal {
	kv := NewKeyVal()
	KeyValAdd(kv, "name", b.Name)
	KeyValAdd(kv, "version", b.Version)
	KeyValAdd(kv, "os", b.OS)
	KeyValAdd(kv, "mobile", fmt.Sprintf("%v", b.Mobile))
	KeyValAdd(kv, "userAgent", b.UserAgent)
	KeyValAdd(kv, "language", b.Language)
	KeyValAdd(kv, "viewportWidth", b.ViewportWidth)
	KeyValAdd(kv, "viewportHeight", b.ViewportHeight)

	if brandsArray, err := b.Brands.AsBrandsArray(); err == nil {
		for i, brand := range brandsArray {
			MergeKeyValWithPrefix(kv, BrandToKeyVal(brand), fmt.Sprintf("brand_%d_", i))
		}
		return kv
	}

	if brandsString, err := b.Brands.AsBrandsString(); err == nil {
		KeyValAdd(kv, "brands", brandsString)
		return kv
	}

	return kv
}

func BrandToKeyVal(b faroTypes.Brand) *KeyVal {
	kv := NewKeyVal()
	KeyValAdd(kv, "brand", b.Brand)
	KeyValAdd(kv, "version", b.Version)
	return kv
}

// K6ToKeyVal produces a key->value representation K6 metadata
func K6ToKeyVal(k faroTypes.K6) *KeyVal {
	kv := NewKeyVal()
	if k.IsK6Browser {
		KeyValAdd(kv, "isK6Browser", strconv.FormatBool(k.IsK6Browser))
	}
	return kv
}

// ViewToKeyVal produces a key->value representation View metadata
func ViewToKeyVal(v faroTypes.View) *KeyVal {
	kv := NewKeyVal()
	KeyValAdd(kv, "name", v.Name)
	return kv
}

// GeoToKeyVal produces a key->value representation Geo metadata
func GeoToKeyVal(g faroTypes.Geo) *KeyVal {
	kv := NewKeyVal()
	KeyValAdd(kv, "continent_iso", g.ContinentISOCode)
	KeyValAdd(kv, "country_iso", g.CountryISOCode)
	KeyValAdd(kv, "subdivision_iso", g.SubdivisionISO)
	KeyValAdd(kv, "city", g.City)
	KeyValAdd(kv, "asn_org", g.ASNOrg)
	KeyValAdd(kv, "asn_id", g.ASNID)
	return kv
}

// TraceToKeyVal produces a key->value representation of the trace context object
func TraceToKeyVal(tc faroTypes.TraceContext) *KeyVal {
	kv := NewKeyVal()
	KeyValAdd(kv, "traceID", tc.TraceID)
	KeyValAdd(kv, "spanID", tc.SpanID)
	return kv
}
