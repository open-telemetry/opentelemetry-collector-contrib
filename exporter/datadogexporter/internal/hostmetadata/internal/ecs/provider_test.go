// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ecs // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/hostmetadata/internal/ecs"

import (
	"context"
	"fmt"
	"testing"

	"github.com/DataDog/opentelemetry-mapping-go/pkg/otlp/attributes/source"
	"github.com/stretchr/testify/assert"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/ecsutil"
)

var _ ecsutil.MetadataProvider = (*mockProvider)(nil)

type mockProvider struct {
	metadata *ecsutil.TaskMetadata
	err      error
}

func (*mockProvider) FetchContainerMetadata() (*ecsutil.ContainerMetadata, error) {
	return nil, fmt.Errorf("not implemented")
}

func (p *mockProvider) FetchTaskMetadata() (*ecsutil.TaskMetadata, error) {
	return p.metadata, p.err
}

func newMock(metadata *ecsutil.TaskMetadata, err error) ecsutil.MetadataProvider {
	return &mockProvider{metadata, err}
}

func TestECSProvider(t *testing.T) {
	tests := []struct {
		name            string
		provider        ecsutil.MetadataProvider
		missingEndpoint bool

		onECSFargate bool
		onErr        string

		src    source.Source
		srcErr string
	}{
		{
			name:            "missing endpoint",
			missingEndpoint: true,

			onECSFargate: false,
			srcErr:       ErrNotOnECSFargate.Error(),
		},
		{
			name:     "On ECS EC2",
			provider: newMock(&ecsutil.TaskMetadata{LaunchType: "ec2"}, nil),

			onECSFargate: false,
			srcErr:       ErrNotOnECSFargate.Error(),
		},
		{
			name:     "endpoint does not have the launch type",
			provider: newMock(&ecsutil.TaskMetadata{}, nil),

			srcErr: "TMDE endpoint is queryable, but launch type is unavailable",
			onErr:  "TMDE endpoint is queryable, but launch type is unavailable",
		},
		{
			name:     "endpoint query failed",
			provider: newMock(nil, fmt.Errorf("network error")),

			onErr:  "failed to fetch task metadata: network error",
			srcErr: "failed to fetch task metadata: network error",
		},
		{
			name: "On ECS Fargate",
			provider: newMock(&ecsutil.TaskMetadata{
				LaunchType: "fargate",
				TaskARN:    "task-arn",
			}, nil),

			onECSFargate: true,
			src:          source.Source{Kind: source.AWSECSFargateKind, Identifier: "task-arn"},
		},
	}

	for _, testInstance := range tests {
		t.Run(testInstance.name, func(t *testing.T) {
			p := &Provider{
				missingEndpoint: testInstance.missingEndpoint,
				ecsMetadata:     testInstance.provider,
			}
			src, srcErr := p.Source(context.Background())
			if srcErr != nil || testInstance.srcErr != "" {
				assert.EqualError(t, srcErr, testInstance.srcErr)
			} else {
				assert.Equal(t, testInstance.src, src)
			}

			onECSFargate, onErr := p.OnECSFargate(context.Background())
			if onErr != nil || testInstance.onErr != "" {
				assert.EqualError(t, onErr, testInstance.onErr)
			} else {
				assert.Equal(t, testInstance.onECSFargate, onECSFargate)
			}
		})
	}
}
