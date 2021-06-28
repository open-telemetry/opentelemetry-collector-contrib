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

// Deprecated: batchpertrace is superseded by batchpersignal.
package batchpertrace

import (
	"go.opentelemetry.io/collector/model/pdata"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/batchpersignal"
)

// Split returns one pdata.Traces for each trace in the given pdata.Traces input. Each of the resulting pdata.Traces contains exactly one trace.
func Split(batch pdata.Traces) []pdata.Traces {
	return batchpersignal.SplitTraces(batch)
}
