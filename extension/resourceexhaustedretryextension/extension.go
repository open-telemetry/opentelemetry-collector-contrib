// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package resourceexhaustedretryextension

import (
	"context"

	"go.opentelemetry.io/collector/component"
)

type resourceExhaustedRetryExtension struct {
	cfg *Config
}

func newExtension(cfg *Config) *resourceExhaustedRetryExtension {
	return &resourceExhaustedRetryExtension{cfg: cfg}
}

func (e *resourceExhaustedRetryExtension) Start(_ context.Context, _ component.Host) error {
	return nil
}

func (e *resourceExhaustedRetryExtension) Shutdown(_ context.Context) error {
	return nil
}
