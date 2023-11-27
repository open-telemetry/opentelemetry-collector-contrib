// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/lokireceiver/internal"

import (
	"io"
	"sort"
	"strconv"
	"strings"
	"time"
	"unsafe"

	"github.com/buger/jsonparser"
	"github.com/grafana/loki/pkg/push"
	jsoniter "github.com/json-iterator/go"
)

// PushRequest models a log stream push but is unmarshalled to proto push format.
type PushRequest struct {
	Streams []Stream `json:"streams"`
}

// Stream helps with unmarshalling of each log stream for push request.
type Stream push.Stream

func (s *Stream) UnmarshalJSON(data []byte) error {
	err := jsonparser.ObjectEach(data, func(key, val []byte, ty jsonparser.ValueType, _ int) error {
		switch string(key) {
		case "stream":
			var labels LabelSet
			if err := labels.UnmarshalJSON(val); err != nil {
				return err
			}
			s.Labels = labels.String()
		case "values":
			if ty == jsonparser.Null {
				return nil
			}
			entries, err := unmarshalHTTPToLogProtoEntries(val)
			if err != nil {
				return err
			}
			s.Entries = entries
		}
		return nil
	})
	return err
}

func unmarshalHTTPToLogProtoEntries(data []byte) ([]push.Entry, error) {
	var (
		entries    []push.Entry
		parseError error
	)
	if _, err := jsonparser.ArrayEach(data, func(value []byte, ty jsonparser.ValueType, _ int, err error) {
		if err != nil || parseError != nil {
			return
		}
		if ty == jsonparser.Null {
			return
		}
		e, err := unmarshalHTTPToLogProtoEntry(value)
		if err != nil {
			parseError = err
			return
		}
		entries = append(entries, e)
	}); err != nil {
		parseError = err
	}

	if parseError != nil {
		return nil, parseError
	}

	return entries, nil
}

func unmarshalHTTPToLogProtoEntry(data []byte) (push.Entry, error) {
	var (
		i          int
		parseError error
		e          push.Entry
	)
	_, err := jsonparser.ArrayEach(data, func(value []byte, t jsonparser.ValueType, _ int, _ error) {
		// assert that both items in array are of type string
		if (i == 0 || i == 1) && t != jsonparser.String {
			parseError = jsonparser.MalformedStringError
			return
		} else if i == 2 && t != jsonparser.Object {
			parseError = jsonparser.MalformedObjectError
			return
		}
		switch i {
		case 0: // timestamp
			ts, err := jsonparser.ParseInt(value)
			if err != nil {
				parseError = err
				return
			}
			e.Timestamp = time.Unix(0, ts)
		case 1: // value
			v, err := jsonparser.ParseString(value)
			if err != nil {
				parseError = err
				return
			}
			e.Line = v
		case 2: // structuredMetadata
			var structuredMetadata []push.LabelAdapter
			err := jsonparser.ObjectEach(value, func(key, val []byte, dataType jsonparser.ValueType, _ int) error {
				if dataType != jsonparser.String {
					return jsonparser.MalformedStringError
				}
				structuredMetadata = append(structuredMetadata, push.LabelAdapter{
					Name:  string(key),
					Value: string(val),
				})
				return nil
			})
			if err != nil {
				parseError = err
				return
			}
			e.StructuredMetadata = structuredMetadata
		}
		i++
	})
	if parseError != nil {
		return e, parseError
	}
	return e, err
}

// LabelSet is a key/value pair mapping of labels
type LabelSet map[string]string

func (l *LabelSet) UnmarshalJSON(data []byte) error {
	if *l == nil {
		*l = make(LabelSet)
	}
	return jsonparser.ObjectEach(data, func(key, val []byte, _ jsonparser.ValueType, _ int) error {
		v, err := jsonparser.ParseString(val)
		if err != nil {
			return err
		}
		k, err := jsonparser.ParseString(key)
		if err != nil {
			return err
		}
		(*l)[k] = v
		return nil
	})
}

// String implements the Stringer interface. It returns a formatted/sorted set of label key/value pairs.
func (l LabelSet) String() string {
	var b strings.Builder

	keys := make([]string, 0, len(l))
	for k := range l {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	b.WriteByte('{')
	for i, k := range keys {
		if i > 0 {
			b.WriteByte(',')
			b.WriteByte(' ')
		}
		b.WriteString(k)
		b.WriteByte('=')
		b.WriteString(strconv.Quote(l[k]))
	}
	b.WriteByte('}')
	return b.String()
}

// decodePushRequest directly decodes json to a push.PushRequest
func decodePushRequest(b io.Reader, r *push.PushRequest) error {
	var request PushRequest

	if err := jsoniter.NewDecoder(b).Decode(&request); err != nil {
		return err
	}
	*r = push.PushRequest{
		Streams: *(*[]push.Stream)(unsafe.Pointer(&request.Streams)),
	}

	return nil
}
