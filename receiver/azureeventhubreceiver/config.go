// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureeventhubreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azureeventhubreceiver"

import (
	"errors"
	"fmt"

	"github.com/Azure/azure-amqp-common-go/v4/conn"
	"go.opentelemetry.io/collector/component"
)

type logFormat string

const (
	defaultLogFormat logFormat = ""
	rawLogFormat     logFormat = "raw"
	azureLogFormat   logFormat = "azure"
)

var (
	validFormats         = []logFormat{defaultLogFormat, rawLogFormat, azureLogFormat}
	errMissingConnection = errors.New("missing connection")
)

type Config struct {
	Connection    string        `mapstructure:"connection"`
	Partition     string        `mapstructure:"partition"`
	Offset        string        `mapstructure:"offset"`
	StorageID     *component.ID `mapstructure:"storage"`
	Format        string        `mapstructure:"format"`
	ConsumerGroup string        `mapstructure:"group"`
}

func isValidFormat(format string) bool {
	for _, validFormat := range validFormats {
		if logFormat(format) == validFormat {
			return true
		}
	}
	return false
}

// Validate config
func (config *Config) Validate() error {
	if config.Connection == "" {
		return errMissingConnection
	}
	if _, err := conn.ParsedConnectionFromStr(config.Connection); err != nil {
		return err
	}
	if !isValidFormat(config.Format) {
		return fmt.Errorf("invalid format; must be one of %#v", validFormats)
	}
	return nil
}
