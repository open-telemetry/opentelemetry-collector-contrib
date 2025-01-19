// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package loadbalancingexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter"

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sort"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/servicediscovery"
	"github.com/aws/aws-sdk-go-v2/service/servicediscovery/types"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter/internal/metadata"
)

const (
	defaultAwsResInterval = 30 * time.Second
	defaultAwsResTimeout  = 5 * time.Second
)

var (
	errNoNamespace   = errors.New("no Cloud Map namespace specified to resolve the backends")
	errNoServiceName = errors.New("no Cloud Map service_name specified to resolve the backends")

	awsResolverAttr           = attribute.String("resolver", "aws")
	awsResolverAttrSet        = attribute.NewSet(awsResolverAttr)
	awsResolverSuccessAttrSet = attribute.NewSet(awsResolverAttr, attribute.Bool("success", true))
	awsResolverFailureAttrSet = attribute.NewSet(awsResolverAttr, attribute.Bool("success", false))
)

func createDiscoveryFunction(client *servicediscovery.Client) func(params *servicediscovery.DiscoverInstancesInput) (*servicediscovery.DiscoverInstancesOutput, error) {
	return func(params *servicediscovery.DiscoverInstancesInput) (*servicediscovery.DiscoverInstancesOutput, error) {
		return client.DiscoverInstances(context.TODO(), params)
	}
}

type cloudMapResolver struct {
	logger *zap.Logger

	namespaceName *string
	serviceName   *string
	port          *uint16
	healthStatus  *types.HealthStatusFilter
	resInterval   time.Duration
	resTimeout    time.Duration

	endpoints         []string
	onChangeCallbacks []func([]string)

	stopCh             chan struct{}
	updateLock         sync.Mutex
	shutdownWg         sync.WaitGroup
	changeCallbackLock sync.RWMutex
	discoveryFn        func(params *servicediscovery.DiscoverInstancesInput) (*servicediscovery.DiscoverInstancesOutput, error)
	telemetry          *metadata.TelemetryBuilder
}

func newCloudMapResolver(
	logger *zap.Logger,
	namespaceName *string,
	serviceName *string,
	port *uint16,
	healthStatus *types.HealthStatusFilter,
	interval time.Duration,
	timeout time.Duration,
	tb *metadata.TelemetryBuilder,
) (*cloudMapResolver, error) { // Using the SDK's default configuration, loading additional config
	// and credentials values from the environment variables, shared
	// credentials, and shared configuration files
	cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithDefaultRegion("us-east-1"))
	if err != nil {
		log.Fatalf("unable to load SDK config, %v", err)
		return nil, err
	}

	// Using the Config value, create the DynamoDB client
	svc := servicediscovery.NewFromConfig(cfg)

	if namespaceName == nil || len(*namespaceName) == 0 {
		return nil, errNoNamespace
	}

	if serviceName == nil || len(*serviceName) == 0 {
		return nil, errNoServiceName
	}

	if interval == 0 {
		interval = defaultAwsResInterval
	}
	if timeout == 0 {
		timeout = defaultAwsResTimeout
	}

	if healthStatus == nil {
		healthStatusFilter := types.HealthStatusFilterHealthy
		healthStatus = &healthStatusFilter
	}

	return &cloudMapResolver{
		logger:        logger,
		namespaceName: namespaceName,
		serviceName:   serviceName,
		port:          port,
		healthStatus:  healthStatus,
		resInterval:   interval,
		resTimeout:    timeout,
		stopCh:        make(chan struct{}),
		discoveryFn:   createDiscoveryFunction(svc),
		telemetry:     tb,
	}, nil
}

func (r *cloudMapResolver) start(ctx context.Context) error {
	if _, err := r.resolve(ctx); err != nil {
		r.logger.Warn("failed initial resolve", zap.Error(err))
	}

	go r.periodicallyResolve()

	r.logger.Info("AWS CloudMap resolver started",
		zap.Stringp("service_name", r.serviceName),
		zap.Stringp("namespaceName", r.namespaceName),
		zap.Uint16p("port", r.port),
		zap.String("health_status", string(*r.healthStatus)),
		zap.Duration("interval", r.resInterval), zap.Duration("timeout", r.resTimeout))
	return nil
}

func (r *cloudMapResolver) shutdown(_ context.Context) error {
	r.changeCallbackLock.Lock()
	r.onChangeCallbacks = nil
	r.changeCallbackLock.Unlock()

	close(r.stopCh)
	r.shutdownWg.Wait()
	return nil
}

func (r *cloudMapResolver) periodicallyResolve() {
	ticker := time.NewTicker(r.resInterval)

	for {
		select {
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), r.resTimeout)
			if _, err := r.resolve(ctx); err != nil {
				r.logger.Warn("failed to resolve", zap.Error(err))
			} else {
				r.logger.Debug("resolved successfully")
			}
			cancel()
		case <-r.stopCh:
			return
		}
	}
}

func (r *cloudMapResolver) resolve(ctx context.Context) ([]string, error) {
	r.shutdownWg.Add(1)
	defer r.shutdownWg.Done()

	discoverInstancesOutput, err := r.discoveryFn(&servicediscovery.DiscoverInstancesInput{
		NamespaceName:      r.namespaceName,
		ServiceName:        r.serviceName,
		HealthStatus:       *r.healthStatus,
		MaxResults:         nil,
		OptionalParameters: nil,
		QueryParameters:    nil,
	})
	if err != nil {
		r.telemetry.LoadbalancerNumResolutions.Add(ctx, 1, metric.WithAttributeSet(awsResolverFailureAttrSet))
		return nil, err
	}

	r.telemetry.LoadbalancerNumResolutions.Add(ctx, 1, metric.WithAttributeSet(awsResolverSuccessAttrSet))

	r.logger.Debug("resolver has discovered instances ",
		zap.Int("Instance Count", len(discoverInstancesOutput.Instances)))

	var backends []string
	for _, instance := range discoverInstancesOutput.Instances {
		ipAddr := instance.Attributes["AWS_INSTANCE_IPV4"]
		var endpoint string
		if r.port == nil {
			ipPort := instance.Attributes["AWS_INSTANCE_PORT"]
			endpoint = fmt.Sprintf("%s:%s", ipAddr, ipPort)
		} else {
			endpoint = fmt.Sprintf("%s:%d", ipAddr, *r.port)
		}
		r.logger.Debug("resolved instance",
			zap.String("Endpoint", endpoint))
		backends = append(backends, endpoint)
	}

	// keep it always in the same order
	sort.Strings(backends)

	if equalStringSlice(r.endpoints, backends) {
		return r.endpoints, nil
	}

	// the list has changed!
	r.updateLock.Lock()
	r.endpoints = backends
	r.updateLock.Unlock()
	r.telemetry.LoadbalancerNumBackends.Record(ctx, int64(len(backends)), metric.WithAttributeSet(awsResolverAttrSet))
	r.telemetry.LoadbalancerNumBackendUpdates.Add(ctx, 1, metric.WithAttributeSet(awsResolverAttrSet))

	// propagate the change
	r.changeCallbackLock.RLock()
	for _, callback := range r.onChangeCallbacks {
		callback(r.endpoints)
	}
	r.changeCallbackLock.RUnlock()

	return r.endpoints, nil
}

func (r *cloudMapResolver) onChange(f func([]string)) {
	r.changeCallbackLock.Lock()
	defer r.changeCallbackLock.Unlock()
	r.onChangeCallbacks = append(r.onChangeCallbacks, f)
}
