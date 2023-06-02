// Copyright The OpenTelemetry Authors
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

package batch // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awskinesisexporter/internal/batch"

import (
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

type unsupported struct{}

var (
	_ ptrace.Marshaler  = (*unsupported)(nil)
	_ pmetric.Marshaler = (*unsupported)(nil)
	_ plog.Marshaler    = (*unsupported)(nil)
)

func (unsupported) MarshalTraces(_ ptrace.Traces) ([]byte, error) {
	return nil, ErrUnsupportedEncoding
}

func (unsupported) MarshalMetrics(_ pmetric.Metrics) ([]byte, error) {
	return nil, ErrUnsupportedEncoding
}

func (unsupported) MarshalLogs(_ plog.Logs) ([]byte, error) {
	return nil, ErrUnsupportedEncoding
}
