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

package kubeletstatsreceiver

import (
	"path"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/config/configtls"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/k8sconfig"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver/kubelet"
)

func TestLoadConfig(t *testing.T) {
	factories, err := config.ExampleComponents()
	require.NoError(t, err)
	factory := &Factory{}
	factories.Receivers[configmodels.Type(typeStr)] = factory
	cfg, err := config.LoadConfigFile(
		t, path.Join(".", "testdata", "config.yaml"), factories,
	)
	require.NoError(t, err)
	require.NotNil(t, cfg)

	tlsCfg := cfg.Receivers["kubeletstats/tls"].(*Config)
	duration := 10 * time.Second
	require.Equal(t, &Config{
		ReceiverSettings: configmodels.ReceiverSettings{
			TypeVal: "kubeletstats",
			NameVal: "kubeletstats/tls",
		},
		Endpoint: "1.2.3.4:5555",
		ClientConfig: kubelet.ClientConfig{
			APIConfig: k8sconfig.APIConfig{
				AuthType: "tls",
			},
			TLSSetting: configtls.TLSSetting{
				CAFile:   "/path/to/ca.crt",
				CertFile: "/path/to/apiserver.crt",
				KeyFile:  "/path/to/apiserver.key",
			},
			InsecureSkipVerify: true,
		},
		CollectionInterval: duration,
	}, tlsCfg)

	saCfg := cfg.Receivers["kubeletstats/sa"].(*Config)
	require.Equal(t, &Config{
		ReceiverSettings: configmodels.ReceiverSettings{
			TypeVal: "kubeletstats",
			NameVal: "kubeletstats/sa",
		},
		ClientConfig: kubelet.ClientConfig{
			APIConfig: k8sconfig.APIConfig{
				AuthType: "serviceAccount",
			},
			InsecureSkipVerify: true,
		},
		CollectionInterval: duration,
	}, saCfg)
}
