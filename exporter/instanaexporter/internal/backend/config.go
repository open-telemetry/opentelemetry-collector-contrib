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

package backend // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/instanaexporter/internal/backend"

const (
	// AttributeInstanaHostID can be used to distinguish multiple hosts' data
	// being processed by a single collector (in a chained scenario)
	AttributeInstanaHostID = "instana.host.id"

	HeaderKey  = "x-instana-key"
	HeaderHost = "x-instana-host"
	HeaderTime = "x-instana-time"
)
