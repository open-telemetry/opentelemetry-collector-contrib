// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package kafkareceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkareceiver"
import (
	"errors"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/textutils"
)

type textLogsUnmarshaler struct {
	enc *textutils.Encoding
}

func newTextLogsUnmarshaler() LogsUnmarshalerWithEnc {
	return &textLogsUnmarshaler{}
}

func (r *textLogsUnmarshaler) Unmarshal(buf []byte) (plog.Logs, error) {
	if r.enc == nil {
		return plog.Logs{}, errors.New("encoding not set")
	}
	p := plog.NewLogs()
	decoded, err := r.enc.Decode(buf)
	if err != nil {
		return p, err
	}

	l := p.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
	l.SetObservedTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	l.Body().SetStr(string(decoded))
	return p, nil
}

func (r *textLogsUnmarshaler) Encoding() string {
	return "text"
}

func (r *textLogsUnmarshaler) WithEnc(encodingName string) (LogsUnmarshalerWithEnc, error) {
	var err error
	encCfg := textutils.NewEncodingConfig()
	encCfg.Encoding = encodingName
	enc, err := encCfg.Build()
	if err != nil {
		return nil, err
	}
	return &textLogsUnmarshaler{
		enc: &enc,
	}, nil
}
