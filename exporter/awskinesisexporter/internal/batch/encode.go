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

package batch

import (
	"errors"

	"go.opentelemetry.io/collector/model/pdata"
)

// ErrUnsupportedEncodedType is used when the encoder type does not support the type of encoding
var ErrUnsupportedEncodedType = errors.New("unsupported type to encode")

// Encoder transforms the internal pipeline format into a configurable
// format that is then used to export to kinesis.
type Encoder interface {
	Metrics(md pdata.Metrics) (*Batch, error)

	Traces(td pdata.Traces) (*Batch, error)

	Logs(ld pdata.Logs) (*Batch, error)
}
