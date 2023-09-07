// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otelarrowreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/otelarrowreceiver"

import (
	"github.com/open-telemetry/otel-arrow/collector/receiver/otelarrowreceiver"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
)

// Config defines configuration for OTel Arrow receiver.
type Config = otelarrowreceiver.Config

var _ component.Config = (*Config)(nil)
var _ confmap.Unmarshaler = (*Config)(nil)
