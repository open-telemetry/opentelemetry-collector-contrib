// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !aix && !freebsd && !solaris

package datadogextension

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/extension/extensiontest"
	"go.uber.org/zap"
)

func TestRCIsOptionalAndUsesExtensionCredentials(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		t.Run(map[bool]string{false: "disabled", true: "enabled"}[enabled], func(t *testing.T) {
			f := NewFactory()
			cfg := f.CreateDefaultConfig().(*Config)
			cfg.API.Key = "0123456789abcdef0123456789abcdef"
			cfg.API.FailOnInvalidKey = false
			cfg.Hostname = "collector-test"
			cfg.RemoteConfig.Enabled = enabled
			cfg.RemoteConfig.CacheDirectory = t.TempDir()
			require.NoError(t, cfg.Validate())
			ext, err := f.Create(t.Context(), extensiontest.NewNopSettings(component.MustNewType("datadog")), cfg)
			require.NoError(t, err)
			e := ext.(*datadogExtension)
			require.Equal(t, enabled, e.rc != nil)
			require.Equal(t, cfg.API.Key, cfg.rcConfig().API.Key)
			require.Equal(t, cfg.API.Site, cfg.rcConfig().API.Site)
			require.NoError(t, e.Shutdown(t.Context()))
		})
	}
}

type lifecycleRC struct {
	started, stopped bool
	startErr         error
}

func (r *lifecycleRC) Start(context.Context, component.Host) error {
	r.started = true
	return r.startErr
}
func (r *lifecycleRC) Shutdown(context.Context) error { r.stopped = true; return nil }

func TestExtensionOwnsRCLifecycle(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{false: "success", true: "start failure"}[fail], func(t *testing.T) {
			rc := &lifecycleRC{}
			if fail {
				rc.startErr = errors.New("RC listener unavailable")
			}
			e := &datadogExtension{rc: rc, info: &info{}, logger: zap.NewNop()}
			err := e.Start(t.Context(), componenttest.NewNopHost())
			if fail {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.True(t, rc.started)
			require.NoError(t, e.Shutdown(t.Context()))
			require.Equal(t, !fail, rc.stopped)
		})
	}
}
