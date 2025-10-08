// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureeventhubreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azureeventhubreceiver"

import (
	"errors"
	"fmt"
	"slices"

	"github.com/Azure/azure-amqp-common-go/v4/conn"
	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azeventhubs/v2"
	"go.opentelemetry.io/collector/component"
)

type logFormat string

const (
	defaultLogFormat logFormat = ""
	rawLogFormat     logFormat = "raw"
	azureLogFormat   logFormat = "azure"
)

var (
	validFormats           = []logFormat{defaultLogFormat, rawLogFormat, azureLogFormat}
	errMissingConnection   = errors.New("missing connection")
	errFeatureGateRequired = fmt.Errorf("poll_rate and max_poll_events can only be used with %s enabled", azEventHubFeatureGateName)
)

type Config struct {
	Connection               string        `mapstructure:"connection"`
	Partition                string        `mapstructure:"partition"`
	Offset                   string        `mapstructure:"offset"`
	StorageID                *component.ID `mapstructure:"storage"`
	Format                   string        `mapstructure:"format"`
	ConsumerGroup            string        `mapstructure:"group"`
	ApplySemanticConventions bool          `mapstructure:"apply_semantic_conventions"`
	TimeFormats              TimeFormat    `mapstructure:"time_formats"`

	// azeventhub lib specific
	PollRate      int `mapstructure:"poll_rate"`
	MaxPollEvents int `mapstructure:"max_poll_events"`
}

type TimeFormat struct {
	Logs    []string `mapstructure:"logs"`
	Metrics []string `mapstructure:"metrics"`
	Traces  []string `mapstructure:"traces"`
}

// Validate config
func (config *Config) Validate() error {
	if !azEventHubFeatureGate.IsEnabled() &&
		(config.PollRate != 0 || config.MaxPollEvents != 0) {
		return errFeatureGateRequired
	}
	if config.Connection == "" {
		return errMissingConnection
	}
	if azEventHubFeatureGate.IsEnabled() {
		if _, err := azeventhubs.ParseConnectionString(config.Connection); err != nil {
			return err
		}
	} else {
		if _, err := conn.ParsedConnectionFromStr(config.Connection); err != nil {
			return err
		}
	}
	if !slices.Contains(validFormats, logFormat(config.Format)) {
		return fmt.Errorf("invalid format; must be one of %#v", validFormats)
	}
	if config.Partition == "" && config.Offset != "" {
		return errors.New("cannot use 'offset' without 'partition'")
	}
	return nil
}
