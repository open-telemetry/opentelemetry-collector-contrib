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

package producer

import (
	"context"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awskinesisexporter/internal/batch"
)

// Batcher abstracts the raw kinesis client to reduce complexity with delivering dynamic encoded data.
type Batcher interface {
	// Put is a blocking operation that will attempt to write the data at most once to kinesis.
	// Any unrecoverable errors such as misconfigured client or hard limits being exceeded
	// will result in consumeerr.Permanent being returned to allow for existing retry patterns within
	// the project to be used.
	Put(ctx context.Context, b *batch.Batch) error

	// Ready ensures that the configuration is valid and can write the configured stream.
	Ready(ctx context.Context) error
}
