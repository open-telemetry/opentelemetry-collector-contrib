// Copyright 2019, OpenTelemetry Authors
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

package awsxrayexporter

import (
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/configtest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/awsutil"
)

func TestLoadConfig(t *testing.T) {
	factories, err := componenttest.NopFactories()
	assert.Nil(t, err)

	factory := NewFactory()
	factories.Exporters[typeStr] = factory
	cfg, err := configtest.LoadConfigAndValidate(path.Join(".", "testdata", "config.yaml"), factories)

	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, len(cfg.Exporters), 2)

	r0 := cfg.Exporters[config.NewComponentID(typeStr)]
	assert.Equal(t, r0, factory.CreateDefaultConfig())

	r1 := cfg.Exporters[config.NewComponentIDWithName(typeStr, "customname")].(*Config)
	assert.Equal(t, r1,
		&Config{
			ExporterSettings: config.NewExporterSettings(config.NewComponentIDWithName(typeStr, "customname")),
			AWSSessionSettings: awsutil.AWSSessionSettings{
				NumberOfWorkers:       8,
				Endpoint:              "",
				RequestTimeoutSeconds: 30,
				MaxRetries:            2,
				NoVerifySSL:           false,
				ProxyAddress:          "",
				Region:                "eu-west-1",
				LocalMode:             false,
				ResourceARN:           "arn:aws:ec2:us-east1:123456789:instance/i-293hiuhe0u",
				RoleARN:               "arn:aws:iam::123456789:role/monitoring-EKS-NodeInstanceRole",
			},
			IndexedAttributes:  []string{"indexed_attr_0", "indexed_attr_1"},
			IndexAllAttributes: false,
		})
}
