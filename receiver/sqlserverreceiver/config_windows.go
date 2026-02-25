// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package sqlserverreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sqlserverreceiver"

import "errors"

func (cfg *Config) validateInstanceAndComputerName() error {
	if cfg.InstanceName != "" && cfg.ComputerName == "" {
		return errors.New("'instance_name' may not be specified without 'computer_name'")
	}
	if cfg.InstanceName == "" && cfg.ComputerName != "" {
		return errors.New("'computer_name' may not be specified without 'instance_name'")
	}

	return nil
}
