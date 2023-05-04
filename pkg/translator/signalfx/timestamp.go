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
