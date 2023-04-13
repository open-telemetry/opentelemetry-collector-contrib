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

// PushRequest models a log stream push
type PushRequest struct {
	Streams []*Stream `json:"streams"`
}

// Stream represents a log stream.  It includes a set of log entries and their labels.
type Stream struct {
	Labels  LabelSet `json:"stream"`
	Entries []Entry  `json:"values"`
}

func (s *Stream) UnmarshalJSON(data []byte) error {
	if s.Labels == nil {
		s.Labels = LabelSet{}
	}
	if len(s.Entries) > 0 {
		s.Entries = s.Entries[:0]
	}
	return jsonparser.ObjectEach(data, func(key, value []byte, ty jsonparser.ValueType, _ int) error {
		switch string(key) {
		case "stream":
			if err := s.Labels.UnmarshalJSON(value); err != nil {
				return err
			}
		case "values":
			if ty == jsonparser.Null {
				return nil
			}
			var parseError error
			_, err := jsonparser.ArrayEach(value, func(value []byte, ty jsonparser.ValueType, _ int, _ error) {
				if ty == jsonparser.Null {
					return
				}
				var entry Entry
				if err := entry.UnmarshalJSON(value); err != nil {
					parseError = err
					return
				}
				s.Entries = append(s.Entries, entry)
			})
			if parseError != nil {
				return parseError
			}
			return err
		}
		return nil
	})
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

// Entry represents a log entry. It includes a log message and the time it occurred at.
type Entry struct {
	Timestamp time.Time
	Line      string
}

func (e *Entry) UnmarshalJSON(data []byte) error {
	var (
		i          int
		parseError error
	)
	_, err := jsonparser.ArrayEach(data, func(value []byte, t jsonparser.ValueType, _ int, _ error) {
		// assert that both items in array are of type string
		if t != jsonparser.String {
			parseError = jsonparser.MalformedStringError
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
		}
		i++
	})
	if parseError != nil {
		return parseError
	}
	return err
}

// decodePushRequest directly decodes json to a push.PushRequest
func decodePushRequest(b io.Reader, r *push.PushRequest) error {
	var request PushRequest

	if err := jsoniter.NewDecoder(b).Decode(&request); err != nil {
		return err
	}
	*r = newPushRequest(request)

	return nil
}

// newPushRequest constructs a push.PushRequest from a PushRequest
func newPushRequest(r PushRequest) push.PushRequest {
	ret := push.PushRequest{
		Streams: make([]push.Stream, len(r.Streams)),
	}

	for i, s := range r.Streams {
		ret.Streams[i] = newStream(s)
	}

	return ret
}

// newStream constructs a push.Stream from a Stream
func newStream(s *Stream) push.Stream {
	return push.Stream{
		Entries: *(*[]push.Entry)(unsafe.Pointer(&s.Entries)),
		Labels:  s.Labels.String(),
	}
}
