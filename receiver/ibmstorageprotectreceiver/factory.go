package ibmstorageprotectreceiver

import (
	"context"
	"fmt"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
)

var (
	TypeR = component.MustNewType("ibmstorageprotectreceiver")
)

func createDefaultConfig() component.Config {
	return &Config{
		Config: *fileconsumer.NewConfig(),
	}
}

func NewFactory() receiver.Factory {
	// fmt.Println("************************  NewFactory function running  ************************")
	return receiver.NewFactory(
		TypeR,
		createDefaultConfig,
		receiver.WithMetrics(createMetricsReceiver, MetricsStability))
}

func createMetricsReceiver(_ context.Context, settings receiver.CreateSettings, configuration component.Config, metrics consumer.Metrics) (receiver.Metrics, error) {
	// fmt.Println("************************  createMetricsReceiver function running  ************************")
	metricsUnmarshaler := &pmetric.JSONUnmarshaler{}
	obsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{
		ReceiverID:             settings.ID,
		Transport:              transport,
		ReceiverCreateSettings: settings,
	})
	if err != nil {
		return nil, err
	}
	cfg := configuration.(*Config)
	input, err := cfg.Config.Build(settings.Logger.Sugar(), func(ctx context.Context, token []byte, _ map[string]any) error {
		//This anonymous function is executed when there is a change in the watched file/directory
		ctx = obsrecv.StartMetricsOp(ctx)
		var m pmetric.Metrics
		fmt.Println()
		fmt.Println()
		//fmt.Println("Token in ASCII: ", token)
		m, err = metricsUnmarshaler.UnmarshalMetrics(token)
		fmt.Println("err: ", err)
		if err != nil {
			obsrecv.EndMetricsOp(ctx, TypeR.String(), 0, err)
		} else {
			if m.ResourceMetrics().Len() != 0 {
				err = metrics.ConsumeMetrics(ctx, m)
			}
			obsrecv.EndMetricsOp(ctx, TypeR.String(), m.MetricCount(), err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &ibmstorageprotectreceiver{input: input, id: settings.ID, storageID: cfg.StorageID}, nil
}
