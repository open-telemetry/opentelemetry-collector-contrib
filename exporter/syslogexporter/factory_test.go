// Copyright The OpenTelemetry Authors
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
	"testing"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

func TestType(t *testing.T) {
	factory := NewFactory()
	pType := factory.Type()
	assert.Equal(t, pType, component.Type("syslog"))
}

func TestCreateDefaultConfig(t *testing.T) {
	cfg := createDefaultConfig()

	assert.Equal(t, cfg, &Config{
		Port:     514,
		Network:  "tcp",
		Protocol: "rfc5424",
		QueueSettings: exporterhelper.QueueSettings{
			Enabled:      false,
			NumConsumers: 10,
			QueueSize:    1000,
		},
		RetrySettings: exporterhelper.RetrySettings{
			Enabled:             true,
			InitialInterval:     5 * time.Second,
			RandomizationFactor: backoff.DefaultRandomizationFactor,
			Multiplier:          backoff.DefaultMultiplier,
			MaxInterval:         30 * time.Second,
			MaxElapsedTime:      5 * time.Minute,
		},
		TimeoutSettings: exporterhelper.TimeoutSettings{
			Timeout: 5 * time.Second,
		},
	})
}
