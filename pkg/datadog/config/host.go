// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package config // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog/config"

import (
	"encoding"
	"fmt"
	"time"
)

// HostnameSource is the source for the hostname of host metadata.
type HostnameSource string

const (
	// HostnameSourceFirstResource picks the host metadata hostname from the resource
	// attributes on the first OTLP payload that gets to the exporter. If it is lacking any
	// hostname-like attributes, it will fallback to 'config_or_system' behavior (see below).
	//
	// Do not use this hostname source if receiving data from multiple hosts.
	HostnameSourceFirstResource HostnameSource = "first_resource"

	// HostnameSourceConfigOrSystem picks the host metadata hostname from the 'hostname' setting,
	// and if this is empty, from available system APIs and cloud provider endpoints.
	HostnameSourceConfigOrSystem HostnameSource = "config_or_system"
)

var _ encoding.TextUnmarshaler = (*HostnameSource)(nil)

// UnmarshalText implements the encoding.TextUnmarshaler interface.
func (sm *HostnameSource) UnmarshalText(in []byte) error {
	switch mode := HostnameSource(in); mode {
	case HostnameSourceFirstResource,
		HostnameSourceConfigOrSystem:
		*sm = mode
		return nil
	default:
		return fmt.Errorf("invalid host metadata hostname source %q", mode)
	}
}

// HostMetadataConfig defines the host metadata related configuration.
// Host metadata is the information used for populating the infrastructure list,
// the host map and providing host tags functionality.
//
// The exporter will send host metadata for a single host, whose name is chosen
// according to `host_metadata::hostname_source`.
type HostMetadataConfig struct {
	// Enabled enables the host metadata functionality.
	Enabled bool `mapstructure:"enabled"`

	// HostnameSource is the source for the hostname of host metadata.
	// This hostname is used for identifying the infrastructure list, host map and host tag information related to the host where the Datadog exporter is running.
	// Changing this setting will not change the host used to tag your metrics, traces and logs in any way.
	// For remote hosts, see https://docs.datadoghq.com/opentelemetry/schema_semantics/host_metadata/.
	//
	// Valid values are 'first_resource' and 'config_or_system':
	// - 'first_resource' picks the host metadata hostname from the resource
	//    attributes on the first OTLP payload that gets to the exporter.
	//    If the first payload lacks hostname-like attributes, it will fallback to 'config_or_system'.
	//    **Do not use this hostname source if receiving data from multiple hosts**.
	// - 'config_or_system' picks the host metadata hostname from the 'hostname' setting,
	//    If this is empty it will use available system APIs and cloud provider endpoints.
	//
	// The default is 'config_or_system'.
	HostnameSource HostnameSource `mapstructure:"hostname_source"`

	// Tags is a list of host tags.
	// These tags will be attached to telemetry signals that have the host metadata hostname.
	// To attach tags to telemetry signals regardless of the host, use a processor instead.
	Tags []string `mapstructure:"tags"`

	// ReporterPeriod is the period at which the host metadata reporter will run.
	ReporterPeriod time.Duration `mapstructure:"reporter_period"`
}
