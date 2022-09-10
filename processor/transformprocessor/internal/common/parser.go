// Copyright  The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package common // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/common"

import (
	"encoding/hex"
	"errors"

	"go.opentelemetry.io/collector/pdata/pcommon"
)

func ParseSpanID(spanIDStr string) (pcommon.SpanID, error) {
	id, err := hex.DecodeString(spanIDStr)
	if err != nil {
		return pcommon.SpanID{}, err
	}
	if len(id) != 8 {
		return pcommon.SpanID{}, errors.New("span ids must be 8 bytes")
	}
	var idArr [8]byte
	copy(idArr[:8], id)
	return idArr, nil
}

func ParseTraceID(traceIDStr string) (pcommon.TraceID, error) {
	id, err := hex.DecodeString(traceIDStr)
	if err != nil {
		return pcommon.TraceID{}, err
	}
	if len(id) != 16 {
		return pcommon.TraceID{}, errors.New("traces ids must be 16 bytes")
	}
	var idArr [16]byte
	copy(idArr[:16], id)
	return idArr, nil
}
