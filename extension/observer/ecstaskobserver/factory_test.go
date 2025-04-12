// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ecstaskobserver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/extension/extensiontest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/endpointswatcher"
)

func TestFactoryCreatedExtensionIsEndpointsLister(t *testing.T) {
	etoFactory := NewFactory()
	cfg := etoFactory.CreateDefaultConfig()
	cfg.(*Config).Endpoint = "http://localhost:1234/mock/endpoint"

	eto, err := etoFactory.Create(context.Background(), extensiontest.NewNopSettings(etoFactory.Type()), cfg)
	require.NoError(t, err)
	require.NotNil(t, eto)
	require.Implements(t, (*endpointswatcher.EndpointsLister)(nil), eto)
}
