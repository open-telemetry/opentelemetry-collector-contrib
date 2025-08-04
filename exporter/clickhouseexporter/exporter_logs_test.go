// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package clickhouseexporter

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestLogsExporter_New(t *testing.T) {
	type validate func(*testing.T, *logsExporter, error)

	_ = func(t *testing.T, exporter *logsExporter, err error) {
		require.NoError(t, err)
		require.NotNil(t, exporter)
	}

	_ = func(want error) validate {
		return func(t *testing.T, exporter *logsExporter, err error) {
			require.Nil(t, exporter)
			require.ErrorIs(t, err, want, "Expected error '%v', but got '%v'", want, err)
		}
	}

	failWithMsg := func(msg string) validate {
		return func(t *testing.T, _ *logsExporter, err error) {
			require.ErrorContains(t, err, msg)
		}
	}

	tests := map[string]struct {
		config *Config
		want   validate
	}{
		"no dsn": {
			config: withDefaultConfig(),
			want:   failWithMsg("parse dsn address failed"),
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			var err error
			exporter := newLogsExporter(zap.NewNop(), test.config)

			if exporter != nil {
				err = errors.Join(err, exporter.start(context.Background(), nil))
				defer func() {
					require.NoError(t, exporter.shutdown(context.Background()))
				}()
			}

			test.want(t, exporter, err)
		})
	}
}
