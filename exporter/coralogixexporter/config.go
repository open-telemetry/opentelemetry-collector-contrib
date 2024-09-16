// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package coralogixexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/coralogixexporter"

import (
	"fmt"

	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

const (
	cxAppNameAttrName       = "cx.application.name"
	cxSubsystemNameAttrName = "cx.subsystem.name"
)

// Config defines by Coralogix.
type Config struct {
	QueueSettings             exporterhelper.QueueConfig `mapstructure:"sending_queue"`
	configretry.BackOffConfig `mapstructure:"retry_on_failure"`
	TimeoutSettings           exporterhelper.TimeoutConfig `mapstructure:",squash"`

	// Coralogix domain
	Domain string `mapstructure:"domain"`
	// GRPC Settings used with Domain
	DomainSettings configgrpc.ClientConfig `mapstructure:"domain_settings"`

	// Deprecated: [v0.60.0] Coralogix jaeger based trace endpoint
	// will be removed in the next version
	// Please use OTLP endpoint using traces.endpoint
	configgrpc.ClientConfig `mapstructure:",squash"`

	// Coralogix traces ingress endpoint
	Traces configgrpc.ClientConfig `mapstructure:"traces"`

	// The Coralogix metrics ingress endpoint
	Metrics configgrpc.ClientConfig `mapstructure:"metrics"`

	// The Coralogix logs ingress endpoint
	Logs configgrpc.ClientConfig `mapstructure:"logs"`

	// Your Coralogix private key (sensitive) for authentication
	PrivateKey configopaque.String `mapstructure:"private_key"`

	// Ordered list of Resource attributes that are used for Coralogix
	// AppName and SubSystem values. The first non-empty Resource attribute is used.
	// Example: AppNameAttributes: ["k8s.namespace.name", "service.namespace"]
	// Example: SubSystemAttributes: ["k8s.deployment.name", "k8s.daemonset.name", "service.name"]
	AppNameAttributes   []string `mapstructure:"application_name_attributes"`
	SubSystemAttributes []string `mapstructure:"subsystem_name_attributes"`
	// Default Coralogix application and subsystem name values.
	AppName   string `mapstructure:"application_name"`
	SubSystem string `mapstructure:"subsystem_name"`
}

func isEmpty(endpoint string) bool {
	if endpoint == "" || endpoint == "https://" || endpoint == "http://" {
		return true
	}
	return false
}
func (c *Config) Validate() error {
	// validate that at least one endpoint is set up correctly
	if isEmpty(c.Domain) &&
		isEmpty(c.Traces.Endpoint) &&
		isEmpty(c.Metrics.Endpoint) &&
		isEmpty(c.Logs.Endpoint) {
		return fmt.Errorf("`domain` or `traces.endpoint` or `metrics.endpoint` or `logs.endpoint` not specified, please fix the configuration")
	}
	if c.PrivateKey == "" {
		return fmt.Errorf("`private_key` not specified, please fix the configuration")
	}
	if c.AppName == "" {
		return fmt.Errorf("`application_name` not specified, please fix the configuration")
	}

	// check if headers exists
	if len(c.ClientConfig.Headers) == 0 {
		c.ClientConfig.Headers = make(map[string]configopaque.String)
	}
	c.ClientConfig.Headers["ACCESS_TOKEN"] = c.PrivateKey
	c.ClientConfig.Headers["appName"] = configopaque.String(c.AppName)
	return nil
}

func (c *Config) getMetadataFromResource(res pcommon.Resource) (appName, subsystem string) {
	// Example application name attributes: service.namespace, k8s.namespace.name
	for _, appNameAttribute := range c.AppNameAttributes {
		attr, ok := res.Attributes().Get(appNameAttribute)
		if ok && attr.AsString() != "" {
			appName = attr.AsString()
			break
		}
	}

	// Example subsystem name attributes: service.name, k8s.deployment.name, k8s.statefulset.name
	for _, subSystemNameAttribute := range c.SubSystemAttributes {
		attr, ok := res.Attributes().Get(subSystemNameAttribute)
		if ok && attr.AsString() != "" {
			subsystem = attr.AsString()
			break
		}
	}

	if appName == "" {
		appName = c.AppName
	}
	if subsystem == "" {
		subsystem = c.SubSystem
	}

	if appName == "" {
		attr, ok := res.Attributes().Get(cxAppNameAttrName)
		if ok && attr.AsString() != "" {
			appName = attr.AsString()
		}
	}
	if subsystem == "" {
		attr, ok := res.Attributes().Get(cxSubsystemNameAttrName)
		if ok && attr.AsString() != "" {
			subsystem = attr.AsString()
		}
	}
	return appName, subsystem
}

func (c *Config) getDomainGrpcSettings() *configgrpc.ClientConfig {
	settings := c.DomainSettings
	settings.Endpoint = fmt.Sprintf("ingress.%s:443", c.Domain)
	return &settings
}
