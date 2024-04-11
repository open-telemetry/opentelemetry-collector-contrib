package datadogexporter

import (
	"strings"

	coreconfig "github.com/DataDog/datadog-agent/comp/core/config"
	"github.com/DataDog/datadog-agent/comp/core/log"
	pkgconfigmodel "github.com/DataDog/datadog-agent/pkg/config/model"
	pkgconfigsetup "github.com/DataDog/datadog-agent/pkg/config/setup"
	"go.opentelemetry.io/collector/component"
)

func newLogComponent(set component.TelemetrySettings) (log.Component, error) {
	zlog := &zaplogger{
		logger: set.Logger,
	}
	return zlog, nil
}

func newConfigComponent(set component.TelemetrySettings, cfg *Config) (coreconfig.Component, error) {
	pkgconfig := pkgconfigmodel.NewConfig("DD", "DD", strings.NewReplacer(".", "_"))
	pkgconfigsetup.InitConfig(pkgconfig)

	// Set the API Key
	pkgconfig.Set("api_key", string(cfg.API.Key), pkgconfigmodel.SourceFile)
	pkgconfig.Set("site", cfg.API.Site, pkgconfigmodel.SourceFile)
	pkgconfig.Set("logs_enabled", true, pkgconfigmodel.SourceFile)
	pkgconfig.Set("log_level", set.Logger.Level().String(), pkgconfigmodel.SourceFile)
	pkgconfig.Set("forwarder_timeout", 10, pkgconfigmodel.SourceDefault)
	pkgconfig.Set("apm_config.enabled", true, pkgconfigmodel.SourceFile)
	pkgconfig.Set("apm_config.apm_non_local_traffic", true, pkgconfigmodel.SourceFile)

	return pkgconfig, nil
}
