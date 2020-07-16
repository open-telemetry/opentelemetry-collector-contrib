// Copyright 2020, OpenTelemetry Authors
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

package cudareceiver

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"
)

// Receiver is the type used to handle metrics from CUDA.
type Receiver struct {
	c      *CUDAMetricsCollector
	logger *zap.Logger
}

// Start scrapes VM metrics based on the OS platform.
func (r *Receiver) Start(ctx context.Context, host component.Host) error {
	r.c.StartCollection()
	return nil
}

// Shutdown stops and cancels the underlying VM metrics scrapers.
func (r *Receiver) Shutdown(context.Context) error {
	r.c.StopCollection()
	return nil
}
