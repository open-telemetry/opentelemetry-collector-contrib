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

package chronyreceiver

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.opentelemetry.io/collector/receiver/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/chronyreceiver/internal/metadata"
)

func TestType(t *testing.T) {
	t.Parallel()

	factory := NewFactory()
	assert.Equal(t, component.Type("chrony"), factory.Type(), "Must match the expected type")
}

func TestValidConfig(t *testing.T) {
	t.Parallel()

	factory := NewFactory()
	assert.NoError(t, componenttest.CheckConfigStruct(factory.CreateDefaultConfig()))
}

func TestCreatingMetricsReceiver(t *testing.T) {
	t.Parallel()

	factory := NewFactory()
	mbc := metadata.DefaultMetricsBuilderConfig()
	mbc.Metrics = metadata.MetricsConfig{
		NtpTimeCorrection: metadata.MetricConfig{
			Enabled: true,
		},
		NtpSkew: metadata.MetricConfig{
			Enabled: true,
		},
	}
	mem, err := factory.CreateMetricsReceiver(
		context.Background(),
		receivertest.NewNopCreateSettings(),
		&Config{
			ScraperControllerSettings: scraperhelper.ScraperControllerSettings{
				CollectionInterval: 30 * time.Second,
			},
			MetricsBuilderConfig: mbc,
			Endpoint:             "udp://localhost:323",
			Timeout:              10 * time.Second,
		},
		consumertest.NewNop(),
	)
	assert.NoError(t, err, "Must not error creating metrics receiver")
	assert.NotNil(t, mem, "Must have a valid metrics receiver client")
}
