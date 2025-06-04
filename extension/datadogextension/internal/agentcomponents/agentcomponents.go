// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// package agentcomponents will define the components imported from datadog-agent repository to be used in the extension
package agentcomponents // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/datadogextension/internal/agentcomponents"

import (
	"compress/gzip"
	"strings"

	coreconfig "github.com/DataDog/datadog-agent/comp/core/config"
	corelog "github.com/DataDog/datadog-agent/comp/core/log/def"
	"github.com/DataDog/datadog-agent/comp/forwarder/defaultforwarder"
	pkgconfigmodel "github.com/DataDog/datadog-agent/pkg/config/model"
	"github.com/DataDog/datadog-agent/pkg/config/utils"
	"github.com/DataDog/datadog-agent/pkg/config/viperconfig"
	"github.com/DataDog/datadog-agent/pkg/serializer"
	"github.com/DataDog/datadog-agent/pkg/util/compression"
	"github.com/DataDog/datadog-agent/pkg/util/compression/selector"
	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog"
)

// NewLogComponent creates a new log component using the provided telemetry settings
func NewLogComponent(set component.TelemetrySettings) corelog.Component {
	zlog := &datadog.Zaplogger{
		Logger: set.Logger,
	}
	return zlog
}

// NewForwarder creates a new forwarder that sends payloads to Datadog backend
func NewForwarder(cfg coreconfig.Component, log corelog.Component) defaultforwarder.Forwarder {
	keysPerDomain := map[string][]utils.APIKeys{
		"https://api." + cfg.GetString("site"): {utils.NewAPIKeys("api_key", cfg.GetString("api_key"))},
	}
	forwarderOptions := defaultforwarder.NewOptions(cfg, log, keysPerDomain)
	forwarderOptions.DisableAPIKeyChecking = true
	return defaultforwarder.NewDefaultForwarder(cfg, log, forwarderOptions)
}

// NewCompressor creates a new compressor with Gzip strategy, best compression
func NewCompressor() compression.Compressor {
	return selector.NewCompressor(compression.GzipKind, gzip.BestCompression)
}

// NewSerializer creates a new serializer that serializes payloads prior to being forwarded
func NewSerializer(fwd defaultforwarder.Forwarder, cmp compression.Compressor, cfg coreconfig.Component, logger corelog.Component, hostname string) *serializer.Serializer {
	return serializer.NewSerializer(fwd, nil, cmp, cfg, logger, hostname)
}

// NewConfigComponent creates a new config component required to use forwarder and serializer components
func NewConfigComponent(set component.TelemetrySettings, key, site string) coreconfig.Component {
	pkgconfig := viperconfig.NewConfig("DD", "DD", strings.NewReplacer(".", "_"))

	// Set the API Key
	pkgconfig.Set("api_key", key, pkgconfigmodel.SourceFile)
	pkgconfig.Set("site", site, pkgconfigmodel.SourceFile)
	pkgconfig.Set("logs_enabled", true, pkgconfigmodel.SourceDefault)
	pkgconfig.Set("log_level", set.Logger.Level().String(), pkgconfigmodel.SourceFile)
	// Set values for serializer
	pkgconfig.Set("enable_payloads.events", true, pkgconfigmodel.SourceDefault)
	pkgconfig.Set("enable_payloads.json_to_v1_intake", true, pkgconfigmodel.SourceDefault)
	pkgconfig.Set("enable_sketch_stream_payload_serialization", true, pkgconfigmodel.SourceDefault)
	pkgconfig.Set("forwarder_apikey_validation_interval", 60, pkgconfigmodel.SourceDefault)
	pkgconfig.Set("forwarder_num_workers", 1, pkgconfigmodel.SourceDefault)
	pkgconfig.Set("logging_frequency", 2, pkgconfigmodel.SourceDefault)
	pkgconfig.Set("forwarder_backoff_factor", 2, pkgconfigmodel.SourceDefault)
	pkgconfig.Set("forwarder_backoff_base", 2, pkgconfigmodel.SourceDefault)
	pkgconfig.Set("forwarder_backoff_max", 64, pkgconfigmodel.SourceDefault)
	pkgconfig.Set("forwarder_recovery_interval", 2, pkgconfigmodel.SourceDefault)
	return pkgconfig
}
