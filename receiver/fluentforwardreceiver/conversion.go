// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fluentforwardreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/fluentforwardreceiver"

import (
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"math"
	"strconv"
	"time"

	"github.com/tinylib/msgp/msgp"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
)

const (
	tagAttributeKey = "fluent.tag"

	// Fluent Forward record/option maps and batches are normally much smaller;
	// 64K keeps legitimate large batches possible while bounding forged
	// msgpack array/map headers before they can drive large allocations.
	maxMsgpackElements = 64 * 1024

	// Tags are identifiers and should be small; 128K preserves existing str32
	// handling without allowing mode detection to peek unbounded tag lengths.
	maxMsgpackTagBytes = 128 * 1024

	// Packed-forward payloads can carry batches, so allow larger raw payloads
	// while still rejecting forged str/bin lengths before allocation.
	maxMsgpackRawBytes = 16 * 1024 * 1024
)

// Most of this logic is derived directly from
// https://github.com/fluent/fluentd/wiki/Forward-Protocol-Specification-v1,
// which describes the fields in much greater detail.

type event interface {
	DecodeMsg(dc *msgp.Reader) error
	LogRecords() plog.LogRecordSlice
	Chunk() string
	Compressed() string
}

type optionsMap map[string]any

func setMsgpackReaderLimits(dc *msgp.Reader) {
	// msgp also uses MaxElements to cap bin/ext byte allocations.
	// Packed-forward payloads are the only place this receiver intentionally allows larger raw bytes.
	dc.SetMaxElements(maxMsgpackElements)
	dc.SetMaxStringLength(maxMsgpackRawBytes)
}

func validateMsgpackElementCount(count uint32) error {
	if count > maxMsgpackElements {
		return msgp.ErrLimitExceeded
	}
	return nil
}

func readPackedForwardPayload(dc *msgp.Reader, typ msgp.Type, maxBytes int) ([]byte, error) {
	if maxBytes <= 0 {
		maxBytes = maxMsgpackRawBytes
	}
	if uint64(maxBytes) > uint64(^uint32(0)) {
		return nil, fmt.Errorf("packed-forward payload limit %d exceeds MessagePack size limit", maxBytes)
	}

	switch typ {
	case msgp.StrType:
		prevMaxStringLength := dc.GetMaxStringLength()
		dc.SetMaxStringLength(uint64(maxBytes))
		defer dc.SetMaxStringLength(prevMaxStringLength)

		entriesStr, err := dc.ReadString()
		if err != nil {
			return nil, err
		}
		return []byte(entriesStr), nil
	case msgp.BinType:
		prevMaxElements := dc.GetMaxElements()
		dc.SetMaxElements(uint32(maxBytes))
		defer dc.SetMaxElements(prevMaxElements)
		return dc.ReadBytesLimit(nil, int64(maxBytes))
	default:
		return nil, fmt.Errorf("invalid type %d", typ)
	}
}

// Chunk returns the `chunk` option or blank string if it was not set.
func (om optionsMap) Chunk() string {
	c, _ := om["chunk"].(string)
	return c
}

func (om optionsMap) Compressed() string {
	compressed, _ := om["compressed"].(string)
	return compressed
}

type eventMode int

type peeker interface {
	Peek(n int) ([]byte, error)
}

// Values for enum eventMode.
const (
	unknownMode eventMode = iota
	messageMode
	forwardMode
	packedForwardMode
)

func (em eventMode) String() string {
	switch em {
	case unknownMode:
		return "unknown"
	case messageMode:
		return "message"
	case forwardMode:
		return "forward"
	case packedForwardMode:
		return "packedforward"
	default:
		panic("programmer bug")
	}
}

// parseInterfaceToMap takes map of interface objects and returns
// AttributeValueMap
func parseInterfaceToMap(msi map[string]any, dest pcommon.Value) {
	am := dest.SetEmptyMap()
	am.EnsureCapacity(len(msi))
	for k, value := range msi {
		parseToAttributeValue(value, am.PutEmpty(k))
	}
}

// parseInterfaceToArray takes array of interface objects and returns
// AttributeValueArray
func parseInterfaceToArray(ai []any, dest pcommon.Value) {
	av := dest.SetEmptySlice()
	av.EnsureCapacity(len(ai))
	for _, value := range ai {
		parseToAttributeValue(value, av.AppendEmpty())
	}
}

// parseToAttributeValue converts interface object to AttributeValue
func parseToAttributeValue(val any, dest pcommon.Value) {
	// See https://github.com/tinylib/msgp/wiki/Type-Mapping-Rules
	switch r := val.(type) {
	case bool:
		dest.SetBool(r)
	case string:
		dest.SetStr(r)
	case uint64:
		// handle overflow of uint64 to int64
		if r > math.MaxInt64 {
			dest.SetStr(strconv.FormatUint(r, 10))
			return
		}
		dest.SetInt(int64(r))
	case int64:
		dest.SetInt(r)
	// Sometimes strings come in as bytes array
	case []byte:
		dest.SetStr(string(r))
	case map[string]any:
		parseInterfaceToMap(r, dest)
	case []any:
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

func timeFromTimestamp(ts any) (time.Time, error) {
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
	if err := validateMsgpackElementCount(recordLen); err != nil {
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

type messageEventLogRecord struct {
	plog.LogRecordSlice
	optionsMap
}

func (melr *messageEventLogRecord) LogRecords() plog.LogRecordSlice {
	return melr.LogRecordSlice
}

func (melr *messageEventLogRecord) DecodeMsg(dc *msgp.Reader) error {
	setMsgpackReaderLimits(dc)

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

	log := melr.AppendEmpty()
	attrs := log.Attributes()
	attrs.PutStr(tagAttributeKey, tag)
	err = parseRecordToLogRecord(dc, log)
	if err != nil {
		return err
	}

	if arrLen == 4 {
		melr.optionsMap, err = parseOptions(dc)
		return err
	}
	return nil
}

func parseOptions(dc *msgp.Reader) (optionsMap, error) {
	var optionLen uint32
	optionLen, err := dc.ReadMapHeader()
	if err != nil {
		return nil, msgp.WrapError(err, "Option")
	}
	if err := validateMsgpackElementCount(optionLen); err != nil {
		return nil, msgp.WrapError(err, "Option")
	}
	out := make(optionsMap, optionLen)

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

type forwardEventLogRecords struct {
	plog.LogRecordSlice
	optionsMap
}

func (fe *forwardEventLogRecords) LogRecords() plog.LogRecordSlice {
	return fe.LogRecordSlice
}

func (fe *forwardEventLogRecords) DecodeMsg(dc *msgp.Reader) error {
	setMsgpackReaderLimits(dc)

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
	err = validateMsgpackElementCount(entryLen)
	if err != nil {
		return msgp.WrapError(err, "Record")
	}

	fe.EnsureCapacity(int(entryLen))
	for i := 0; i < int(entryLen); i++ {
		lr := fe.AppendEmpty()

		err = parseEntryToLogRecord(dc, lr)
		if err != nil {
			return msgp.WrapError(err, "Entries", i)
		}
		lr.Attributes().PutStr(tagAttributeKey, tag)
	}

	if arrLen == 3 {
		fe.optionsMap, err = parseOptions(dc)
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

type packedForwardEventLogRecords struct {
	plog.LogRecordSlice
	optionsMap
	maxRawBytes int
}

func (pfe *packedForwardEventLogRecords) LogRecords() plog.LogRecordSlice {
	return pfe.LogRecordSlice
}

// DecodeMsg implements msgp.Decodable.  This was originally code generated but
// then manually copied here in order to handle the optional Options field.
func (pfe *packedForwardEventLogRecords) DecodeMsg(dc *msgp.Reader) error {
	setMsgpackReaderLimits(dc)

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
	entriesRaw, err := readPackedForwardPayload(dc, entriesType, pfe.maxRawBytes)
	if err != nil {
		return msgp.WrapError(err, "EntriesRaw")
	}

	if arrLen == 3 {
		pfe.optionsMap, err = parseOptions(dc)
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

func (pfe *packedForwardEventLogRecords) parseEntries(entriesRaw []byte, isGzipped bool, tag string) error {
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
	setMsgpackReaderLimits(msgpReader)
	// Allocate only once, since the MoveTo cleans the lr, so we can reuse.
	lr := plog.NewLogRecord()
	var entries uint32
	for {
		err := parseEntryToLogRecord(msgpReader, lr)
		if err != nil {
			if errors.Is(msgp.Cause(err), io.EOF) {
				return nil
			}
			return err
		}
		if entries == maxMsgpackElements {
			return msgp.WrapError(msgp.ErrLimitExceeded, "Entries")
		}
		entries++
		lr.Attributes().PutStr(tagAttributeKey, tag)
		lr.MoveTo(pfe.AppendEmpty())
	}
}
