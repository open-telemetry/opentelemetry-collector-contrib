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

package statsreader // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudspannerreceiver/internal/statsreader"

import (
	"context"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudspannerreceiver/internal/metadata"
)

type ReaderConfig struct {
	TopMetricsQueryMaxRows int
	BackfillEnabled        bool
}

type Reader interface {
	Name() string
	Read(ctx context.Context) ([]*metadata.MetricsDataPoint, error)
}

// CompositeReader - this interface is used for the composition of multiple Reader(s).
// Main difference between it and Reader - this interface also has Shutdown method for performing some additional
// cleanup necessary for each Reader instance.
type CompositeReader interface {
	Reader
	// Shutdown Use this method to perform any additional cleanup of underlying components.
	Shutdown()
}
