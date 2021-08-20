// Copyright  OpenTelemetry Authors
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

package translate

import (
	"errors"

	"go.opentelemetry.io/collector/model/pdata"
)

var (
	// ErrUnsupportedEncodedType is used when the encoder type does not support the type of encoding
	ErrUnsupportedEncodedType = errors.New("unsupported type to encode")
)

// ExportWriter wraps the kinesis exporter and transforms the data into
// the desired output format.
type ExportWriter interface {
	WriteMetrics(md pdata.Metrics) error

	WriteTraces(td pdata.Traces) error

	WriteLogs(ld pdata.Logs) error
}
