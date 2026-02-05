// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package textencodingextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/textencodingextension"

import (
	"bufio"
	"bytes"
	"io"
	"regexp"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	txt "golang.org/x/text/encoding"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/xstream"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/textutils"
)

type textLogCodec struct {
	decoder               *txt.Decoder
	marshalingSeparator   string
	unmarshalingSeparator *regexp.Regexp
}

func (r *textLogCodec) UnmarshalLogs(buf []byte) (plog.Logs, error) {
	decoder, err := r.NewLogsDecoder(bytes.NewReader(buf))
	if err != nil {
		return plog.Logs{}, err
	}

	logs, err := decoder.DecodeLogs()
	if err != nil {
		return plog.Logs{}, err
	}

	return logs, nil
}

func (r *textLogCodec) NewLogsDecoder(reader io.Reader, options ...encoding.DecoderOption) (encoding.LogsDecoder, error) {
	s := bufio.NewScanner(reader)
	if r.unmarshalingSeparator != nil {
		s.Split(func(data []byte, atEOF bool) (advance int, token []byte, err error) {
			if atEOF && len(data) == 0 {
				return 0, nil, nil
			}
			if loc := r.unmarshalingSeparator.FindIndex(data); len(loc) > 0 && loc[0] >= 0 {
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
			if atEOF {
				return len(data), data, nil
			}
			return 0, nil, nil // Request more data until EOF
		})
	}

	batchHelper := xstream.NewBatchHelper(options...)
	return xstream.LogsDecoderFunc(func() (plog.Logs, error) {
		p := plog.NewLogs()
		now := pcommon.NewTimestampFromTime(time.Now())

		for s.Scan() {
			l := p.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
			l.SetObservedTimestamp(now)

			b := s.Bytes()
			decoded, err := textutils.DecodeAsString(r.decoder, b)
			if err != nil {
				return p, err
			}
			l.Body().SetStr(decoded)

			batchHelper.IncrementItems(1)
			batchHelper.IncrementBytes(int64(len(b)))
			if batchHelper.ShouldFlush() {
				batchHelper.Reset()
				return p, nil
			}
		}

		if err := s.Err(); err != nil {
			return p, err
		}

		// check for stream EOF which results in empty log batch
		if p.LogRecordCount() == 0 {
			return p, io.EOF
		}

		return p, nil
	}), nil
}

func (r *textLogCodec) MarshalLogs(ld plog.Logs) ([]byte, error) {
	var b []byte
	appendedLogRecord := false

	for i := 0; i < ld.ResourceLogs().Len(); i++ {
		rl := ld.ResourceLogs().At(i)
		for j := 0; j < rl.ScopeLogs().Len(); j++ {
			sl := rl.ScopeLogs().At(j)
			for k := 0; k < sl.LogRecords().Len(); k++ {
				lr := sl.LogRecords().At(k)
				if appendedLogRecord {
					b = append(b, []byte(r.marshalingSeparator)...)
				}
				b = append(b, []byte(lr.Body().AsString())...)
				appendedLogRecord = true
			}
		}
	}
	return b, nil
}
