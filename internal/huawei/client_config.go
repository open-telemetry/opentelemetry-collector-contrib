// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package huawei // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/huawei"

import (
	"errors"
	"net/url"
	"strconv"

	"github.com/huaweicloud/huaweicloud-sdk-go-v3/core/config"
	"go.opentelemetry.io/collector/config/configopaque"
)

var (
	// Predefined error responses for configuration validation failures
	ErrMissingProjectID = errors.New(`"project_id" is not specified in config`)
	ErrMissingRegionID  = errors.New(`"region_id" is not specified in config`)

	ErrInvalidProxy = errors.New(`"proxy_address" must be specified if "proxy_user" or "proxy_password" is set"`)
)

type HuaweiSessionConfig struct {
	AccessKey configopaque.String `mapstructure:"access_key"`

	SecretKey configopaque.String `mapstructure:"secret_key"`
	// Number of seconds before timing out a request.
	NoVerifySSL bool `mapstructure:"no_verify_ssl"`
	// Upload segments to AWS X-Ray through a proxy.
	ProxyAddress  string `mapstructure:"proxy_address"`
	ProxyUser     string `mapstructure:"proxy_user"`
	ProxyPassword string `mapstructure:"proxy_password"`
}

func CreateHTTPConfig(cfg HuaweiSessionConfig) (*config.HttpConfig, error) {
	if cfg.ProxyAddress == "" {
		return config.DefaultHttpConfig().WithIgnoreSSLVerification(cfg.NoVerifySSL), nil
	}
	proxy, err := configureHTTPProxy(cfg)
	if err != nil {
		return nil, err
	}
	return config.DefaultHttpConfig().WithProxy(proxy), nil
}

func configureHTTPProxy(cfg HuaweiSessionConfig) (*config.Proxy, error) {
	proxyURL, err := url.Parse(cfg.ProxyAddress)
	if err != nil {
		return nil, err
	}

	proxy := config.NewProxy().
		WithSchema(proxyURL.Scheme).
		WithHost(proxyURL.Hostname())
	if len(proxyURL.Port()) > 0 {
		if i, err := strconv.Atoi(proxyURL.Port()); err == nil {
			proxy = proxy.WithPort(i)
		}
	}

	// Configure the username and password if the proxy requires authentication
	if len(cfg.ProxyUser) > 0 {
		proxy = proxy.WithUsername(cfg.ProxyUser).WithPassword(cfg.ProxyPassword)
	}
	return proxy, nil
}
