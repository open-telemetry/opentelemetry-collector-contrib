// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ackextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/ackextension"
import (
	"go.opentelemetry.io/collector/component"
)

// Config defines configuration for ack extension
type Config struct {
	// StorageID defines the storage type of the extension. In-memory type is set by default (if not provided). Future consideration is disk type.
	StorageID *component.ID `mapstructure:"storage"`
}
