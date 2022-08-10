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

package syslogexporter

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	factory := NewFactory()

	t.Run("No endpoint causes error", func(t *testing.T) {
		cfg := factory.CreateDefaultConfig().(*Config)
		cfg.Endpoint = ""
		err := cfg.Validate()
		assert.IsType(t, errConfigNoEndpoint, err)
	})

	t.Run("Set default port if not set", func(t *testing.T) {
		cfg := factory.CreateDefaultConfig().(*Config)
		cfg.Endpoint = "localhost"
		err := cfg.Validate()
		assert.NoError(t, err)

		assert.Equal(t, "localhost:514", cfg.Endpoint)
	})

	t.Run("Set default port if not set", func(t *testing.T) {
		cfg := factory.CreateDefaultConfig().(*Config)
		cfg.Endpoint = "localhost"
		err := cfg.Validate()
		assert.NoError(t, err)

		assert.Equal(t, "localhost:514", cfg.Endpoint)
	})

	t.Run("Net protocol set as TCP if not set", func(t *testing.T) {
		cfg := factory.CreateDefaultConfig().(*Config)
		cfg.Endpoint = "localhost"
		err := cfg.Validate()
		assert.NoError(t, err)

		assert.Equal(t, "tcp", cfg.NetProtocol)
	})
}
