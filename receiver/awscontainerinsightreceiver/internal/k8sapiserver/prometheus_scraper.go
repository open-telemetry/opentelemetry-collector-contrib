package k8sapiserver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/k8sapiserver"

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/simpleprometheusreceiver"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/k8s/k8sclient"
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

	// TODO: test RBAC permissions?

	spFactory := simpleprometheusreceiver.NewFactory() // TODO: pass this in?

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

func (ps *PrometheusScraper) Start() {
	ps.simplePrometheusReceiver.Start(ps.ctx, ps.host)
}

func (ps *PrometheusScraper) Shutdown() {
	ps.simplePrometheusReceiver.Shutdown(ps.ctx)
}
