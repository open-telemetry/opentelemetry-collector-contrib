// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package windowseventlogreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/windowseventlogreceiver"

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/windowseventlogreceiver/internal/metadata"
)

func TestNewFactory(t *testing.T) {
	t.Run("NewFactoryCorrectType", func(t *testing.T) {
		factory := NewFactory()
		require.EqualValues(t, metadata.Type, factory.Type())
	})
}
