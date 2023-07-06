// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fluentforwardreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/fluentforwardreceiver"

import (
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/tinylib/msgp/msgp"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
)

const tagAttributeKey = "fluent.tag"

// Most of this logic is derived directly from
// https://github.com/fluent/fluentd/wiki/Forward-Protocol-Specification-v1,
// which describes the fields in much greater detail.

type Event interface {
	DecodeMsg(dc *msgp.Reader) error
	LogRecords() plog.LogRecordSlice
	Chunk() string
	Compressed() string
}

type OptionsMap map[string]interface{}

// Chunk returns the `chunk` option or blank string if it was not set.
func (om OptionsMap) Chunk() string {
	c, _ := om["chunk"].(string)
	return c
}

func (om OptionsMap) Compressed() string {
	compressed, _ := om["compressed"].(string)
	return compressed
}

type EventMode int

type Peeker interface {
	Peek(n int) ([]byte, error)
}

// Values for enum EventMode.
const (
	UnknownMode EventMode = iota
	MessageMode
	ForwardMode
	PackedForwardMode
)

func (em EventMode) String() string {
	switch em {
	case UnknownMode:
		return "unknown"
	case MessageMode:
		return "message"
	case ForwardMode:
		return "forward"
	case PackedForwardMode:
		return "packedforward"
	default:
		panic("programmer bug")
	}
}

// parseInterfaceToMap takes map of interface objects and returns
// AttributeValueMap
func parseInterfaceToMap(msi map[string]interface{}, dest pcommon.Value) {
	am := dest.SetEmptyMap()
	am.EnsureCapacity(len(msi))
	for k, value := range msi {
		parseToAttributeValue(value, am.PutEmpty(k))
	}
}

// parseInterfaceToArray takes array of interface objects and returns
// AttributeValueArray
func parseInterfaceToArray(ai []interface{}, dest pcommon.Value) {
	av := dest.SetEmptySlice()
	av.EnsureCapacity(len(ai))
	for _, value := range ai {
		parseToAttributeValue(value, av.AppendEmpty())
	}
}

// parseToAttributeValue converts interface object to AttributeValue
func parseToAttributeValue(val interface{}, dest pcommon.Value) {
	// See https://github.com/tinylib/msgp/wiki/Type-Mapping-Rules
	switch r := val.(type) {
	case bool:
		dest.SetBool(r)
	case string:
		dest.SetStr(r)
	case uint64:
		dest.SetInt(int64(r))
	case int64:
		dest.SetInt(r)
	// Sometimes strings come in as bytes array
	case []byte:
		dest.SetStr(string(r))
	case map[string]interface{}:
		parseInterfaceToMap(r, dest)
	case []interface{}:
		parseInterfaceToArray(r, dest)
	case float32:
		dest.SetDouble(float64(r))
	case float64:
		dest.SetDouble(r)
	case nil:
	default:
		dest.SetStr(fmt.Sprintf("%v", val))
	}
}

func timeFromTimestamp(ts interface{}) (time.Time, error) {
	switch v := ts.(type) {
	case uint64:
		return time.Unix(int64(v), 0), nil
	case int64:
		return time.Unix(v, 0), nil
	case *eventTimeExt:
		return time.Time(*v), nil
	default:
		return time.Time{}, fmt.Errorf("unknown type of value: %v", ts)
	}
}

func parseRecordToLogRecord(dc *msgp.Reader, lr plog.LogRecord) error {
	tsIntf, err := dc.ReadIntf()
	if err != nil {
		return msgp.WrapError(err, "Time")
	}

	ts, err := timeFromTimestamp(tsIntf)
	if err != nil {
		return msgp.WrapError(err, "Time")
	}

	lr.SetTimestamp(pcommon.NewTimestampFromTime(ts))

	recordLen, err := dc.ReadMapHeader()
	if err != nil {
		return msgp.WrapError(err, "Record")
	}

	for recordLen > 0 {
		recordLen--
		key, err := dc.ReadString()
		if err != nil {
			// The protocol doesn't specify this but apparently some map keys
			// can be binary type instead of string
			keyBytes, keyBytesErr := dc.ReadBytes(nil)
			if keyBytesErr != nil {
				return msgp.WrapError(keyBytesErr, "Record")
			}
			key = string(keyBytes)
		}
		val, err := dc.ReadIntf()
		if err != nil {
			return msgp.WrapError(err, "Record", key)
		}

		// fluentd uses message, fluentbit log.
		if key == "message" || key == "log" {
			parseToAttributeValue(val, lr.Body())
		} else {
			parseToAttributeValue(val, lr.Attributes().PutEmpty(key))
		}
	}

	return nil
}

type MessageEventLogRecord struct {
	plog.LogRecordSlice
	OptionsMap
}

func (melr *MessageEventLogRecord) LogRecords() plog.LogRecordSlice {
	return melr.LogRecordSlice
}

func (melr *MessageEventLogRecord) DecodeMsg(dc *msgp.Reader) error {
	melr.LogRecordSlice = plog.NewLogRecordSlice()
	var arrLen uint32
	var err error

	arrLen, err = dc.ReadArrayHeader()
	if err != nil {
		return err
	}
	if arrLen > 4 || arrLen < 3 {
		return msgp.ArrayError{Wanted: 3, Got: arrLen}
	}

	tag, err := dc.ReadString()
	if err != nil {
		return msgp.WrapError(err, "Tag")
	}

	log := melr.LogRecordSlice.AppendEmpty()
	attrs := log.Attributes()
	attrs.PutStr(tagAttributeKey, tag)
	err = parseRecordToLogRecord(dc, log)
	if err != nil {
		return err
	}

	if arrLen == 4 {
		melr.OptionsMap, err = parseOptions(dc)
		return err
	}
	return nil
}

func parseOptions(dc *msgp.Reader) (OptionsMap, error) {
	var optionLen uint32
	optionLen, err := dc.ReadMapHeader()
	if err != nil {
		return nil, msgp.WrapError(err, "Option")
	}
	out := make(OptionsMap, optionLen)

	for optionLen > 0 {
		optionLen--
		key, err := dc.ReadString()
		if err != nil {
			return nil, msgp.WrapError(err, "Option")
		}
		val, err := dc.ReadIntf()
		if err != nil {
			return nil, msgp.WrapError(err, "Option", key)
		}
		out[key] = val
	}
	return out, nil
}

type ForwardEventLogRecords struct {
	plog.LogRecordSlice
	OptionsMap
}

func (fe *ForwardEventLogRecords) LogRecords() plog.LogRecordSlice {
	return fe.LogRecordSlice
}

func (fe *ForwardEventLogRecords) DecodeMsg(dc *msgp.Reader) error {
	fe.LogRecordSlice = plog.NewLogRecordSlice()

	arrLen, err := dc.ReadArrayHeader()
	if err != nil {
		return err
	}
	if arrLen < 2 || arrLen > 3 {
		return msgp.ArrayError{Wanted: 2, Got: arrLen}
	}

	tag, err := dc.ReadString()
	if err != nil {
		return msgp.WrapError(err, "Tag")
	}

	entryLen, err := dc.ReadArrayHeader()
	if err != nil {
		return msgp.WrapError(err, "Record")
	}

	fe.LogRecordSlice.EnsureCapacity(int(entryLen))
	for i := 0; i < int(entryLen); i++ {
		lr := fe.LogRecordSlice.AppendEmpty()

		err = parseEntryToLogRecord(dc, lr)
		if err != nil {
			return msgp.WrapError(err, "Entries", i)
		}
		lr.Attributes().PutStr(tagAttributeKey, tag)
	}

	if arrLen == 3 {
		fe.OptionsMap, err = parseOptions(dc)
		return err
	}

	return nil
}

func parseEntryToLogRecord(dc *msgp.Reader, lr plog.LogRecord) error {
	arrLen, err := dc.ReadArrayHeader()
	if err != nil {
		return err
	}
	if arrLen != 2 {
		return msgp.ArrayError{Wanted: 2, Got: arrLen}
	}
	return parseRecordToLogRecord(dc, lr)
}

type PackedForwardEventLogRecords struct {
	plog.LogRecordSlice
	OptionsMap
}

func (pfe *PackedForwardEventLogRecords) LogRecords() plog.LogRecordSlice {
	return pfe.LogRecordSlice
}

// DecodeMsg implements msgp.Decodable.  This was originally code generated but
// then manually copied here in order to handle the optional Options field.
func (pfe *PackedForwardEventLogRecords) DecodeMsg(dc *msgp.Reader) error {
	pfe.LogRecordSlice = plog.NewLogRecordSlice()

	arrLen, err := dc.ReadArrayHeader()
	if err != nil {
		return err
	}
	if arrLen < 2 || arrLen > 3 {
		return msgp.ArrayError{Wanted: 2, Got: arrLen}
	}

	tag, err := dc.ReadString()
	if err != nil {
		return msgp.WrapError(err, "Tag")
	}

	entriesFirstByte, err := dc.R.Peek(1)
	if err != nil {
		return msgp.WrapError(err, "EntriesRaw")
	}

	entriesType := msgp.NextType(entriesFirstByte)
	// We have to read out the entries raw all the way first because we don't
	// know whether it is compressed or not until we read the options map which
	// comes after.  I guess we could use some kind of detection logic to
	// determine if it is gzipped by peeking and just ignoring options, but
	// this seems simpler for now.
	var entriesRaw []byte
	switch entriesType {
	case msgp.StrType:
		var entriesStr string
		entriesStr, err = dc.ReadString()
		if err != nil {
			return msgp.WrapError(err, "EntriesRaw")
		}
		entriesRaw = []byte(entriesStr)
	case msgp.BinType:
		entriesRaw, err = dc.ReadBytes(nil)
		if err != nil {
			return msgp.WrapError(err, "EntriesRaw")
		}
	case msgp.InvalidType, msgp.MapType, msgp.ArrayType, msgp.Float64Type, msgp.Float32Type, msgp.BoolType, msgp.IntType, msgp.UintType, msgp.NilType, msgp.DurationType, msgp.ExtensionType, msgp.Complex64Type, msgp.Complex128Type, msgp.TimeType:
		fallthrough
	default:
		return msgp.WrapError(fmt.Errorf("invalid type %d", entriesType), "EntriesRaw")
	}

	if arrLen == 3 {
		pfe.OptionsMap, err = parseOptions(dc)
		if err != nil {
			return err
		}
	}

	err = pfe.parseEntries(entriesRaw, pfe.Compressed() == "gzip", tag)
	if err != nil {
		return err
	}

	return nil
}

func (pfe *PackedForwardEventLogRecords) parseEntries(entriesRaw []byte, isGzipped bool, tag string) error {
	var reader io.Reader
	reader = bytes.NewReader(entriesRaw)

	if isGzipped {
		var err error
		reader, err = gzip.NewReader(reader)
		if err != nil {
			return err
		}
		defer reader.(*gzip.Reader).Close()
	}

	msgpReader := msgp.NewReader(reader)
	// Allocate only once, since the MoveTo cleans the lr, so we can reuse.
	lr := plog.NewLogRecord()
	for {
		err := parseEntryToLogRecord(msgpReader, lr)
		if err != nil {
			if errors.Is(msgp.Cause(err), io.EOF) {
				return nil
			}
			return err
		}
		lr.Attributes().PutStr(tagAttributeKey, tag)
		lr.MoveTo(pfe.LogRecordSlice.AppendEmpty())
	}
}
