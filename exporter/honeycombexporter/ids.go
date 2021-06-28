// Copyright 2019 OpenTelemetry Authors
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

package honeycombexporter

import (
	"encoding/binary"
	"fmt"

	"go.opentelemetry.io/collector/model/pdata"
)

const (
	traceIDShortLength = 8
)

// getHoneycombID returns an ID suitable for use for spans and traces. Before
// encoding the bytes as a hex string, we want to handle cases where we are
// given 128-bit IDs with zero padding, e.g. 0000000000000000f798a1e7f33c8af6.
// To do this, we borrow a strategy from Jaeger [1] wherein we split the byte
// sequence into two parts. The leftmost part could contain all zeros. We use
// that to determine whether to return a 64-bit hex encoded string or a 128-bit
// one.
//
// [1]: https://github.com/jaegertracing/jaeger/blob/cd19b64413eca0f06b61d92fe29bebce1321d0b0/model/ids.go#L81
func getHoneycombTraceID(traceID pdata.TraceID) string {
	// binary.BigEndian.Uint64() does a bounds check on traceID which will
	// cause a panic if traceID is fewer than 8 bytes. In this case, we don't
	// need to check for zero padding on the high part anyway, so just return a
	// hex string.

	var low uint64
	tID := traceID.Bytes()

	low = binary.BigEndian.Uint64(tID[traceIDShortLength:])
	if high := binary.BigEndian.Uint64(tID[:traceIDShortLength]); high != 0 {
		return fmt.Sprintf("%016x%016x", high, low)
	}

	return fmt.Sprintf("%016x", low)
}

// getHoneycombSpanID just takes a byte array and hex encodes it.
func getHoneycombSpanID(id pdata.SpanID) string {
	if !id.IsEmpty() {
		return fmt.Sprintf("%x", id.Bytes())
	}
	return ""
}
