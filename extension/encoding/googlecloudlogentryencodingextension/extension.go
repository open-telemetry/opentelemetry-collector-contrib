// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package googlecloudlogentryencodingextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/googlecloudlogentryencodingextension"

import (
	"context"
	"errors"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding"
)

var _ encoding.LogsUnmarshalerExtension = (*ext)(nil)

type ext struct {
	Config Config
}

func newExtension(cfg *Config) (*ext, error) {
	if cfg == nil {
		// this check is to keep the function signature the same as the real function (lint)
		return nil, errors.New("nil Config")
	}
	return &ext{Config: *cfg}, nil
}

func (ex *ext) Start(_ context.Context, _ component.Host) error {
	return nil
}

func (ex *ext) Shutdown(_ context.Context) error {
	return nil
}

func (ex *ext) UnmarshalLogs(_ []byte) (plog.Logs, error) {
	out := plog.NewLogs()
	return out, nil
}
