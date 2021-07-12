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

package awsxrayreceiver

import (
	"context"
	"os"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenterror"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/configcheck"
	"go.opentelemetry.io/collector/consumer/consumertest"

	awsxray "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/xray"
)

func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	assert.NotNil(t, cfg, "failed to create default config")
	assert.NoError(t, configcheck.ValidateConfig(cfg))

	assert.Equal(t, config.Type(awsxray.TypeStr), factory.Type())
}

func TestCreateTracesReceiver(t *testing.T) {
	// TODO review if test should succeed on Windows
	if runtime.GOOS == "windows" {
		t.Skip()
	}

	env := stashEnv()
	defer restoreEnv(env)
	os.Setenv(defaultRegionEnvName, mockRegion)

	factory := NewFactory()
	_, err := factory.CreateTracesReceiver(
		context.Background(),
		componenttest.NewNopReceiverCreateSettings(),
		factory.CreateDefaultConfig().(*Config),
		consumertest.NewNop(),
	)
	assert.Nil(t, err, "trace receiver can be created")
}

func TestCreateMetricsReceiver(t *testing.T) {
	factory := NewFactory()
	_, err := factory.CreateMetricsReceiver(
		context.Background(),
		componenttest.NewNopReceiverCreateSettings(),
		factory.CreateDefaultConfig().(*Config),
		consumertest.NewNop(),
	)
	assert.NotNil(t, err, "a trace receiver factory should not create a metric receiver")
	assert.ErrorIs(t, err, componenterror.ErrDataTypeIsNotSupported)
}

func stashEnv() []string {
	env := os.Environ()
	os.Clearenv()

	return env
}

func restoreEnv(env []string) {
	os.Clearenv()

	for _, e := range env {
		p := strings.SplitN(e, "=", 2)
		os.Setenv(p[0], p[1])
	}
}
