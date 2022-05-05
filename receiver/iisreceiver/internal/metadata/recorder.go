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

package metadata // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/iisreceiver/internal/metadata"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
)

func (mb *MetricsBuilder) RecordAny(ts pcommon.Timestamp, val float64, name string, attributes map[string]string) {
	switch name {
	case "iis.connection.active":
		mb.RecordIisConnectionActiveDataPoint(ts, int64(val))
	case "iis.connection.anonymous":
		mb.RecordIisConnectionAnonymousDataPoint(ts, int64(val))
	case "iis.connection.attempt.count":
		mb.RecordIisConnectionAttemptCountDataPoint(ts, int64(val))
	case "iis.network.blocked":
		mb.RecordIisNetworkBlockedDataPoint(ts, int64(val))
	case "iis.network.file.count":
		mb.RecordIisNetworkFileCountDataPoint(ts, int64(val), MapAttributeDirection[attributes[A.Direction]])
	case "iis.network.io":
		mb.RecordIisNetworkIoDataPoint(ts, int64(val), MapAttributeDirection[attributes[A.Direction]])
	case "iis.request.count":
		mb.RecordIisRequestCountDataPoint(ts, int64(val), MapAttributeRequest[attributes[A.Request]])
	case "iis.request.queue.age.max":
		mb.RecordIisRequestQueueAgeMaxDataPoint(ts, int64(val))
	case "iis.request.queue.count":
		mb.RecordIisRequestQueueCountDataPoint(ts, int64(val))
	case "iis.request.rejected":
		mb.RecordIisRequestRejectedDataPoint(ts, int64(val))
	case "iis.thread.active":
		mb.RecordIisThreadActiveDataPoint(ts, int64(val))
	case "iis.uptime":
		mb.RecordIisUptimeDataPoint(ts, int64(val))
	}
}
