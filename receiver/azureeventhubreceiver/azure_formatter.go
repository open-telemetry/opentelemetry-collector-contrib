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

package azureeventhubreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azureeventhubreceiver"

import (
	eventhub "github.com/Azure/azure-event-hubs-go/v3"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/receiver"
)

type azureLogFormatConverter struct {
	buildInfo component.BuildInfo
}

func newAzureLogFormatConverter(settings receiver.CreateSettings) *azureLogFormatConverter {
	return &azureLogFormatConverter{buildInfo: settings.BuildInfo}
}

func (c *azureLogFormatConverter) ToLogs(event *eventhub.Event) (plog.Logs, error) {
	logs, err := transform(c.buildInfo, event.Data)
	return logs, err
}
