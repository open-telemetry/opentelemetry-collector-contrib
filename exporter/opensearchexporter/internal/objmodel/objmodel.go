// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// The objmodel package provides tools for converting OpenTelemetry Log records into
// JSON documents.
//
// The JSON parsing in OpenSearch does not support parsing JSON documents
// with duplicate fields. The fields in the docuemt can be sort and duplicate entries
// can be removed before serializing. Deduplication ensures that ambigious
// events can still be indexed.
//
// With attributes map encoded as a list of key value
// pairs, we might find some structured loggers that create log records with
// duplicate fields. Although the AttributeMap wrapper tries to give a
// dictionary like view into the list, it is not 'complete'. When iterating the map
// for encoding, we still will encounter the duplicates.
// The AttributeMap helpers treat the first occurrence as the actual field.
// For high-performance structured loggers (e.g. zap) the AttributeMap
// semantics are not necessarily correct. Most often the last occurrence will be
// what we want to export, as the last occurrence represents the last overwrite
// within a context/dictionary (the leaf-logger its context).
// Some Loggers might even allow users to create a mix of dotted and dedotted fields.
// The Document type also tries to combine these into a proper structure, such that these mixed
// representations have a unique encoding only, which allows us to properly remove duplicates.
//
// The `.` is special to OpenSearch. In order to handle common prefixes and attributes
// being a mix of key value pairs with dots and complex objects, we flatten the document first
// before we deduplicate. Final dedotting is optional and only required when
// Ingest Node is used. But either way, we try to present only well-formed
// document to OpenSearch.

package objmodel // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/opensearchexporter/internal/objmodel"

import (
	"encoding/hex"
	"io"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/elastic/go-structform"
	"github.com/elastic/go-structform/json"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

// Document is an intermediate representation for converting open telemetry records with arbitrary attributes
// into a JSON document that can be processed by OpenSearch.
type Document struct {
	fields []field
}

type field struct {
	key   string
	value Value
}

// Value type that can be added to a Document.
type Value struct {
	kind      Kind
	primitive uint64
	dbl       float64
	str       string
	arr       []Value
	doc       Document
	ts        time.Time
}

// Kind represent the internal kind of a value stored in a Document.
type Kind uint8

// Enum values for Kind.
const (
	KindNil Kind = iota
	KindBool
	KindInt
	KindDouble
	KindString
	KindArr
	KindObject
	KindTimestamp
	KindIgnore
)

const tsLayout = "2006-01-02T15:04:05.000000000Z"

var nilValue = Value{kind: KindNil}
var ignoreValue = Value{kind: KindIgnore}

// DocumentFromAttributes creates a document from a OpenTelemetry attribute
// map. All nested maps will be flattened, with keys being joined using a `.` symbol.
func DocumentFromAttributes(am pcommon.Map) Document {
	return DocumentFromAttributesWithPath("", am)
}

// DocumentFromAttributesWithPath creates a document from a OpenTelemetry attribute
// map. All nested maps will be flattened, with keys being joined using a `.` symbol.
//
// All keys in the map will be prefixed with path.
func DocumentFromAttributesWithPath(path string, am pcommon.Map) Document {
	if am.Len() == 0 {
		return Document{}
	}

	fields := make([]field, 0, am.Len())
	fields = appendAttributeFields(fields, path, am)
	return Document{fields}
}

// AddTimestamp adds a raw timestamp value to the Document.
func (doc *Document) AddTimestamp(key string, ts pcommon.Timestamp) {
	doc.Add(key, TimestampValue(ts.AsTime()))
}

// Add adds a converted value to the document.
func (doc *Document) Add(key string, v Value) {
	doc.fields = append(doc.fields, field{key: key, value: v})
}

// AddString adds a string to the document.
func (doc *Document) AddString(key string, v string) {
	if v != "" {
		doc.Add(key, StringValue(v))
	}
}

// AddSpanID adds the hex presentation of a SpanID to the document. If the SpanID
// is empty, no value will be added.
func (doc *Document) AddSpanID(key string, id pcommon.SpanID) {
	if !id.IsEmpty() {
		doc.AddString(key, hex.EncodeToString(id[:]))
	}
}

// AddTraceID adds the hex presentation of a TraceID value to the document. If the TraceID
// is empty, no value will be added.
func (doc *Document) AddTraceID(key string, id pcommon.TraceID) {
	if !id.IsEmpty() {
		doc.AddString(key, hex.EncodeToString(id[:]))
	}
}

// AddInt adds an integer value to the document.
func (doc *Document) AddInt(key string, value int64) {
	doc.Add(key, IntValue(value))
}

// AddAttributes expands and flattens all key-value pairs from the input attribute map into
// the document.
func (doc *Document) AddAttributes(key string, attributes pcommon.Map) {
	doc.fields = appendAttributeFields(doc.fields, key, attributes)
}

// AddAttribute converts and adds a AttributeValue to the document. If the attribute represents a map,
// the fields will be flattened.
func (doc *Document) AddAttribute(key string, attribute pcommon.Value) {
	switch attribute.Type() {
	case pcommon.ValueTypeEmpty:
		// do not add 'null'
	case pcommon.ValueTypeMap:
		doc.AddAttributes(key, attribute.Map())
	default:
		doc.Add(key, ValueFromAttribute(attribute))
	}
}

// Sort sorts all fields in the document by key name.
func (doc *Document) Sort() {
	sort.SliceStable(doc.fields, func(i, j int) bool {
		return doc.fields[i].key < doc.fields[j].key
	})

	for i := range doc.fields {
		fld := &doc.fields[i]
		fld.value.Sort()
	}
}

// Dedup removes fields from the document, that have duplicate keys.
// The filtering only keeps the last value for a key.
//
// Dedup ensure that keys are sorted.
func (doc *Document) Dedup() {
	// 1. Always ensure the fields are sorted, Dedup support requires
	// Fields to be sorted.
	doc.Sort()

	// 2. rename fields if a primitive value is overwritten by an object.
	//    For example the pair (path.x=1, path.x.a="test") becomes:
	//    (path.x.value=1, path.x.a="test").
	//
	//    NOTE: We do the renaming, in order to preserve the original value
	//    in case of conflicts after dedotting, which would lead to the removal of the field.
	//    For example docker/k8s labels tend to use `.`, which need to be handled in case
	//    The collector does pass us these kind of labels as an AttributeMap.
	//
	//    NOTE: If the embedded document already has a field name `value`, we will remove the renamed
	//    field in favor of the `value` field in the document.
	//
	//    This step removes potential conflicts when dedotting and serializing fields.
	var renamed bool
	for i := 0; i < len(doc.fields)-1; i++ {
		key, nextKey := doc.fields[i].key, doc.fields[i+1].key
		if len(key) < len(nextKey) && strings.HasPrefix(nextKey, key) && nextKey[len(key)] == '.' {
			renamed = true
			doc.fields[i].key = key + ".value"
		}
	}
	if renamed {
		doc.Sort()
	}

	// 3. mark duplicates as 'ignore'
	//
	//    This step ensures that we do not have duplicate fields names when serializing.
	//    OpenSearch JSON parser will fail otherwise.
	for i := 0; i < len(doc.fields)-1; i++ {
		if doc.fields[i].key == doc.fields[i+1].key {
			doc.fields[i].value = ignoreValue
		}
	}

	// 4. fix objects that might be stored in arrays
	for i := range doc.fields {
		doc.fields[i].value.Dedup()
	}
}

// Serialize writes the document to the given writer. The serializer will create nested objects if dedot is true.
//
// NOTE: The documented MUST be sorted if dedot is true.
func (doc *Document) Serialize(w io.Writer, dedot bool) error {
	v := json.NewVisitor(w)
	return doc.iterJSON(v, dedot)
}

func (doc *Document) iterJSON(v *json.Visitor, dedot bool) error {
	if dedot {
		return doc.iterJSONDedot(v)
	}
	return doc.iterJSONFlat(v)
}

func (doc *Document) iterJSONFlat(w *json.Visitor) error {
	err := w.OnObjectStart(-1, structform.AnyType)
	if err != nil {
		return err
	}
	defer func() {
		_ = w.OnObjectFinished()
	}()

	for i := range doc.fields {
		fld := &doc.fields[i]
		if fld.value.IsEmpty() {
			continue
		}

		if err := w.OnKey(fld.key); err != nil {
			return err
		}

		if err := fld.value.iterJSON(w, true); err != nil {
			return err
		}
	}

	return nil
}

func (doc *Document) iterJSONDedot(w *json.Visitor) error {
	objPrefix := ""
	level := 0

	if err := w.OnObjectStart(-1, structform.AnyType); err != nil {
		return err
	}
	defer func() {
		_ = w.OnObjectFinished()
	}()

	for i := range doc.fields {
		fld := &doc.fields[i]
		if fld.value.IsEmpty() {
			continue
		}

		key := fld.key
		// decrease object level until last reported and current key have the same path prefix
		for L := commonObjPrefix(key, objPrefix); L < len(objPrefix); {
			for L > 0 && key[L-1] != '.' {
				L--
			}

			// remove levels and append write list of outstanding '}' into the writer
			if L > 0 {
				for delta := objPrefix[L:]; len(delta) > 0; {
					idx := strings.IndexByte(delta, '.')
					if idx < 0 {
						break
					}

					delta = delta[idx+1:]
					level--
					if err := w.OnObjectFinished(); err != nil {
						return err
					}
				}

				objPrefix = key[:L]
			} else { // no common prefix, close all objects we reported so far.
				for ; level > 0; level-- {
					if err := w.OnObjectFinished(); err != nil {
						return err
					}
				}
				objPrefix = ""
			}
		}

		// increase object level up to current field
		for {
			start := len(objPrefix)
			idx := strings.IndexByte(key[start:], '.')
			if idx < 0 {
				break
			}

			level++
			objPrefix = key[:len(objPrefix)+idx+1]
			fieldName := key[start : start+idx]
			if err := w.OnKey(fieldName); err != nil {
				return err
			}
			if err := w.OnObjectStart(-1, structform.AnyType); err != nil {
				return err
			}
		}

		// report value
		fieldName := key[len(objPrefix):]
		if err := w.OnKey(fieldName); err != nil {
			return err
		}
		if err := fld.value.iterJSON(w, true); err != nil {
			return err
		}
	}

	// close all pending object levels
	for ; level > 0; level-- {
		if err := w.OnObjectFinished(); err != nil {
			return err
		}
	}

	return nil
}

// StringValue create a new value from a string.
func StringValue(str string) Value { return Value{kind: KindString, str: str} }

// IntValue creates a new value from an integer.
func IntValue(i int64) Value { return Value{kind: KindInt, primitive: uint64(i)} }

// DoubleValue creates a new value from a double value..
func DoubleValue(d float64) Value { return Value{kind: KindDouble, dbl: d} }

// BoolValue creates a new value from a double value..
func BoolValue(b bool) Value {
	var v uint64
	if b {
		v = 1
	}
	return Value{kind: KindBool, primitive: v}
}

// ArrValue combines multiple values into an array value.
func ArrValue(values ...Value) Value {
	return Value{kind: KindArr, arr: values}
}

// TimestampValue create a new value from a time.Time.
func TimestampValue(ts time.Time) Value {
	return Value{kind: KindTimestamp, ts: ts}
}

// ValueFromAttribute converts a AttributeValue into a value.
func ValueFromAttribute(attr pcommon.Value) Value {
	switch attr.Type() {
	case pcommon.ValueTypeInt:
		return IntValue(attr.Int())
	case pcommon.ValueTypeDouble:
		return DoubleValue(attr.Double())
	case pcommon.ValueTypeStr:
		return StringValue(attr.Str())
	case pcommon.ValueTypeBool:
		return BoolValue(attr.Bool())
	case pcommon.ValueTypeSlice:
		sub := arrFromAttributes(attr.Slice())
		return ArrValue(sub...)
	case pcommon.ValueTypeMap:
		sub := DocumentFromAttributes(attr.Map())
		return Value{kind: KindObject, doc: sub}
	default:
		return nilValue
	}
}

// Sort recursively sorts all keys in docuemts held by the value.
func (v *Value) Sort() {
	switch v.kind {
	case KindObject:
		v.doc.Sort()
	case KindArr:
		for i := range v.arr {
			v.arr[i].Sort()
		}
	}
}

// Dedup recursively dedups keys in stored documents.
//
// NOTE: The value MUST be sorted.
func (v *Value) Dedup() {
	switch v.kind {
	case KindObject:
		v.doc.Dedup()
	case KindArr:
		for i := range v.arr {
			v.arr[i].Dedup()
		}
	}
}

func (v *Value) IsEmpty() bool {
	switch v.kind {
	case KindNil, KindIgnore:
		return true
	case KindArr:
		return len(v.arr) == 0
	case KindObject:
		return len(v.doc.fields) == 0
	default:
		return false
	}
}

func (v *Value) iterJSON(w *json.Visitor, dedot bool) error {
	switch v.kind {
	case KindNil:
		return w.OnNil()
	case KindBool:
		return w.OnBool(v.primitive == 1)
	case KindInt:
		return w.OnInt64(int64(v.primitive))
	case KindDouble:
		if math.IsNaN(v.dbl) || math.IsInf(v.dbl, 0) {
			// NaN and Inf are undefined for JSON. Let's serialize to "null"
			return w.OnNil()
		}
		return w.OnFloat64(v.dbl)
	case KindString:
		return w.OnString(v.str)
	case KindTimestamp:
		str := v.ts.UTC().Format(tsLayout)
		return w.OnString(str)
	case KindObject:
		if len(v.doc.fields) == 0 {
			return w.OnNil()
		}
		return v.doc.iterJSON(w, dedot)
	case KindArr:
		if err := w.OnArrayStart(-1, structform.AnyType); err != nil {
			return err
		}
		for i := range v.arr {
			if err := v.arr[i].iterJSON(w, dedot); err != nil {
				return err
			}
		}
		if err := w.OnArrayFinished(); err != nil {
			return err
		}
	}

	return nil
}

func arrFromAttributes(aa pcommon.Slice) []Value {
	if aa.Len() == 0 {
		return nil
	}

	values := make([]Value, aa.Len())
	for i := 0; i < aa.Len(); i++ {
		values[i] = ValueFromAttribute(aa.At(i))
	}
	return values
}

func appendAttributeFields(fields []field, path string, am pcommon.Map) []field {
	am.Range(func(k string, val pcommon.Value) bool {
		fields = appendAttributeValue(fields, path, k, val)
		return true
	})
	return fields
}

func appendAttributeValue(fields []field, path string, key string, attr pcommon.Value) []field {
	if attr.Type() == pcommon.ValueTypeEmpty {
		return fields
	}

	if attr.Type() == pcommon.ValueTypeMap {
		return appendAttributeFields(fields, flattenKey(path, key), attr.Map())
	}

	return append(fields, field{
		key:   flattenKey(path, key),
		value: ValueFromAttribute(attr),
	})
}

func flattenKey(path, key string) string {
	if path == "" {
		return key
	}
	return path + "." + key
}

func commonObjPrefix(a, b string) int {
	end := len(a)
	if alt := len(b); alt < end {
		end = alt
	}

	for i := 0; i < end; i++ {
		if a[i] != b[i] {
			return i
		}
	}
	return end
}
