// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package loadbalancingexporter

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/servicediscovery"
	"github.com/aws/aws-sdk-go-v2/service/servicediscovery/types"
	"github.com/aws/smithy-go/middleware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

var instanceID = "test"
var namespaceName = "cloudmap"
var statusFilterHealthy = types.HealthStatusFilterHealthy

var port uint16 = 1234

func TestInitialCloudMapResolution(t *testing.T) {
	// prepare
	_, tb := getTelemetryAssets(t)

	res := &cloudMapResolver{
		logger:        zap.NewNop(),
		namespaceName: &namespaceName,
		serviceName:   &instanceID,
		healthStatus:  &statusFilterHealthy,
		resInterval:   5 * time.Second,
		resTimeout:    1 * time.Second,
		stopCh:        make(chan struct{}),
		discoveryFn:   mockDiscovery,
		telemetry:     tb,
	}

	// test
	var resolved []string
	res.onChange(func(endpoints []string) {
		resolved = endpoints
	})
	require.NoError(t, res.start(context.Background()))
	defer func() {
		require.NoError(t, res.shutdown(context.Background()))
	}()

	// verify
	assert.Len(t, resolved, 3)
	for i, value := range []string{"127.0.0.1:8080", "127.0.0.2:8080", "127.0.0.3:8080"} {
		assert.Equal(t, value, resolved[i])
	}
}

func TestInitialCloudMapResolutionWithPort(t *testing.T) {
	// prepare
	_, tb := getTelemetryAssets(t)

	res := &cloudMapResolver{
		logger:        zap.NewNop(),
		namespaceName: &namespaceName,
		serviceName:   &instanceID,
		port:          &port,
		healthStatus:  &statusFilterHealthy,
		resInterval:   5 * time.Second,
		resTimeout:    1 * time.Second,
		stopCh:        make(chan struct{}),
		discoveryFn:   mockDiscovery,
		telemetry:     tb,
	}

	// test
	var resolved []string
	res.onChange(func(endpoints []string) {
		resolved = endpoints
	})
	require.NoError(t, res.start(context.Background()))
	defer func() {
		require.NoError(t, res.shutdown(context.Background()))
	}()

	// verify
	assert.Len(t, resolved, 3)
	for i, value := range []string{"127.0.0.1:1234", "127.0.0.2:1234", "127.0.0.3:1234"} {
		assert.Equal(t, value, resolved[i])
	}
}

func makeSummary(i int) types.HttpInstanceSummary {
	return types.HttpInstanceSummary{
		Attributes: map[string]string{
			"AWS_INSTANCE_IPV4": fmt.Sprintf("127.0.0.%d", i),
			"AWS_INSTANCE_PORT": "8080",
		},
		HealthStatus:  types.HealthStatusHealthy,
		InstanceId:    &instanceID,
		NamespaceName: nil,
		ServiceName:   nil,
	}
}
func mockDiscovery(*servicediscovery.DiscoverInstancesInput) (*servicediscovery.DiscoverInstancesOutput, error) {
	s := &servicediscovery.DiscoverInstancesOutput{
		Instances: []types.HttpInstanceSummary{
			makeSummary(1),
			makeSummary(2),
			makeSummary(3),
		},
		ResultMetadata: middleware.Metadata{},
	}
	return s, nil
}
