// Copyright observIQ, Inc.
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

//go:build !windows

package windowseventlogreceiver

import (
	"go.opentelemetry.io/collector/consumer"
	"go.uber.org/zap"

	"github.com/observiq/bindplane-otel-collector/receiver/windowseventlogreceiver/internal/sidcache"
)

// newSIDEnrichingConsumer is a stub for non-Windows platforms
// This allows the code to compile on macOS/Linux during development
func newSIDEnrichingConsumer(next consumer.Logs, _ sidcache.Cache, _ *zap.Logger) consumer.Logs {
	// SID enrichment is only supported on Windows
	// Return the next consumer unchanged
	return next
}
