// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package serializer // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/serializer"

import (
	"encoding/json"
	"fmt"
	"math"
	"time"
)

// UnixTime64 represents nanoseconds since epoch.
type UnixTime64 uint64

// NewUnixTime64 creates a UnixTime64 from either seconds or nanoseconds since the epoch.
func NewUnixTime64(t uint64) UnixTime64 {
	if t <= math.MaxUint32 {
		return UnixTime64(t) * 1e9
	}
	return UnixTime64(t)
}

func (t UnixTime64) MarshalJSON() ([]byte, error) {
	// Nanoseconds, ES does not support 'epoch_nanoseconds' so
	// we have to pass it a value formatted as 'strict_date_optional_time_nanos'.
	out := fmt.Appendf(nil, "%q",
		time.Unix(0, int64(t)).UTC().Format(time.RFC3339Nano))
	return out, nil
}

// Compile-time interface checks
var _ json.Marshaler = (*UnixTime64)(nil)
