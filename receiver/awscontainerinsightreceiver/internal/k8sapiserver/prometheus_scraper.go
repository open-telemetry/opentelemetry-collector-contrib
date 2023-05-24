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

package k8sapiserver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/k8sapiserver"

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/k8s/k8sclient"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/simpleprometheusreceiver"
)

const (
	caFile             = "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"
	collectionInterval = 60 * time.Second
)

type PrometheusScraper struct {
	ctx                      context.Context
	settings                 component.TelemetrySettings
	host                     component.Host
	clusterNameProvider      clusterNameProvider
	simplePrometheusReceiver receiver.Metrics
}

func NewPrometheusScraper(ctx context.Context, telemetrySettings component.TelemetrySettings, nextConsumer consumer.Metrics, host component.Host, clusterNameProvider clusterNameProvider) (*PrometheusScraper, error) {
	// TODO: need leader election

	k8sClient := k8sclient.Get(telemetrySettings.Logger)
	if k8sClient == nil {
		return nil, errors.New("failed to start k8sapiserver because k8sclient is nil")
	}

	// get endpoint
	endpoint := k8sClient.GetClientSet().CoreV1().RESTClient().Get().AbsPath("/").URL().Hostname()

	spConfig := simpleprometheusreceiver.Config{
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Endpoint: endpoint,
			TLSSetting: configtls.TLSClientSetting{
				TLSSetting: configtls.TLSSetting{
					CAFile: caFile,
				},
				Insecure:           false,
				InsecureSkipVerify: false,
			},
		},
		MetricsPath:        "/metrics",
		CollectionInterval: collectionInterval,
		UseServiceAccount:  true,
	}

	consumer := newPrometheusConsumer(telemetrySettings.Logger, nextConsumer, clusterNameProvider.GetClusterName(), os.Getenv("HOST_NAME"))

	params := receiver.CreateSettings{
		TelemetrySettings: telemetrySettings,
	}

	spFactory := simpleprometheusreceiver.NewFactory()
	spr, err := spFactory.CreateMetricsReceiver(ctx, params, &spConfig, consumer)
	if err != nil {
		return nil, fmt.Errorf("failed to create simple prometheus receiver: %w", err)
	}

	return &PrometheusScraper{
		ctx:                      ctx,
		settings:                 telemetrySettings,
		host:                     host,
		clusterNameProvider:      clusterNameProvider,
		simplePrometheusReceiver: spr,
	}, nil
}

func (ps *PrometheusScraper) Start() error {
	return ps.simplePrometheusReceiver.Start(ps.ctx, ps.host)
}

func (ps *PrometheusScraper) Shutdown() error {
	return ps.simplePrometheusReceiver.Shutdown(ps.ctx)
}
