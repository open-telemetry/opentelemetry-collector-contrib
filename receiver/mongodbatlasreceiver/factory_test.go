// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package mongodbatlasreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbatlasreceiver"

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbatlasreceiver/internal/metadata"
)

func TestType(t *testing.T) {
	factory := NewFactory()
	ft := factory.Type()
	require.Equal(t, metadata.Type, ft)
}

func TestBadAlertsReceiver(t *testing.T) {
	conf := createDefaultConfig()
	cfg := conf.(*Config)

	cfg.Alerts.Enabled = true
	cfg.Alerts.TLS = &configtls.ServerConfig{
		ClientCAFile: "/not/a/file",
	}
	params := receivertest.NewNopSettings(metadata.Type)

	_, err := createCombinedLogReceiver(context.Background(), params, cfg, consumertest.NewNop())
	require.Error(t, err)
	require.ErrorContains(t, err, "unable to create a MongoDB Atlas Alerts Receiver")
}

func TestBadStorageExtension(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.StorageID = &component.ID{}
	cfg.Events = &EventsConfig{
		Projects: []*ProjectConfig{
			{
				Name: testProjectName,
			},
		},
		PollInterval: time.Minute,
	}

	params := receivertest.NewNopSettings(metadata.Type)
	lr, err := createCombinedLogReceiver(context.Background(), params, cfg, consumertest.NewNop())
	require.NoError(t, err)

	err = lr.Start(context.Background(), componenttest.NewNopHost())
	require.ErrorContains(t, err, "failed to get storage client")
}
