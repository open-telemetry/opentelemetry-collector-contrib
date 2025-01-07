// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awscontainerinsightreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver"

import (
	"context"
	"errors"
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/amazon-contributing/opentelemetry-collector-contrib/extension/awsmiddleware"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"
	"k8s.io/client-go/rest"

	ci "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/containerinsight"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/k8s/k8sclient"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/cadvisor"
	ecsinfo "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/ecsInfo"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/efa"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/gpu"
	hostinfo "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/host"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/k8sapiserver"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/k8swindows"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/neuron"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/prometheusscraper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/prometheusscraper/decoratorconsumer"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/stores"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/stores/kubeletutil"
)

const (
	waitForKubeletInterval = 10 * time.Second
)

var _ receiver.Metrics = (*awsContainerInsightReceiver)(nil)

type metricsProvider interface {
	GetMetrics() []pmetric.Metrics
	Shutdown() error
}

// awsContainerInsightReceiver implements the receiver.Metrics
type awsContainerInsightReceiver struct {
	settings                 component.TelemetrySettings
	nextConsumer             consumer.Metrics
	config                   *Config
	cancel                   context.CancelFunc
	decorators               []stores.Decorator
	containerMetricsProvider metricsProvider
	k8sapiserver             metricsProvider
	prometheusScraper        *k8sapiserver.PrometheusScraper
	podResourcesStore        *stores.PodResourcesStore
	dcgmScraper              *prometheusscraper.SimplePrometheusScraper
	neuronMonitorScraper     *prometheusscraper.SimplePrometheusScraper
	efaSysfsScraper          *efa.Scraper
}

// newAWSContainerInsightReceiver creates the aws container insight receiver with the given parameters.
func newAWSContainerInsightReceiver(
	settings component.TelemetrySettings,
	config *Config,
	nextConsumer consumer.Metrics,
) (receiver.Metrics, error) {
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
	var configurer *awsmiddleware.Configurer
	if acir.config != nil && acir.config.MiddlewareID != nil {
		configurer, _ = awsmiddleware.GetConfigurer(host.GetExtensions(), *acir.config.MiddlewareID)
	}

	hostInfo, hostInfoErr := hostinfo.NewInfo(acir.config.AWSSessionSettings, acir.config.ContainerOrchestrator,
		acir.config.CollectionInterval, acir.settings.Logger, configurer, hostinfo.WithClusterName(acir.config.ClusterName),
		hostinfo.WithSystemdEnabled(acir.config.RunOnSystemd))
	if hostInfoErr != nil {
		return hostInfoErr
	}

	hostName := os.Getenv(ci.HostName)
	if hostName == "" {
		hostName = acir.config.HostName
	}

	switch acir.config.ContainerOrchestrator {
	case ci.EKS:
		hostIP := os.Getenv(ci.HostIP)
		if hostIP == "" {
			hostIP = acir.config.HostIP
			if hostIP == "" {
				return errors.New("environment variable HOST_IP is not set in k8s deployment config or passed as part of the agent config")
			}
		}
		client, err := kubeletutil.NewKubeletClient(hostIP, ci.KubeSecurePort, kubeletutil.ClientConfig(acir.config.KubeConfigPath, acir.config.RunOnSystemd), acir.settings.Logger)
		if err != nil {
			return fmt.Errorf("cannot initialize kubelet client: %w", err)
		}
		// wait for kubelet availability, but don't block on it
		if acir.config.RunOnSystemd {
			go func() {
				if err = waitForKubelet(ctx, client, acir.settings.Logger); err != nil {
					acir.settings.Logger.Error("Unable to connect to kubelet", zap.Error(err))
					return
				}
				acir.settings.Logger.Debug("Kubelet is available. Initializing the receiver")
				if err = acir.initEKS(ctx, host, hostInfo, hostName, client); err != nil {
					acir.settings.Logger.Error("Unable to initialize receiver", zap.Error(err))
					return
				}
				acir.start(ctx)
			}()
		} else {
			if err = checkKubelet(client); err != nil {
				return err
			}
			if err = acir.initEKS(ctx, host, hostInfo, hostName, client); err != nil {
				return err
			}
			go acir.start(ctx)
		}
	case ci.ECS:
		if err := acir.initECS(host, hostInfo, hostName); err != nil {
			return err
		}
		go acir.start(ctx)
	default:
		return fmt.Errorf("unsupported container_orchestrator: %s", acir.config.ContainerOrchestrator)
	}

	return nil
}

func (acir *awsContainerInsightReceiver) initEKS(ctx context.Context, host component.Host, hostInfo *hostinfo.Info,
	hostName string, kubeletClient *kubeletutil.KubeletClient,
) error {
	k8sDecorator, err := stores.NewK8sDecorator(ctx, kubeletClient, acir.config.TagService, acir.config.PrefFullPodName,
		acir.config.AddFullPodNameMetricLabel, acir.config.AddContainerNameMetricLabel,
		acir.config.EnableControlPlaneMetrics, acir.config.EnableAcceleratedComputeMetrics,
		acir.config.KubeConfigPath, hostName, acir.config.RunOnSystemd, acir.settings.Logger)
	if err != nil {
		acir.settings.Logger.Warn("Unable to start K8s decorator", zap.Error(err))
	} else {
		acir.decorators = append(acir.decorators, k8sDecorator)
	}

	if runtime.GOOS == ci.OperatingSystemWindows {
		acir.containerMetricsProvider, err = k8swindows.New(acir.settings.Logger, k8sDecorator, *hostInfo)
		if err != nil {
			return err
		}
	} else {
		localNodeDecorator, err := stores.NewLocalNodeDecorator(acir.settings.Logger, acir.config.ContainerOrchestrator,
			hostInfo, hostName, stores.WithK8sDecorator(k8sDecorator))
		if err != nil {
			acir.settings.Logger.Warn("Unable to start local node decorator", zap.Error(err))
		} else {
			acir.decorators = append(acir.decorators, localNodeDecorator)
		}

		acir.containerMetricsProvider, err = cadvisor.New(acir.config.ContainerOrchestrator, hostInfo,
			acir.settings.Logger, cadvisor.WithDecorator(localNodeDecorator))
		if err != nil {
			return err
		}

		var leaderElection *k8sapiserver.LeaderElection
		leaderElection, err = k8sapiserver.NewLeaderElection(acir.settings.Logger, k8sapiserver.WithLeaderLockName(acir.config.LeaderLockName),
			k8sapiserver.WithLeaderLockUsingConfigMapOnly(acir.config.LeaderLockUsingConfigMapOnly))
		if err != nil {
			acir.settings.Logger.Warn("Unable to elect leader node", zap.Error(err))
		}

		acir.k8sapiserver, err = k8sapiserver.NewK8sAPIServer(hostInfo, acir.settings.Logger, leaderElection, acir.config.AddFullPodNameMetricLabel, acir.config.EnableControlPlaneMetrics)
		if err != nil {
			acir.k8sapiserver = nil
			acir.settings.Logger.Warn("Unable to connect to api-server", zap.Error(err))
		}

		if acir.k8sapiserver != nil {
			err = acir.initPrometheusScraper(ctx, host, hostInfo, leaderElection)
			if err != nil {
				acir.settings.Logger.Warn("Unable to start kube apiserver prometheus scraper", zap.Error(err))
			}
		}

		err = acir.initDcgmScraper(ctx, host, hostInfo, localNodeDecorator)
		if err != nil {
			acir.settings.Logger.Debug("Unable to start dcgm scraper", zap.Error(err))
		}
		err = acir.initPodResourcesStore()
		if err != nil {
			acir.settings.Logger.Debug("Unable to start pod resources store", zap.Error(err))
		}
		err = acir.initNeuronScraper(ctx, host, hostInfo, localNodeDecorator)
		if err != nil {
			acir.settings.Logger.Debug("Unable to start neuron scraper", zap.Error(err))
		}
		err = acir.initEfaSysfsScraper(localNodeDecorator)
		if err != nil {
			acir.settings.Logger.Debug("Unable to start EFA scraper", zap.Error(err))
		}
	}
	return nil
}

func (acir *awsContainerInsightReceiver) initECS(host component.Host, hostInfo *hostinfo.Info, hostName string) error {
	ecsInfo, err := ecsinfo.NewECSInfo(acir.config.CollectionInterval, hostInfo, host, acir.settings, ecsinfo.WithClusterName(acir.config.ClusterName))
	if err != nil {
		return err
	}

	localNodeDecorator, err := stores.NewLocalNodeDecorator(acir.settings.Logger, acir.config.ContainerOrchestrator,
		hostInfo, hostName, stores.WithECSInfo(ecsInfo))
	if err != nil {
		return err
	}
	acir.decorators = append(acir.decorators, localNodeDecorator)

	acir.containerMetricsProvider, err = cadvisor.New(acir.config.ContainerOrchestrator, hostInfo,
		acir.settings.Logger, cadvisor.WithECSInfoCreator(ecsInfo), cadvisor.WithDecorator(localNodeDecorator))
	if err != nil {
		return err
	}
	return nil
}

func (acir *awsContainerInsightReceiver) start(ctx context.Context) {
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
}

func (acir *awsContainerInsightReceiver) initPrometheusScraper(ctx context.Context, host component.Host, hostInfo *hostinfo.Info, leaderElection *k8sapiserver.LeaderElection) error {
	if !acir.config.EnableControlPlaneMetrics {
		return nil
	}

	endpoint, err := acir.getK8sAPIServerEndpoint()
	if err != nil {
		return err
	}

	acir.settings.Logger.Debug("kube apiserver endpoint found", zap.String("endpoint", endpoint))
	// use the same leader

	restConfig, err := rest.InClusterConfig()
	if err != nil {
		return err
	}
	bearerToken := restConfig.BearerToken
	if bearerToken == "" {
		return errors.New("bearer token was empty")
	}

	acir.prometheusScraper, err = k8sapiserver.NewPrometheusScraper(k8sapiserver.PrometheusScraperOpts{
		Ctx:                 ctx,
		TelemetrySettings:   acir.settings,
		Endpoint:            endpoint,
		Consumer:            acir.nextConsumer,
		Host:                host,
		ClusterNameProvider: hostInfo,
		LeaderElection:      leaderElection,
		BearerToken:         bearerToken,
	})
	return err
}

func (acir *awsContainerInsightReceiver) initDcgmScraper(ctx context.Context, host component.Host, hostInfo *hostinfo.Info, localNodeDecorator stores.Decorator) error {
	if !acir.config.EnableAcceleratedComputeMetrics {
		return nil
	}

	decoConsumer := decoratorconsumer.DecorateConsumer{
		ContainerOrchestrator: ci.EKS,
		NextConsumer:          acir.nextConsumer,
		MetricType:            ci.TypeContainerGPU,
		MetricToUnitMap:       gpu.MetricToUnit,
		K8sDecorator:          localNodeDecorator,
		Logger:                acir.settings.Logger,
	}

	scraperOpts := prometheusscraper.SimplePrometheusScraperOpts{
		Ctx:               ctx,
		TelemetrySettings: acir.settings,
		Consumer:          &decoConsumer,
		Host:              host,
		ScraperConfigs:    gpu.GetScraperConfig(hostInfo),
		HostInfoProvider:  hostInfo,
		Logger:            acir.settings.Logger,
	}

	var err error
	acir.dcgmScraper, err = prometheusscraper.NewSimplePrometheusScraper(scraperOpts)
	return err
}

func (acir *awsContainerInsightReceiver) initPodResourcesStore() error {
	var err error
	acir.podResourcesStore, err = stores.NewPodResourcesStore(acir.settings.Logger)
	return err
}

func (acir *awsContainerInsightReceiver) initNeuronScraper(ctx context.Context, host component.Host, hostInfo *hostinfo.Info, localNodeDecorator stores.Decorator) error {
	if !acir.config.EnableAcceleratedComputeMetrics {
		return nil
	}
	var err error

	decoConsumer := decoratorconsumer.DecorateConsumer{
		ContainerOrchestrator: ci.EKS,
		NextConsumer:          acir.nextConsumer,
		MetricType:            ci.TypeContainerNeuron,
		K8sDecorator:          localNodeDecorator,
		Logger:                acir.settings.Logger,
	}

	emptyMetricDecoratorConsumer := neuron.EmptyMetricDecorator{
		NextConsumer: &decoConsumer,
		Logger:       acir.settings.Logger,
	}

	if acir.podResourcesStore == nil {
		return errors.New("pod resources store was not initialized")
	}

	acir.podResourcesStore.AddResourceName("aws.amazon.com/neuroncore")
	acir.podResourcesStore.AddResourceName("aws.amazon.com/neuron")
	acir.podResourcesStore.AddResourceName("aws.amazon.com/neurondevice")

	podAttributesDecoratorConsumer := neuron.PodAttributesDecoratorConsumer{
		NextConsumer:      &emptyMetricDecoratorConsumer,
		PodResourcesStore: acir.podResourcesStore,
		Logger:            acir.settings.Logger,
	}

	scraperOpts := prometheusscraper.SimplePrometheusScraperOpts{
		Ctx:               ctx,
		TelemetrySettings: acir.settings,
		Consumer:          &podAttributesDecoratorConsumer,
		Host:              host,
		ScraperConfigs:    neuron.GetNeuronScrapeConfig(hostInfo),
		HostInfoProvider:  hostInfo,
		Logger:            acir.settings.Logger,
	}

	acir.neuronMonitorScraper, err = prometheusscraper.NewSimplePrometheusScraper(scraperOpts)
	return err
}

func (acir *awsContainerInsightReceiver) initEfaSysfsScraper(localNodeDecorator stores.Decorator) error {
	if !acir.config.EnableAcceleratedComputeMetrics {
		return nil
	}

	if acir.podResourcesStore == nil {
		return errors.New("pod resources store was not initialized")
	}
	acir.efaSysfsScraper = efa.NewEfaSyfsScraper(acir.settings.Logger, localNodeDecorator, acir.podResourcesStore)
	return nil
}

// Shutdown stops the awsContainerInsightReceiver receiver.
func (acir *awsContainerInsightReceiver) Shutdown(context.Context) error {
	if acir.prometheusScraper != nil {
		acir.prometheusScraper.Shutdown() //nolint:errcheck
	}

	if acir.cancel == nil {
		return nil
	}
	acir.cancel()

	var errs error

	if acir.k8sapiserver != nil {
		errs = errors.Join(errs, acir.k8sapiserver.Shutdown())
	}
	if acir.containerMetricsProvider != nil {
		errs = errors.Join(errs, acir.containerMetricsProvider.Shutdown())
	}
	if acir.dcgmScraper != nil {
		acir.dcgmScraper.Shutdown()
	}
	if acir.neuronMonitorScraper != nil {
		acir.neuronMonitorScraper.Shutdown()
	}
	if acir.efaSysfsScraper != nil {
		acir.efaSysfsScraper.Shutdown()
	}
	if acir.decorators != nil {
		for i := len(acir.decorators) - 1; i >= 0; i-- {
			errs = errors.Join(errs, acir.decorators[i].Shutdown())
		}
	}

	if acir.podResourcesStore != nil {
		acir.podResourcesStore.Shutdown()
	}

	return errs
}

// collectData collects container stats from cAdvisor and k8s api server (if it is an elected leader)
func (acir *awsContainerInsightReceiver) collectData(ctx context.Context) error {
	var mds []pmetric.Metrics

	if acir.containerMetricsProvider == nil && acir.k8sapiserver == nil {
		err := errors.New("both cadvisor and k8sapiserver failed to start")
		acir.settings.Logger.Error("Failed to collect stats", zap.Error(err))
		return err
	}

	if acir.containerMetricsProvider != nil {
		mds = append(mds, acir.containerMetricsProvider.GetMetrics()...)
	}

	if acir.k8sapiserver != nil {
		mds = append(mds, acir.k8sapiserver.GetMetrics()...)
	}

	if acir.prometheusScraper != nil {
		// this does not return any metrics, it just indirectly ensures scraping is running on a leader
		acir.prometheusScraper.GetMetrics() //nolint:errcheck
	}

	if acir.dcgmScraper != nil {
		acir.dcgmScraper.GetMetrics() //nolint:errcheck
	}

	if acir.neuronMonitorScraper != nil {
		acir.neuronMonitorScraper.GetMetrics() //nolint:errcheck
	}

	if acir.efaSysfsScraper != nil {
		mds = append(mds, acir.efaSysfsScraper.GetMetrics()...)
	}

	for _, md := range mds {
		err := acir.nextConsumer.ConsumeMetrics(ctx, md)
		if err != nil {
			return err
		}
	}

	return nil
}

func (acir *awsContainerInsightReceiver) getK8sAPIServerEndpoint() (string, error) {
	k8sClient := k8sclient.Get(acir.settings.Logger)
	if k8sClient == nil {
		return "", errors.New("cannot start k8s client, unable to find K8sApiServer endpoint")
	}
	endpoint := k8sClient.GetClientSet().CoreV1().RESTClient().Get().AbsPath("/").URL().Hostname()

	return endpoint, nil
}

func waitForKubelet(ctx context.Context, client *kubeletutil.KubeletClient, logger *zap.Logger) error {
	for {
		err := checkKubelet(client)
		if err == nil {
			return nil
		}
		logger.Debug("Kubelet unavailable. Waiting for next interval", zap.Error(err), zap.Stringer("interval", waitForKubeletInterval))
		select {
		case <-time.After(waitForKubeletInterval):
			continue
		case <-ctx.Done():
			return fmt.Errorf("context closed without getting kubelet client: %w", ctx.Err())
		}
	}
}

func checkKubelet(client *kubeletutil.KubeletClient) error {
	// Try to detect kubelet permission issue here
	if _, err := client.ListPods(); err != nil {
		return fmt.Errorf("cannot get pods from kubelet: %w", err)
	}
	return nil
}
