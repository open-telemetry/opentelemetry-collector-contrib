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

package azureeventhubreceiver // import "github.com/loomis-relativity/opentelemetry-collector-contrib/receiver/azureeventhubreceiver"

import (
	eventhub "github.com/Azure/azure-event-hubs-go/v3"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
)

type RawConverter struct{}

func (_ *RawConverter) ToLogs(event *eventhub.Event) (plog.Logs, error) {
	l := plog.NewLogs()
	lr := l.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
	slice := lr.Body().SetEmptyBytes()
	slice.Append(event.Data...)
	lr.Attributes().FromRaw(event.Properties)
	if event.SystemProperties.EnqueuedTime != nil {
		lr.SetTimestamp(pcommon.NewTimestampFromTime(*event.SystemProperties.EnqueuedTime))
	}
	return l, nil
}
