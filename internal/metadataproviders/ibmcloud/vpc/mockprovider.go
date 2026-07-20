// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package vpc // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/metadataproviders/ibmcloud/vpc"

import (
	"context"

	"github.com/stretchr/testify/mock"
)

type MockProvider struct {
	mock.Mock
}

var _ Provider = (*MockProvider)(nil)

func (m *MockProvider) InstanceMetadata(_ context.Context) (*InstanceMetadata, error) {
	args := m.MethodCalled("InstanceMetadata")
	arg := args.Get(0)
	var im *InstanceMetadata
	if arg != nil {
		im = arg.(*InstanceMetadata)
	}
	return im, args.Error(1)
}
