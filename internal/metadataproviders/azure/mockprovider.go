// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azure // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/metadataproviders/azure"

import (
	"context"

	"github.com/stretchr/testify/mock"
)

type MockProvider struct {
	mock.Mock
}

func (m *MockProvider) Metadata(_ context.Context) (*ComputeMetadata, error) {
	args := m.MethodCalled("Metadata")
	arg := args.Get(0)
	var cm *ComputeMetadata
	if arg != nil {
		cm = arg.(*ComputeMetadata)
	}
	return cm, args.Error(1)
}
