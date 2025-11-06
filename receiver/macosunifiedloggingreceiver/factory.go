// Copyright observIQ, Inc.
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

package macosunifiedloggingreceiver

import (
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/receiver"
)

// NewFactory creates a factory for the macOS unified logging receiver
func NewFactory() receiver.Factory {
	return newFactoryAdapter()
}

// createDefaultConfig creates a config with default values
func createDefaultConfig() component.Config {
	// Default to live mode
	return &Config{
		MaxPollInterval: 30 * time.Second,
		MaxLogAge:       24 * time.Hour,
		Format:          "default",
	}
}
