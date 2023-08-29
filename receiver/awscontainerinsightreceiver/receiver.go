// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awscontainerinsightreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver"

import (
	"context"
	"errors"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"

	ci "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/containerinsight"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/cadvisor"
	ecsinfo "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/ecsInfo"
	hostInfo "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/host"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/k8sapiserver"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/stores"
)

var _ receiver.Metrics = (*awsContainerInsightReceiver)(nil)

type metricsProvider interface {
	GetMetrics() []pmetric.Metrics
	Shutdown() error
}

// awsContainerInsightReceiver implements the receiver.Metrics
type awsContainerInsightReceiver struct {
	settings     component.TelemetrySettings
	nextConsumer consumer.Metrics
	config       *Config
	cancel       context.CancelFunc
	cadvisor     metricsProvider
	k8sapiserver metricsProvider
}

// newAWSContainerInsightReceiver creates the aws container insight receiver with the given parameters.
func newAWSContainerInsightReceiver(
	settings component.TelemetrySettings,
	config *Config,
	nextConsumer consumer.Metrics) (receiver.Metrics, error) {
	if nextConsumer == nil {
		return nil, component.ErrNilNextConsumer
	}

	r := &awsContainerInsightReceiver{
		settings:     settings,
		nextConsumer: nextConsumer,
		config:       config,
	}
	return r, nil
}

// Start collecting metrics from cadvisor and k8s api server (if it is an elected leader)
func (acir *awsContainerInsightReceiver) Start(ctx context.Context, host component.Host) error {
	ctx, acir.cancel = context.WithCancel(ctx)

	hostinfo, err := hostInfo.NewInfo(acir.config.ContainerOrchestrator, acir.config.CollectionInterval, acir.settings.Logger)
	if err != nil {
		return err
	}

	if acir.config.ContainerOrchestrator == ci.EKS {
		k8sDecorator, err := stores.NewK8sDecorator(ctx, acir.config.TagService, acir.config.PrefFullPodName, acir.config.AddFullPodNameMetricLabel, acir.settings.Logger)
		if err != nil {
			return err
		}

		decoratorOption := cadvisor.WithDecorator(k8sDecorator)
		acir.cadvisor, err = cadvisor.New(acir.config.ContainerOrchestrator, hostinfo, acir.settings.Logger, decoratorOption)
		if err != nil {
			return err
		}
		acir.k8sapiserver, err = k8sapiserver.New(hostinfo, acir.settings.Logger)
		if err != nil {
			return err
		}
	}
	if acir.config.ContainerOrchestrator == ci.ECS {

		ecsInfo, err := ecsinfo.NewECSInfo(acir.config.CollectionInterval, hostinfo, host, acir.settings)
		if err != nil {
			return err
		}

		ecsOption := cadvisor.WithECSInfoCreator(ecsInfo)

		acir.cadvisor, err = cadvisor.New(acir.config.ContainerOrchestrator, hostinfo, acir.settings.Logger, ecsOption)
		if err != nil {
			return err
		}
	}

	go func() {
		// cadvisor collects data at dynamical intervals (from 1 to 15 seconds). If the ticker happens
		// at beginning of a minute, it might read the data collected at end of last minute. To avoid this,
		// we want to wait until at least two cadvisor collection intervals happens before collecting the metrics
		secondsInMin := time.Now().Second()
		if secondsInMin < 30 {
			time.Sleep(time.Duration(30-secondsInMin) * time.Second)
		}
		ticker := time.NewTicker(acir.config.CollectionInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				_ = acir.collectData(ctx)
			case <-ctx.Done():
				return
			}
		}
	}()

	return nil
}

// Shutdown stops the awsContainerInsightReceiver receiver.
func (acir *awsContainerInsightReceiver) Shutdown(context.Context) error {
	if acir.cancel == nil {
		return nil
	}
	acir.cancel()

	var errs error

	if acir.k8sapiserver != nil {
		errs = errors.Join(errs, acir.k8sapiserver.Shutdown())
	}
	if acir.cadvisor != nil {
		errs = errors.Join(errs, acir.cadvisor.Shutdown())
	}

	return errs

}

// collectData collects container stats from Amazon ECS Task Metadata Endpoint
func (acir *awsContainerInsightReceiver) collectData(ctx context.Context) error {
	var mds []pmetric.Metrics
	if acir.cadvisor == nil && acir.k8sapiserver == nil {
		err := errors.New("both cadvisor and k8sapiserver failed to start")
		acir.settings.Logger.Error("Failed to collect stats", zap.Error(err))
		return err
	}

	if acir.cadvisor != nil {
		mds = append(mds, acir.cadvisor.GetMetrics()...)
	}

	if acir.k8sapiserver != nil {
		mds = append(mds, acir.k8sapiserver.GetMetrics()...)
	}

	for _, md := range mds {
		err := acir.nextConsumer.ConsumeMetrics(ctx, md)
		if err != nil {
			return err
		}
	}

	return nil
}
