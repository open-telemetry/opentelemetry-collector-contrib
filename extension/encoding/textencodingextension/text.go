// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package textencodingextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/textencodingextension"

import (
	"bufio"
	"bytes"
	"regexp"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/textutils"
)

type textLogCodec struct {
	enc                   *textutils.Encoding
	marshalingSeparator   string
	unmarshalingSeparator *regexp.Regexp
}

func (r *textLogCodec) UnmarshalLogs(buf []byte) (plog.Logs, error) {
	p := plog.NewLogs()
	now := pcommon.NewTimestampFromTime(time.Now())

	s := bufio.NewScanner(bytes.NewReader(buf))
	if r.unmarshalingSeparator != nil {
		s.Split(func(data []byte, atEOF bool) (advance int, token []byte, err error) {
			if atEOF && len(data) == 0 {
				return 0, nil, nil
			}
			if loc := r.unmarshalingSeparator.FindIndex(data); loc != nil && loc[0] >= 0 {
				return loc[1], data[0:loc[0]], nil
			}
			if atEOF {
				return len(data), data, nil
			}
			return 0, nil, nil
		})
	} else {
		s.Split(func(data []byte, atEOF bool) (advance int, token []byte, err error) {
			if atEOF && len(data) == 0 {
				return 0, nil, nil
			}
			return len(data), data, nil
		})
	}
	for s.Scan() {
		l := p.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
		l.SetObservedTimestamp(now)
		decoded, err := r.enc.Decode(s.Bytes())
		if err != nil {
			return p, err
		}
		l.Body().SetStr(string(decoded))
	}

	return p, nil
}

func (r *textLogCodec) MarshalLogs(ld plog.Logs) ([]byte, error) {
	var b []byte
	for i := 0; i < ld.ResourceLogs().Len(); i++ {
		rl := ld.ResourceLogs().At(i)
		for j := 0; j < rl.ScopeLogs().Len(); j++ {
			sl := rl.ScopeLogs().At(j)
			for k := 0; k < sl.LogRecords().Len(); k++ {
				lr := sl.LogRecords().At(k)
				b = append(b, []byte(lr.Body().AsString())...)
				b = append(b, []byte(r.marshalingSeparator)...)
			}
		}
	}
	return b, nil
}
