// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package googlecloudlogentryencodingextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/googlecloudlogentryencodingextension"

import (
	"bytes"
	stdjson "encoding/json"
	"errors"
	"fmt"
	"strconv"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/known/anypb"
)

func setBodyFromProto(logRecord plog.LogRecord, value stdjson.RawMessage) error {
	err := translateInto(logRecord.Body().SetEmptyMap(), (&anypb.Any{}).ProtoReflect().Descriptor(), value)
	return err
}

func (opts fieldTranslateOptions) translateValue(dst pcommon.Value, fd protoreflect.FieldDescriptor, src stdjson.RawMessage) error {
	var err error
	switch fd.Kind() {
	case protoreflect.MessageKind:
		msg := fd.Message()
		switch fd.Message().FullName() {
		case "google.protobuf.Duration", "google.protobuf.Timestamp":
			// protojson represents both of these as strings
			return translateStr(dst, src)
		case "google.protobuf.Struct", "google.protobuf.Value":
			return translateRaw(dst, src)
		case
			"google.protobuf.BoolValue",
			"google.protobuf.BytesValue",
			"google.protobuf.DoubleValue",
			"google.protobuf.FloatValue",
			"google.protobuf.Int32Value",
			"google.protobuf.Int64Value",
			"google.protobuf.StringValue",
			"google.protobuf.UInt32Value",
			"google.protobuf.UInt64Value":
			// All the wrapper types have a single field with name
			// `value` and field number 1, and are represented in
			// protojson without the wrapping.
			innerFd := fd.Message().Fields().ByNumber(1)
			_ = opts.translateValue(dst, innerFd, src)
		default:
			var m pcommon.Map
			switch dst.Type() {
			case pcommon.ValueTypeMap:
				m = dst.Map()
			default:
				m = dst.SetEmptyMap()
			}
			return translateInto(m, msg, src)
		}
	case protoreflect.EnumKind:
		// protojson accepts either string name or enum int value; try both.
		if translateStr(dst, src) == nil {
			return nil
		}

		enum := fd.Enum()
		var i int32
		if err = json.Unmarshal(src, &i); err != nil {
			return fmt.Errorf("wrong type for enum: %v", getTokenType(src))
		}
		enumValue := enum.Values().ByNumber(protoreflect.EnumNumber(i))
		if enumValue == nil {
			return fmt.Errorf("%v has no enum value for %v", enum.FullName(), i)
		}

		dst.SetStr(string(enumValue.Name()))
	case protoreflect.BoolKind:
		var val bool
		err = json.Unmarshal(src, &val)
		if err != nil {
			return err
		}
		dst.SetBool(val)
	case protoreflect.Int32Kind,
		protoreflect.Uint32Kind,
		protoreflect.Sfixed32Kind,
		protoreflect.Fixed32Kind,
		protoreflect.Sint32Kind,
		protoreflect.Int64Kind,
		protoreflect.Uint64Kind,
		protoreflect.Sfixed64Kind,
		protoreflect.Fixed64Kind,
		protoreflect.Sint64Kind:
		// The protojson encoding accepts either string or number for
		// integer types, so try both.
		var val int64
		if json.Unmarshal(src, &val) == nil {
			dst.SetInt(val)
			return nil
		}

		var s string
		if err = json.Unmarshal(src, &s); err != nil {
			return err
		}
		if val, err = strconv.ParseInt(s, 10, 64); err != nil {
			return err
		}
		dst.SetInt(val)
		return nil
	case protoreflect.FloatKind, protoreflect.DoubleKind:
		var val float64
		err := json.Unmarshal(src, &val)
		if err != nil {
			return err
		}
		dst.SetDouble(val)
		return nil
	case protoreflect.BytesKind:
		var val []byte
		err := json.Unmarshal(src, &val)
		if err != nil {
			return err
		}
		dst.SetEmptyBytes().Append(val...)
		return nil
	case protoreflect.StringKind:
		return translateStr(dst, src)
	default:
		return errors.New("unknown field kind")
	}
	return nil
}

func (opts fieldTranslateOptions) translateList(dst pcommon.Slice, fd protoreflect.FieldDescriptor, src stdjson.RawMessage) error {
	var msg []stdjson.RawMessage
	if err := json.Unmarshal(src, &msg); err != nil {
		return err
	}

	for _, v := range msg {
		err := opts.translateValue(dst.AppendEmpty(), fd, v)
		if err != nil {
			return err
		}
	}
	return nil
}

func (opts fieldTranslateOptions) translateMap(dst pcommon.Map, fd protoreflect.FieldDescriptor, src stdjson.RawMessage) error {
	var msg map[string]stdjson.RawMessage
	if err := json.Unmarshal(src, &msg); err != nil {
		return err
	}
	for k, v := range msg {
		err := opts.translateValue(dst.PutEmpty(k), fd.MapValue(), v)
		if err != nil {
			return err
		}
	}
	return nil
}

func translateAny(dst pcommon.Map, src map[string]stdjson.RawMessage) error {
	// protojson represents Any as the JSON representation of the actual
	// message, plus a special @type field containing the type URL of the
	// message.
	typeURL, ok := src["@type"]
	if !ok {
		return errors.New("no @type member in Any message")
	}
	var typeURLStr string
	if err := json.Unmarshal(typeURL, &typeURLStr); err != nil {
		return err
	}
	delete(src, "@type")

	msgType, err := protoregistry.GlobalTypes.FindMessageByURL(typeURLStr)
	if errors.Is(err, protoregistry.NotFound) {
		// If we don't have the type, we do a best-effort JSON decode;
		// some ints might be floats or strings.
		for k, v := range src {
			var val any
			err = json.Unmarshal(v, &val)
			if err != nil {
				return nil
			}
			_ = dst.PutEmpty(k).FromRaw(val)
		}
		return nil
	}

	err = translateInto(dst, msgType.Descriptor(), src)
	if err != nil {
		return err
	}

	dst.PutStr("@type", typeURLStr)
	return nil
}

func translateProtoMessage(dst pcommon.Map, desc protoreflect.MessageDescriptor, src map[string]stdjson.RawMessage, opts fieldTranslateOptions) error {
	if !opts.preserveDst {
		dst.Clear()
	}

	// Handle well-known aggregate types.
	switch desc.FullName() {
	case "google.protobuf.Any":
		return translateAny(dst, src)
	case "google.protobuf.Struct":
		for k, v := range src {
			var val any
			if err := json.Unmarshal(v, &val); err != nil {
				return err
			}
			_ = dst.PutEmpty(k).FromRaw(val)
		}
		return nil
	case "google.protobuf.Empty":
		dst.Clear()
		return nil
	default:
	}

	for k, v := range src {
		key := opts.mapKey(k)
		fd := desc.Fields().ByJSONName(k)
		if fd == nil {
			return fmt.Errorf("%v has no known field with JSON name %v", desc.FullName(), k)
		}
		var err error
		switch {
		case fd.IsList():
			err = opts.translateList(dst.PutEmptySlice(key), fd, v)
		case fd.IsMap():
			err = opts.translateMap(dst.PutEmptyMap(key), fd, v)
		default:
			err = opts.translateValue(dst.PutEmpty(key), fd, v)
		}
		if err != nil {
			return err
		}
	}

	return nil
}

func translateInto(dst pcommon.Map, desc protoreflect.MessageDescriptor, src any, opts ...fieldTranslateFn) error {
	var toTranslate map[string]stdjson.RawMessage

	switch msg := src.(type) {
	case stdjson.RawMessage:
		err := json.Unmarshal(msg, &toTranslate)
		if err != nil {
			return err
		}
	case map[string]stdjson.RawMessage:
		toTranslate = msg
	}

	options := fieldTranslateOptions{}
	for _, opt := range opts {
		opt(&options)
	}

	return translateProtoMessage(dst, desc, toTranslate, options)
}

func translateStr(dst pcommon.Value, src stdjson.RawMessage) error {
	var val string
	err := json.Unmarshal(src, &val)
	if err != nil {
		return err
	}
	dst.SetStr(val)
	return nil
}

func translateRaw(dst pcommon.Value, src stdjson.RawMessage) error {
	var val any
	err := json.Unmarshal(src, &val)
	if err != nil {
		return err
	}
	_ = dst.FromRaw(val)
	return nil
}

func getTokenType(src stdjson.RawMessage) string {
	dec := stdjson.NewDecoder(bytes.NewReader(src))
	tok, err := dec.Token()
	if err != nil {
		return "invalid json"
	}
	switch t := tok.(type) {
	case stdjson.Delim:
		switch t {
		case '[':
			return "array"
		case '{':
			return "object"
		default:
			return "invalid json"
		}
	case bool:
		return "bool"
	case float64, stdjson.Number:
		return "number"
	case string:
		return "string"
	case nil:
		return "null"
	default:
		return "unknown"
	}
}
