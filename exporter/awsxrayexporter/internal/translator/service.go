// Copyright 2019, OpenTelemetry Authors
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

package translator

import (
	"go.opentelemetry.io/collector/model/pdata"
	conventions "go.opentelemetry.io/collector/model/semconv/v1.5.0"

	awsxray "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/xray"
)

func makeService(resource pdata.Resource) *awsxray.ServiceData {
	var service *awsxray.ServiceData

	verStr, ok := resource.Attributes().Get(conventions.AttributeServiceVersion)
	if !ok {
		verStr, ok = resource.Attributes().Get(conventions.AttributeContainerImageTag)
	}
	if ok {
		service = &awsxray.ServiceData{
			Version: awsxray.String(verStr.StringVal()),
		}
	}
	return service
}
