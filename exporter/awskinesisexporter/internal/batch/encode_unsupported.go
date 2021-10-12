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

import "go.opentelemetry.io/collector/model/pdata"

type unsupported struct{}

var (
	_ pdata.TracesMarshaler  = (*unsupported)(nil)
	_ pdata.MetricsMarshaler = (*unsupported)(nil)
	_ pdata.LogsMarshaler    = (*unsupported)(nil)
)

func (unsupported) MarshalTraces(_ pdata.Traces) ([]byte, error) {
	return nil, ErrUnsupportedEncoding
}

func (unsupported) MarshalMetrics(_ pdata.Metrics) ([]byte, error) {
	return nil, ErrUnsupportedEncoding
}

func (unsupported) MarshalLogs(_ pdata.Logs) ([]byte, error) {
	return nil, ErrUnsupportedEncoding
}
