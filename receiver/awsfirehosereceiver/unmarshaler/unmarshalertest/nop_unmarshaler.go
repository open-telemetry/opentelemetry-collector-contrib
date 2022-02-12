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

package unmarshalertest // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsfirehosereceiver/unmarshaler/unmarshalertest"

import (
	"go.opentelemetry.io/collector/model/pdata"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsfirehosereceiver/unmarshaler"
)

const encoding = "nop"

type nopMetricsUnmarshaler struct {
	err error
}

var _ unmarshaler.MetricsUnmarshaler = (*nopMetricsUnmarshaler)(nil)

func NewNopMetrics() *nopMetricsUnmarshaler {
	return &nopMetricsUnmarshaler{}
}

func NewErrMetrics(err error) *nopMetricsUnmarshaler {
	return &nopMetricsUnmarshaler{err}
}

// Unmarshal deserializes the records into metrics
func (u *nopMetricsUnmarshaler) Unmarshal([][]byte) (pdata.Metrics, error) {
	return pdata.NewMetrics(), u.err
}

// Encoding of the serialized messages
func (u *nopMetricsUnmarshaler) Encoding() string {
	return encoding
}
