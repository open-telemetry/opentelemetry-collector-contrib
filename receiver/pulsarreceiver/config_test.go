// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package pulsarreceiver

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/service/servicetest"
)

func TestLoadConfig(t *testing.T) {
	factories, err := componenttest.NopFactories()
	assert.NoError(t, err)

	factory := NewFactory()
	factories.Receivers[typeStr] = factory
	cfg, err := servicetest.LoadConfigAndValidate(filepath.Join("testdata", "config.yml"), factories)
	require.NoError(t, err)
	require.Equal(t, 1, len(cfg.Receivers))

	r := cfg.Receivers[config.NewComponentID(typeStr)].(*Config)

	paramMap := map[string]string{
		"tlsCertFile": "cert.pem",
		"tlsKeyFile":  "key.pem",
	}
	param, _ := json.Marshal(paramMap)
	assert.Equal(t, &Config{
		ReceiverSettings:      config.NewReceiverSettings(config.NewComponentID(typeStr)),
		Topic:                 "otel-pulsar",
		ServiceUrl:            "pulsar://localhost:6500",
		ConsumerName:          "otel-collector",
		Subscription:          "otel-collector",
		Encoding:              defaultEncoding,
		TLSTrustCertsFilePath: "ca.pem",
		AuthName:              "tls",
		AuthParam:             string(param),
	}, r)
}
