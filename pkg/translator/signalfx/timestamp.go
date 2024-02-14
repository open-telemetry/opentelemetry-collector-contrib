// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package signalfx // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/signalfx"

import "go.opentelemetry.io/collector/pdata/pcommon"

const millisToNanos = 1e6

func fromTimestamp(ts pcommon.Timestamp) int64 {
	// Convert nanos to millis.
	return int64(ts) / millisToNanos
}

func toTimestamp(ts int64) pcommon.Timestamp {
	// Convert millis to nanos.
	return pcommon.Timestamp(ts * millisToNanos)
}
