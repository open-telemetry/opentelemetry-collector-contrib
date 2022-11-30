// Copyright 2022 The OpenTelemetry Authors
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

package scrapertest // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/purefareceiver/internal/scrapertest"

import "go.opentelemetry.io/collector/component"

type MockHost struct {
	Extensions map[component.ID]component.Component
}

func (h *MockHost) ReportFatalError(_ error) {}

func (h *MockHost) GetFactory(_ component.Kind, _ component.Type) component.Factory {
	return nil
}

func (h *MockHost) GetExtensions() map[component.ID]component.Component {
	return h.Extensions
}

func (h *MockHost) GetExporters() map[component.DataType]map[component.ID]component.Component {
	return nil
}
