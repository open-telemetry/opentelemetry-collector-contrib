// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datadogexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter"

import (
	"runtime"
	"strings"

	coreconfig "github.com/DataDog/datadog-agent/comp/core/config"
	corelog "github.com/DataDog/datadog-agent/comp/core/log/def"
	pkgconfigmodel "github.com/DataDog/datadog-agent/pkg/config/model"
	pkgconfigsetup "github.com/DataDog/datadog-agent/pkg/config/setup"
	"github.com/DataDog/datadog-agent/pkg/config/viperconfig"
	"go.opentelemetry.io/collector/component"
	"golang.org/x/net/http/httpproxy"

	pkgdatadog "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog"
	datadogconfig "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog/config"
)

func newLogComponent(set component.TelemetrySettings) corelog.Component {
	zlog := &pkgdatadog.Zaplogger{
		Logger: set.Logger,
	}
	return zlog
}

func newConfigComponent(set component.TelemetrySettings, cfg *datadogconfig.Config) coreconfig.Component {
	pkgconfig := viperconfig.NewConfig("DD", "DD", strings.NewReplacer(".", "_"))

	// Set the API Key
	pkgconfig.Set("api_key", string(cfg.API.Key), pkgconfigmodel.SourceFile)
	pkgconfig.Set("site", cfg.API.Site, pkgconfigmodel.SourceFile)
	pkgconfig.Set("logs_enabled", true, pkgconfigmodel.SourceDefault)
	pkgconfig.Set("log_level", set.Logger.Level().String(), pkgconfigmodel.SourceFile)
	pkgconfig.Set("logs_config.batch_wait", cfg.Logs.BatchWait, pkgconfigmodel.SourceFile)
	pkgconfig.Set("logs_config.use_compression", cfg.Logs.UseCompression, pkgconfigmodel.SourceFile)
	pkgconfig.Set("logs_config.compression_level", cfg.Logs.CompressionLevel, pkgconfigmodel.SourceFile)
	pkgconfig.Set("logs_config.logs_dd_url", cfg.Logs.TCPAddrConfig.Endpoint, pkgconfigmodel.SourceFile)
	pkgconfig.Set("logs_config.auditor_ttl", pkgconfigsetup.DefaultAuditorTTL, pkgconfigmodel.SourceDefault)
	pkgconfig.Set("logs_config.batch_max_content_size", pkgconfigsetup.DefaultBatchMaxContentSize, pkgconfigmodel.SourceDefault)
	pkgconfig.Set("logs_config.batch_max_size", pkgconfigsetup.DefaultBatchMaxSize, pkgconfigmodel.SourceDefault)
	pkgconfig.Set("logs_config.force_use_http", true, pkgconfigmodel.SourceDefault)
	pkgconfig.Set("logs_config.input_chan_size", pkgconfigsetup.DefaultInputChanSize, pkgconfigmodel.SourceDefault)
	pkgconfig.Set("logs_config.max_message_size_bytes", pkgconfigsetup.DefaultMaxMessageSizeBytes, pkgconfigmodel.SourceDefault)
	pkgconfig.Set("logs_config.run_path", "/opt/datadog-agent/run", pkgconfigmodel.SourceDefault)
	pkgconfig.Set("logs_config.sender_backoff_factor", pkgconfigsetup.DefaultLogsSenderBackoffFactor, pkgconfigmodel.SourceDefault)
	pkgconfig.Set("logs_config.sender_backoff_base", pkgconfigsetup.DefaultLogsSenderBackoffBase, pkgconfigmodel.SourceDefault)
	pkgconfig.Set("logs_config.sender_backoff_max", pkgconfigsetup.DefaultLogsSenderBackoffMax, pkgconfigmodel.SourceDefault)
	pkgconfig.Set("logs_config.sender_recovery_interval", pkgconfigsetup.DefaultForwarderRecoveryInterval, pkgconfigmodel.SourceDefault)
	pkgconfig.Set("logs_config.stop_grace_period", 30, pkgconfigmodel.SourceDefault)
	pkgconfig.Set("logs_config.use_v2_api", true, pkgconfigmodel.SourceDefault)
	pkgconfig.SetKnown("logs_config.dev_mode_no_ssl")
	// add logs config pipelines config value, see https://github.com/DataDog/datadog-agent/pull/31190
	logsPipelines := min(4, runtime.GOMAXPROCS(0))
	pkgconfig.Set("logs_config.pipelines", logsPipelines, pkgconfigmodel.SourceDefault)
	setProxyFromEnv(pkgconfig)

	return pkgconfig
}

func setProxyFromEnv(config pkgconfigmodel.Config) {
	proxyConfig := httpproxy.FromEnvironment()
	config.Set("proxy.http", proxyConfig.HTTPProxy, pkgconfigmodel.SourceEnvVar)
	config.Set("proxy.https", proxyConfig.HTTPSProxy, pkgconfigmodel.SourceEnvVar)

	// If this is set to an empty []string, viper will have a type conflict when merging
	// this config during secrets resolution. It unmarshals empty yaml lists to type
	// []interface{}, which will then conflict with type []string and fail to merge.
	var noProxy []any
	for _, v := range strings.Split(proxyConfig.NoProxy, ",") {
		noProxy = append(noProxy, v)
	}
	config.Set("proxy.no_proxy", noProxy, pkgconfigmodel.SourceEnvVar)
}
