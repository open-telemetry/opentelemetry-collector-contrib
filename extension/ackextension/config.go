// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ackextension

// Config defines configuration for ack extension
type Config struct {
	// StorageType defines the storage type of the extension. Currently planned for In memory type. Future consideration is disk type.
	StorageType string `mapstructure:"StorageType,omitempty"`
}
