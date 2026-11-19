// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package coredistro

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestWarnRemoval(t *testing.T) {
	tests := []struct {
		name      string
		buildInfo component.BuildInfo
		wantWarn  bool
	}{
		{
			name:      "core distro",
			buildInfo: component.BuildInfo{Command: "otelcol", Description: "OpenTelemetry Collector"},
			wantWarn:  true,
		},
		{
			name:      "contrib distro",
			buildInfo: component.BuildInfo{Command: "otelcol-contrib", Description: "OpenTelemetry Collector Contrib"},
		},
		{
			name:      "k8s distro",
			buildInfo: component.BuildInfo{Command: "otelcol-k8s", Description: "OpenTelemetry Collector for Kubernetes"},
		},
		{
			name:      "custom distro named otelcol",
			buildInfo: component.BuildInfo{Command: "otelcol", Description: "Custom Collector"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			core, logs := observer.New(zap.WarnLevel)
			set := componenttest.NewNopTelemetrySettings()
			set.Logger = zap.New(core)

			WarnRemoval(set, tt.buildInfo, "v1.2.3", "Use foo instead.")

			assert.Equal(t, tt.wantWarn, IsCoreDistro(tt.buildInfo))
			if tt.wantWarn {
				assert.Equal(t, 1, logs.FilterMessage(RemovalWarning+" in v1.2.3. Use foo instead.").Len())
			} else {
				assert.Zero(t, logs.Len())
			}
		})
	}
}
