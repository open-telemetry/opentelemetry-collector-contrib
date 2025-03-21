// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package faro // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/faro"

import (
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"

	faroTypes "github.com/grafana/faro/pkg/go"
	om "github.com/wk8/go-ordered-map"
)

// keyVal is an ordered map of string to interface
type keyVal = om.OrderedMap

// newKeyVal creates new empty keyVal
func newKeyVal() *keyVal {
	return om.New()
}

// keyValFromMap will instantiate keyVal from a map[string]string
func keyValFromMap(m map[string]string) *keyVal {
	kv := newKeyVal()
	for _, k := range slices.Sorted(maps.Keys(m)) {
		keyValAdd(kv, k, m[k])
	}
	return kv
}

// keyValFromFloatMap will instantiate keyVal from a map[string]float64
func keyValFromFloatMap(m map[string]float64) *keyVal {
	kv := newKeyVal()
	for _, k := range slices.Sorted(maps.Keys(m)) {
		kv.Set(k, m[k])
	}
	return kv
}

// mergeKeyVal will merge source in target
func mergeKeyVal(target *keyVal, source *keyVal) {
	for el := source.Oldest(); el != nil; el = el.Next() {
		target.Set(el.Key, el.Value)
	}
}

// mergeKeyValWithPrefix will merge source in target, adding a prefix to each key being merged in
func mergeKeyValWithPrefix(target *keyVal, source *keyVal, prefix string) {
	for el := source.Oldest(); el != nil; el = el.Next() {
		target.Set(fmt.Sprintf("%s%s", prefix, el.Key), el.Value)
	}
}

// keyValAdd adds a key + value string pair to kv
func keyValAdd(kv *keyVal, key string, value string) {
	if len(value) > 0 {
		kv.Set(key, value)
	}
}

// keyValToInterfaceSlice converts keyVal to []interface{}, typically used for logging
func keyValToInterfaceSlice(kv *keyVal) []any {
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

// logToKeyVal represents a Log object as keyVal
func logToKeyVal(l faroTypes.Log) *keyVal {
	kv := newKeyVal()
	keyValAdd(kv, "timestamp", l.Timestamp.Format(string(faroTypes.TimeFormatRFC3339Milli)))
	keyValAdd(kv, "kind", string(faroTypes.KindLog))
	keyValAdd(kv, "message", l.Message)
	keyValAdd(kv, "level", string(l.LogLevel))
	mergeKeyValWithPrefix(kv, keyValFromMap(l.Context), "context_")
	mergeKeyVal(kv, traceToKeyVal(l.Trace))
	return kv
}

// exceptionToKeyVal represents an Exception object as keyVal
func exceptionToKeyVal(e faroTypes.Exception) *keyVal {
	kv := newKeyVal()
	keyValAdd(kv, "timestamp", e.Timestamp.Format(string(faroTypes.TimeFormatRFC3339Milli)))
	keyValAdd(kv, "kind", string(faroTypes.KindException))
	keyValAdd(kv, "type", e.Type)
	keyValAdd(kv, "value", e.Value)
	keyValAdd(kv, "stacktrace", exceptionToString(e))
	mergeKeyVal(kv, traceToKeyVal(e.Trace))
	mergeKeyValWithPrefix(kv, keyValFromMap(e.Context), "context_")
	return kv
}

// exceptionMessage string is concatenating of the Exception.Type and Exception.Value
func exceptionMessage(e faroTypes.Exception) string {
	return fmt.Sprintf("%s: %s", e.Type, e.Value)
}

// exceptionToString is the string representation of an Exception
func exceptionToString(e faroTypes.Exception) string {
	stacktrace := exceptionMessage(e)
	if e.Stacktrace != nil {
		for _, frame := range e.Stacktrace.Frames {
			stacktrace += frameToString(frame)
		}
	}
	return stacktrace
}

// frameToString function converts a Frame into a human readable string
func frameToString(frame faroTypes.Frame) string {
	module := ""
	if len(frame.Module) > 0 {
		module = frame.Module + "|"
	}
	return fmt.Sprintf("\n  at %s (%s%s:%v:%v)", frame.Function, module, frame.Filename, frame.Lineno, frame.Colno)
}

// measurementToKeyVal representation of the measurement object
func measurementToKeyVal(m faroTypes.Measurement) *keyVal {
	kv := newKeyVal()

	keyValAdd(kv, "timestamp", m.Timestamp.Format(string(faroTypes.TimeFormatRFC3339Milli)))
	keyValAdd(kv, "kind", string(faroTypes.KindMeasurement))
	keyValAdd(kv, "type", m.Type)
	mergeKeyValWithPrefix(kv, keyValFromMap(m.Context), "context_")

	for _, k := range slices.Sorted(maps.Keys(m.Values)) {
		keyValAdd(kv, k, fmt.Sprintf("%f", m.Values[k]))
	}
	mergeKeyVal(kv, traceToKeyVal(m.Trace))

	values := make(map[string]float64, len(m.Values))
	for key, value := range m.Values {
		values[key] = value
	}

	mergeKeyValWithPrefix(kv, keyValFromFloatMap(values), "value_")

	return kv
}

// eventToKeyVal produces key -> value representation of Event metadata
func eventToKeyVal(e faroTypes.Event) *keyVal {
	kv := newKeyVal()
	keyValAdd(kv, "timestamp", e.Timestamp.Format(string(faroTypes.TimeFormatRFC3339Milli)))
	keyValAdd(kv, "kind", string(faroTypes.KindEvent))
	keyValAdd(kv, "event_name", e.Name)
	keyValAdd(kv, "event_domain", e.Domain)
	if e.Attributes != nil {
		mergeKeyValWithPrefix(kv, keyValFromMap(e.Attributes), "event_data_")
	}
	mergeKeyVal(kv, traceToKeyVal(e.Trace))
	return kv
}

// MetaToKeyVal produces key->value representation of the metadata
func MetaToKeyVal(m faroTypes.Meta) *keyVal {
	kv := newKeyVal()
	mergeKeyValWithPrefix(kv, sdkToKeyVal(m.SDK), "sdk_")
	mergeKeyValWithPrefix(kv, appToKeyVal(m.App), "app_")
	mergeKeyValWithPrefix(kv, userToKeyVal(m.User), "user_")
	mergeKeyValWithPrefix(kv, sessionToKeyVal(m.Session), "session_")
	mergeKeyValWithPrefix(kv, pageToKeyVal(m.Page), "page_")
	mergeKeyValWithPrefix(kv, browserToKeyVal(m.Browser), "browser_")
	mergeKeyValWithPrefix(kv, k6ToKeyVal(m.K6), "k6_")
	mergeKeyValWithPrefix(kv, viewToKeyVal(m.View), "view_")
	mergeKeyValWithPrefix(kv, geoToKeyVal(m.Geo), "geo_")
	return kv
}

// sdkToKeyVal produces key->value representation of Sdk metadata
func sdkToKeyVal(sdk faroTypes.SDK) *keyVal {
	kv := newKeyVal()
	keyValAdd(kv, "name", sdk.Name)
	keyValAdd(kv, "version", sdk.Version)

	if len(sdk.Integrations) > 0 {
		integrations := make([]string, len(sdk.Integrations))

		for i, integration := range sdk.Integrations {
			integrations[i] = sdkIntegrationToString(integration)
		}

		keyValAdd(kv, "integrations", strings.Join(integrations, ","))
	}

	return kv
}

// sdkIntegrationToString is the string representation of an SDKIntegration
func sdkIntegrationToString(i faroTypes.SDKIntegration) string {
	return fmt.Sprintf("%s:%s", i.Name, i.Version)
}

// appToKeyVal produces key-> value representation of App metadata
func appToKeyVal(a faroTypes.App) *keyVal {
	kv := newKeyVal()
	keyValAdd(kv, "name", a.Name)
	keyValAdd(kv, "namespace", a.Namespace)
	keyValAdd(kv, "release", a.Release)
	keyValAdd(kv, "version", a.Version)
	keyValAdd(kv, "environment", a.Environment)
	return kv
}

// userToKeyVal produces a key->value representation User metadata
func userToKeyVal(u faroTypes.User) *keyVal {
	kv := newKeyVal()
	keyValAdd(kv, "email", u.Email)
	keyValAdd(kv, "id", u.ID)
	keyValAdd(kv, "username", u.Username)
	mergeKeyValWithPrefix(kv, keyValFromMap(u.Attributes), "attr_")
	return kv
}

// sessionToKeyVal produces key->value representation of the Session metadata
func sessionToKeyVal(s faroTypes.Session) *keyVal {
	kv := newKeyVal()
	keyValAdd(kv, "id", s.ID)
	mergeKeyValWithPrefix(kv, keyValFromMap(s.Attributes), "attr_")
	return kv
}

// pageToKeyVal produces key->val representation of Page metadata
func pageToKeyVal(p faroTypes.Page) *keyVal {
	kv := newKeyVal()
	keyValAdd(kv, "id", p.ID)
	keyValAdd(kv, "url", p.URL)
	mergeKeyValWithPrefix(kv, keyValFromMap(p.Attributes), "attr_")

	return kv
}

// browserToKeyVal produces key->value representation of the Browser metadata
func browserToKeyVal(b faroTypes.Browser) *keyVal {
	kv := newKeyVal()
	keyValAdd(kv, "name", b.Name)
	keyValAdd(kv, "version", b.Version)
	keyValAdd(kv, "os", b.OS)
	keyValAdd(kv, "mobile", fmt.Sprintf("%v", b.Mobile))
	keyValAdd(kv, "userAgent", b.UserAgent)
	keyValAdd(kv, "language", b.Language)
	keyValAdd(kv, "viewportWidth", b.ViewportWidth)
	keyValAdd(kv, "viewportHeight", b.ViewportHeight)

	if brandsArray, err := b.Brands.AsBrandsArray(); err == nil {
		for i, brand := range brandsArray {
			mergeKeyValWithPrefix(kv, brandToKeyVal(brand), fmt.Sprintf("brand_%d_", i))
		}
		return kv
	}

	if brandsString, err := b.Brands.AsBrandsString(); err == nil {
		keyValAdd(kv, "brands", brandsString)
		return kv
	}

	return kv
}

func brandToKeyVal(b faroTypes.Brand) *keyVal {
	kv := newKeyVal()
	keyValAdd(kv, "brand", b.Brand)
	keyValAdd(kv, "version", b.Version)
	return kv
}

// k6ToKeyVal produces a key->value representation K6 metadata
func k6ToKeyVal(k faroTypes.K6) *keyVal {
	kv := newKeyVal()
	if k.IsK6Browser {
		keyValAdd(kv, "isK6Browser", strconv.FormatBool(k.IsK6Browser))
	}
	return kv
}

// viewToKeyVal produces a key->value representation View metadata
func viewToKeyVal(v faroTypes.View) *keyVal {
	kv := newKeyVal()
	keyValAdd(kv, "name", v.Name)
	return kv
}

// geoToKeyVal produces a key->value representation Geo metadata
func geoToKeyVal(g faroTypes.Geo) *keyVal {
	kv := newKeyVal()
	keyValAdd(kv, "continent_iso", g.ContinentISOCode)
	keyValAdd(kv, "country_iso", g.CountryISOCode)
	keyValAdd(kv, "subdivision_iso", g.SubdivisionISO)
	keyValAdd(kv, "city", g.City)
	keyValAdd(kv, "asn_org", g.ASNOrg)
	keyValAdd(kv, "asn_id", g.ASNID)
	return kv
}

// traceToKeyVal produces a key->value representation of the trace context object
func traceToKeyVal(tc faroTypes.TraceContext) *keyVal {
	kv := newKeyVal()
	keyValAdd(kv, "traceID", tc.TraceID)
	keyValAdd(kv, "spanID", tc.SpanID)
	return kv
}
